package lotto

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
)

const (
	// defaultMaxOutputSize 외부 명령어 실행 시 캡처할 수 있는 표준 출력 및 표준 에러의 최대 크기입니다.
	// 이 크기를 초과하는 출력은 잘리며, 로그 끝에 "...(생략됨)" 메시지가 추가됩니다.
	defaultMaxOutputSize = 10 * 1024 * 1024
)

// commandProcess 실행 중인 외부 명령(프로세스)을 제어하고, 실행 결과를 조회하는 인터페이스입니다.
type commandProcess interface {
	// Wait 명령어 실행이 완료될 때까지 기다립니다. (Blocking)
	Wait() error

	// Kill 실행 중인 명령어를 강제로 종료합니다.
	Kill() error

	// Stdout 실행 과정에서 발생한 표준 출력(Stdout) 내용을 반환합니다.
	Stdout() string

	// Stderr 실행 과정에서 발생한 표준 에러(Stderr) 내용을 반환합니다.
	Stderr() string
}

// defaultCommandProcess 실행 중인 실제 시스템 명령(프로세스)을 제어하고 결과를 관리하는 구현체입니다.
// `os/exec.Cmd`를 감싸고 있으며, 프로세스 대기(Wait), 강제 종료(Kill) 및 출력 캡처(Stdout/Stderr)를 담당합니다.
type defaultCommandProcess struct {
	cmd *exec.Cmd

	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ commandProcess = (*defaultCommandProcess)(nil)

func (p *defaultCommandProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *defaultCommandProcess) Kill() error {
	if p.cmd.Process == nil {
		return nil
	}

	return p.cmd.Process.Signal(os.Kill)
}

func (p *defaultCommandProcess) Stdout() string {
	return p.stdout.String()
}

func (p *defaultCommandProcess) Stderr() string {
	return p.stderr.String()
}

// commandExecutor 외부 명령 실행을 담당하는 추상화 인터페이스입니다.
// 로직과 시스템 의존성을 분리하여 테스트 시 Stub/Mock 구현체로 쉽게 대체할 수 있습니다.
type commandExecutor interface {
	// Start 주어진 이름과 인자로 외부 명령을 비동기적으로 실행하고, 제어 가능한 핸들러를 반환합니다.
	Start(ctx context.Context, name string, args ...string) (commandProcess, error)
}

// defaultCommandExecutor `os/exec` 패키지를 사용하여 실제 시스템 명령을 실행하는 기본 구현체입니다.
type defaultCommandExecutor struct {
	dir string   // 프로세스 실행 시 사용할 작업 디렉터리 (기본값: 서버의 실행 위치)
	env []string // 프로세스 실행 시 적용할 환경 변수 목록 (예: "KEY=VALUE")

	limit int // 출력 캡처 용량 제한 (0일 경우 기본값 defaultMaxOutputSize 사용)
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ commandExecutor = (*defaultCommandExecutor)(nil)

func (e *defaultCommandExecutor) Start(ctx context.Context, name string, args ...string) (commandProcess, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	// 명령어 실행 시 작업 디렉터리를 설정합니다.
	if e.dir != "" {
		cmd.Dir = e.dir
	}

	// 설정된 환경 변수가 있다면 명령어 실행 시 적용합니다.
	if len(e.env) > 0 {
		cmd.Env = e.env
	}

	// 출력이 너무 많아 메모리를 고갈시키는 것을 방지하기 위해 제한된 Writer를 사용합니다.
	limit := e.limit
	if limit <= 0 {
		limit = defaultMaxOutputSize
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &limitWriter{w: &stdout, limit: limit}
	cmd.Stderr = &limitWriter{w: &stderr, limit: limit}

	// 명령을 실행합니다. 실행이 완료될 때까지 기다리지 않습니다.
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &defaultCommandProcess{
		cmd: cmd,

		stdout: &stdout,
		stderr: &stderr,
	}, nil
}

// limitWriter 지정된 크기까지만 데이터를 기록하고, 나머지는 버리는 Writer 구현체입니다.
// 로그가 너무 많이 쌓여 메모리가 부족해지는 것을 막기 위해 사용됩니다.
// 제한 크기를 넘으면 더 이상 기록하지 않고, 마지막에 "...(생략됨)" 이라고 표시됩니다.
type limitWriter struct {
	w         io.Writer // 실제 데이터를 기록할 대상 Writer (예: bytes.Buffer)
	limit     int       // 기록 가능한 최대 바이트 수
	written   int       // 현재까지 기록된 바이트 수
	truncated bool      // 제한 초과 메시지가 기록되었는지 여부
}

func (lw *limitWriter) Write(p []byte) (n int, err error) {
	if lw.written >= lw.limit {
		// 이미 제한 용량에 도달한 경우, 데이터는 버리되 잘림 표시가 없다면 추가합니다.
		if !lw.truncated {
			lw.truncated = true
			if _, err := lw.w.Write([]byte("\n...(생략됨)")); err != nil {
				return 0, err
			}
		}

		// 데이터를 실제로 기록하지는 않지만, 에러를 반환하면 호출자가 작업을 중단할 수 있으므로 성공한 것처럼 처리합니다.
		return len(p), nil
	}

	// 앞으로 더 기록할 수 있는 남은 용량을 계산합니다.
	remaining := lw.limit - lw.written

	// 입력된 데이터가 남은 용량보다 크면, 기록 가능한 만큼만 잘라서 준비합니다.
	chunk := p
	if len(p) > remaining {
		chunk = p[:remaining]
	}

	// 자른 데이터를 실제로 기록(Write)하고, 사용된 용량을 갱신합니다.
	n, err = lw.w.Write(chunk)
	lw.written += n
	if err != nil {
		return n, err
	}

	// 이번 쓰기로 인해 제한 용량을 초과하게 된 경우
	if len(p) > remaining {
		if !lw.truncated {
			lw.truncated = true
			if _, err := lw.w.Write([]byte("\n...(생략됨)")); err != nil {
				return n, err
			}
		}

		// 일부 데이터만 기록했지만, 호출자에게는 전체 데이터를 모두 처리했다고 보고하여 불필요한 에러 발생을 방지합니다.
		return len(p), nil
	}

	return n, err
}
