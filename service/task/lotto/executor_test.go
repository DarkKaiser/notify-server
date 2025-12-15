package lotto

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCommandExecutor_StartCommand_Success(t *testing.T) {
	// 통합 테스트는 윈도우 환경에서만 동작하거나, OS에 따라 명령어가 달라져야 하므로 주의가 필요합니다.
	// OS에 독립적인 테스트를 위해 간단한 echo 명령을 사용합니다.
	executor := &defaultCommandExecutor{}

	var cmdName string
	var cmdArgs []string

	if runtime.GOOS == "windows" {
		cmdName = "cmd"
		cmdArgs = []string{"/c", "echo", "hello"}
	} else {
		cmdName = "echo"
		cmdArgs = []string{"hello"}
	}

	process, err := executor.StartCommand(cmdName, cmdArgs...)
	assert.NoError(t, err)
	assert.NotNil(t, process)

	// 프로세스 종료 대기
	err = process.Wait()
	assert.NoError(t, err)

	// 출력 확인
	output := process.Output()
	assert.Contains(t, output, "hello")
}

func TestDefaultCommandExecutor_StartCommand_Fail(t *testing.T) {
	executor := &defaultCommandExecutor{}

	// 존재하지 않는 명령어 실행 시 에러 발생 확인
	process, err := executor.StartCommand("nonexistent_command_12345")
	assert.Error(t, err)
	assert.Nil(t, process)
}

func TestDefaultCommandProcess_Kill(t *testing.T) {
	executor := &defaultCommandExecutor{}

	// 오래 실행되는 명령(ping)을 실행하고 강제 종료 테스트
	var cmdName string
	var cmdArgs []string

	if runtime.GOOS == "windows" {
		cmdName = "ping"
		cmdArgs = []string{"127.0.0.1", "-n", "10"}
	} else {
		cmdName = "sleep"
		cmdArgs = []string{"10"}
	}

	process, err := executor.StartCommand(cmdName, cmdArgs...)
	if err != nil {
		t.Skip("ping/sleep 명령을 실행할 수 없어 테스트를 건너뜁니다.")
	}
	assert.NotNil(t, process)

	// 즉시 Kill
	err = process.Kill()
	assert.NoError(t, err)

	// Kill 이후 Wait는 에러를 반환할 수 있음 (OS 및 Signal 처리에 따라 다름)
}
