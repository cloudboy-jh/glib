package opencode

import (
	"io"
	"os/exec"
	"runtime"
	"sync"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
)

type State struct {
	Cmd     *exec.Cmd
	Reader  io.Reader
	Writer  io.Writer
	closers []io.Closer
	Buffer  []byte
	Running bool
}

func Start() (State, error) {
	cmd := exec.Command("opencode")
	if runtime.GOOS == "windows" {
		return startWithPipes(cmd)
	}
	f, err := pty.Start(cmd)
	if err != nil {
		return startWithPipes(cmd)
	}
	return State{Cmd: cmd, Reader: f, Writer: f, closers: []io.Closer{f}, Running: true}, nil
}

func startWithPipes(cmd *exec.Cmd) (State, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return State{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return State{}, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return State{}, err
	}
	if err := cmd.Start(); err != nil {
		return State{}, err
	}

	pr, pw := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(pw, stderr)
	}()
	go func() {
		wg.Wait()
		_ = pw.Close()
		_ = cmd.Wait()
	}()

	return State{
		Cmd:     cmd,
		Reader:  pr,
		Writer:  stdin,
		closers: []io.Closer{stdin, stdout, stderr, pr},
		Running: true,
	}, nil
}

func (s *State) Stop() {
	for _, c := range s.closers {
		_ = c.Close()
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		_ = s.Cmd.Process.Kill()
	}
	*s = State{}
}

func (s *State) ReadChunk() ([]byte, error) {
	if !s.Running || s.Reader == nil {
		return nil, io.EOF
	}
	buf := make([]byte, 4096)
	n, err := s.Reader.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *State) ForwardInput(k tea.KeyMsg) {
	if !s.Running || s.Writer == nil {
		return
	}
	data := keyToBytes(k)
	if len(data) == 0 {
		return
	}
	_, _ = s.Writer.Write(data)
}

func keyToBytes(k tea.KeyMsg) []byte {
	s := k.String()
	switch s {
	case "enter":
		return []byte("\r")
	case "tab":
		return []byte("\t")
	case "backspace":
		return []byte{127}
	case "up":
		return []byte("\x1b[A")
	case "down":
		return []byte("\x1b[B")
	case "right":
		return []byte("\x1b[C")
	case "left":
		return []byte("\x1b[D")
	}
	if utf8.RuneCountInString(s) == 1 {
		return []byte(s)
	}
	return nil
}
