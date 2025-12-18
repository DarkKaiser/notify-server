package lotto

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
)

var (
	// MaxOutputBufferSize 외부 명령 실행 시 Stdout/Stderr 캡처를 위한 버퍼의 최대 크기입니다 (기본 10MB).
	//
	// 이 제한은 하위 프로세스가 의도치 않게(또는 악의적으로) 과도한 출력을 생성하여
	// 서버의 메모리를 고갈시키는 상황(OOM, DoS)을 방지하기 위한 안전 장치입니다.
	// 제한을 초과하는 출력은 limitWriter에 의해 자동으로 잘림(Truncated) 처리됩니다.
	MaxOutputBufferSize = 10 * 1024 * 1024
)

// commandProcess 실행 중인 프로세스를 추상화하는 인터페이스
type commandProcess interface {
	Wait() error
	Kill() error
	Stdout() string
	Stderr() string
}

// defaultCommandProcess exec.Cmd를 래핑한 기본 프로세스 구현
type defaultCommandProcess struct {
	cmd          *exec.Cmd
	stdoutBuffer *bytes.Buffer
	stderrBuffer *bytes.Buffer
}

func (p *defaultCommandProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *defaultCommandProcess) Kill() error {
	return p.cmd.Process.Signal(os.Kill)
}

func (p *defaultCommandProcess) Stdout() string {
	return p.stdoutBuffer.String()
}

func (p *defaultCommandProcess) Stderr() string {
	return p.stderrBuffer.String()
}

// commandExecutor 외부 명령 실행을 추상화하는 인터페이스
type commandExecutor interface {
	StartCommand(ctx context.Context, name string, args ...string) (commandProcess, error)
}

// defaultCommandExecutor 기본 명령 실행기 (os/exec 사용)
type defaultCommandExecutor struct {
	limit int // 출력 제한 크기 (0이면 기본값 사용)
	env   []string
}

func (e *defaultCommandExecutor) StartCommand(ctx context.Context, name string, args ...string) (commandProcess, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// 출력이 너무 많아 메모리를 고갈시키는 것을 방지하기 위해 제한된 Writer를 사용합니다.
	limit := e.limit
	if limit <= 0 {
		limit = MaxOutputBufferSize
	}

	var stdoutBuffer bytes.Buffer
	var stderrBuffer bytes.Buffer

	cmd.Stdout = &limitWriter{w: &stdoutBuffer, limit: limit}
	cmd.Stderr = &limitWriter{w: &stderrBuffer, limit: limit}

	// 설정된 환경 변수가 있다면 명령어 실행 시 적용합니다.
	if len(e.env) > 0 {
		cmd.Env = e.env
	}

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &defaultCommandProcess{
		cmd:          cmd,
		stdoutBuffer: &stdoutBuffer,
		stderrBuffer: &stderrBuffer,
	}, nil
}

type limitWriter struct {
	w       io.Writer
	limit   int
	written int
}

func (l *limitWriter) Write(p []byte) (n int, err error) {
	if l.written >= l.limit {
		return len(p), nil // 제한 초과 시 조용히 버림
	}

	remaining := l.limit - l.written

	toWrite := p
	if len(p) > remaining {
		toWrite = p[:remaining]
	}

	n, err = l.w.Write(toWrite)
	l.written += n

	if len(p) > remaining {
		if err == nil {
			// 제한에 도달했으므로 잘림 표시를 추가합니다. (에러 무시)
			_, _ = l.w.Write([]byte("\n... (truncated)"))
			// 이후 쓰기를 막기 위해 written을 limit 이상으로 설정
			l.written = l.limit
		}
		// 실제로는 일부만 썼지만, 호출자에게는 다 썼다고 거짓말하여 에러 방지
		return len(p), nil
	}
	return n, err
}
