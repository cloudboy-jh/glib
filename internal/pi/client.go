package pi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
)

type PiProcess struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *bytes.Buffer

	mu      sync.Mutex
	running bool
	closed  bool
	events  chan tea.Msg
	done    chan struct{}
}

func Start(repoPath string) (*PiProcess, error) {
	cmd := exec.Command("pi", "--mode", "rpc", "--cwd", repoPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &PiProcess{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		running: true,
		events:  make(chan tea.Msg, 128),
		done:    make(chan struct{}),
	}
	go p.readStdoutLoop()
	go p.waitLoop()
	return p, nil
}

func (p *PiProcess) Send(cmd any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running || p.stdin == nil {
		return fmt.Errorf("pi process not running")
	}
	b, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	if _, err := p.stdin.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}

func (p *PiProcess) ReadLoop() tea.Cmd {
	return func() tea.Msg {
		if p == nil {
			return PiExitMsg{Err: io.EOF}
		}
		msg, ok := <-p.events
		if !ok {
			return PiExitMsg{Err: io.EOF}
		}
		return msg
	}
}

func (p *PiProcess) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	stdin := p.stdin
	cmd := p.cmd
	p.stdin = nil
	p.mu.Unlock()

	if stdin != nil {
		_ = stdin.Close()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	<-p.done
}

func (p *PiProcess) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

func (p *PiProcess) readStdoutLoop() {
	buf := make([]byte, 0, 4096)
	for {
		line, err := readJSONLRecord(p.stdout, &buf)
		if err != nil {
			if err != io.EOF {
				p.emit(PiExitMsg{Err: err})
			}
			return
		}
		payload, err := decodeJSONL(line)
		if err != nil {
			p.emit(PiEventMsg{Raw: line, Type: "raw", Payload: map[string]any{"line": string(line)}})
			continue
		}
		t, _ := payload["type"].(string)
		if t == "response" {
			p.emit(ResponseFromPayload(line, payload))
			continue
		}
		p.emit(PiEventMsg{Raw: line, Type: t, Payload: payload})
	}
}

func (p *PiProcess) waitLoop() {
	defer close(p.done)
	err := p.cmd.Wait()
	p.mu.Lock()
	wasRunning := p.running
	p.running = false
	p.mu.Unlock()

	if !wasRunning {
		p.closeEvents()
		return
	}
	if err != nil {
		if p.stderr != nil {
			if msg := strings.TrimSpace(p.stderr.String()); msg != "" {
				p.emit(PiExitMsg{Err: fmt.Errorf("%s", msg)})
				p.closeEvents()
				return
			}
		}
		p.emit(PiExitMsg{Err: err})
	} else {
		p.emit(PiExitMsg{Err: io.EOF})
	}
	p.closeEvents()
}

func (p *PiProcess) emit(msg tea.Msg) {
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()
	if closed {
		return
	}
	select {
	case p.events <- msg:
	default:
		go func() {
			defer func() { _ = recover() }()
			p.events <- msg
		}()
	}
}

func (p *PiProcess) closeEvents() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()
	close(p.events)
}
