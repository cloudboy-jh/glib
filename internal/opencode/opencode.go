package opencode

import (
	"io"
	"os"
	"os/exec"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
)

type State struct {
	Cmd     *exec.Cmd
	PTYFile *os.File
	Buffer  []byte
	Running bool
}

func Start() (State, error) {
	cmd := exec.Command("opencode")
	f, err := pty.Start(cmd)
	if err != nil {
		return State{}, err
	}
	return State{Cmd: cmd, PTYFile: f, Running: true}, nil
}

func (s *State) Stop() {
	if s.PTYFile != nil {
		_ = s.PTYFile.Close()
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		_ = s.Cmd.Process.Kill()
	}
	*s = State{}
}

func (s *State) ReadChunk() ([]byte, error) {
	if !s.Running || s.PTYFile == nil {
		return nil, io.EOF
	}
	buf := make([]byte, 4096)
	n, err := s.PTYFile.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func (s *State) ForwardInput(k tea.KeyMsg) {
	if !s.Running || s.PTYFile == nil {
		return
	}
	data := keyToBytes(k)
	if len(data) == 0 {
		return
	}
	_, _ = s.PTYFile.Write(data)
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
