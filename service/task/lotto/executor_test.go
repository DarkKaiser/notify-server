package lotto

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultCommandExecutor_StartCommand_Success(t *testing.T) {
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

	// Context 추가
	process, err := executor.StartCommand(context.Background(), cmdName, cmdArgs...)
	assert.NoError(t, err)
	assert.NotNil(t, process)

	err = process.Wait()
	assert.NoError(t, err)

	output := process.Output()
	assert.Contains(t, output, "hello")
}

func TestDefaultCommandExecutor_StartCommand_Fail(t *testing.T) {
	executor := &defaultCommandExecutor{}

	process, err := executor.StartCommand(context.Background(), "nonexistent_command_12345")
	assert.Error(t, err)
	assert.Nil(t, process)
}

func TestDefaultCommandProcess_Kill(t *testing.T) {
	executor := &defaultCommandExecutor{}

	var cmdName string
	var cmdArgs []string

	if runtime.GOOS == "windows" {
		cmdName = "ping"
		cmdArgs = []string{"127.0.0.1", "-n", "10"}
	} else {
		cmdName = "sleep"
		cmdArgs = []string{"10"}
	}

	process, err := executor.StartCommand(context.Background(), cmdName, cmdArgs...)
	if err != nil {
		t.Skip("ping/sleep 명령을 실행할 수 없어 테스트를 건너뜁니다.")
	}
	assert.NotNil(t, process)

	err = process.Kill()
	assert.NoError(t, err)
}
