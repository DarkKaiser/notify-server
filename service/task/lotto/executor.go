package lotto

import (
	"bytes"
	"os"
	"os/exec"
)

// commandProcess 실행 중인 프로세스를 추상화하는 인터페이스
type commandProcess interface {
	Wait() error
	Kill() error
	Output() string
}

// commandExecutor 외부 명령 실행을 추상화하는 인터페이스
type commandExecutor interface {
	StartCommand(name string, args ...string) (commandProcess, error)
}

// defaultCommandProcess exec.Cmd를 래핑한 기본 프로세스 구현
type defaultCommandProcess struct {
	cmd       *exec.Cmd
	outBuffer *bytes.Buffer
}

func (p *defaultCommandProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *defaultCommandProcess) Kill() error {
	return p.cmd.Process.Signal(os.Kill)
}

func (p *defaultCommandProcess) Output() string {
	return p.outBuffer.String()
}

// defaultCommandExecutor 기본 명령 실행기 (os/exec 사용)
type defaultCommandExecutor struct{}

func (e *defaultCommandExecutor) StartCommand(name string, args ...string) (commandProcess, error) {
	cmd := exec.Command(name, args...)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &defaultCommandProcess{
		cmd:       cmd,
		outBuffer: &outBuffer,
	}, nil
}
