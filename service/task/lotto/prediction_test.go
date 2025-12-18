package lotto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecutePrediction_Success(t *testing.T) {
	// 1. Setup - Temp Dir & dummy log file
	tmpDir := t.TempDir()
	logFileName := "result.log"
	logFilePath := filepath.Join(tmpDir, logFileName)

	// Create dummy result log file
	dummyLogContent := `
======================
- 분석결과
======================
당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.

당첨번호1 [ 1, 2, 3, 4, 5, 6 ]
당첨번호2 [ 7, 8, 9, 10, 11, 12 ]
당첨번호3 [ 13, 14, 15, 16, 17, 18 ]
당첨번호4 [ 19, 20, 21, 22, 23, 24 ]
당첨번호5 [ 25, 26, 27, 28, 29, 30 ]
`
	err := os.WriteFile(logFilePath, []byte(dummyLogContent), 0644)
	assert.NoError(t, err)

	// 2. Setup - Mocks
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	// Mock behavior: Wait succeeds
	mockProcess.On("Wait").Return(nil)

	// Mock behavior: Output returns string containing path to log file
	// The parser looks for "로또 당첨번호 예측작업이 종료되었습니다... (경로:...)"
	mockOutput := fmt.Sprintf(`
[INFO] Start
...
로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)
[INFO] End
`, logFilePath)
	mockProcess.On("Stdout").Return(mockOutput)
	// Stderr is only called on error, so we don't expect it in success case

	// Executor should start "java"
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	// 3. Setup - Task
	lottoTask := &task{
		Task:     tasksvc.NewBaseTask(ID, PredictionCommand, "instance-1", "notifier-1", tasksvc.RunByUser),
		appPath:  tmpDir,
		executor: mockExecutor,
	}

	// 4. Execution
	msg, _, err := lottoTask.executePrediction()

	// 5. Verification
	assert.NoError(t, err)
	assert.Contains(t, msg, "당첨번호1 [ 1, 2, 3, 4, 5, 6 ]")

	// Verify Cleanup: The log file should be removed
	_, err = os.Stat(logFilePath)
	assert.True(t, os.IsNotExist(err), "Log file should be deleted after processing")

	mockExecutor.AssertExpectations(t)
	mockProcess.AssertExpectations(t)
}

func TestExecutePrediction_StartCommandFail(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)

	// Mock behavior: StartCommand fails immediately
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(nil, fmt.Errorf("failed to start"))

	lottoTask := &task{
		Task:     tasksvc.NewBaseTask(ID, PredictionCommand, "instance-1", "notifier-1", tasksvc.RunByUser),
		appPath:  "dummy",
		executor: mockExecutor,
	}

	msg, _, err := lottoTask.executePrediction()

	assert.Error(t, err)
	assert.Equal(t, "failed to start", err.Error())
	assert.Empty(t, msg)
}

func TestExecutePrediction_WaitFail(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	mockProcess.On("Wait").Return(fmt.Errorf("process crashed"))
	mockProcess.On("Stderr").Return("Java StackTrace...")

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	lottoTask := &task{
		Task:     tasksvc.NewBaseTask(ID, PredictionCommand, "instance-1", "notifier-1", tasksvc.RunByUser),
		appPath:  "dummy",
		executor: mockExecutor,
	}

	msg, _, err := lottoTask.executePrediction()

	assert.Error(t, err)
	assert.Equal(t, "process crashed", err.Error())
	assert.Empty(t, msg)
}

func TestExecutePrediction_LogFileParseFail(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	mockProcess.On("Wait").Return(nil)
	// Output does not contain the expected path pattern
	mockProcess.On("Stdout").Return("Invalid Output")
	mockProcess.On("Stderr").Return("")

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	lottoTask := &task{
		Task:     tasksvc.NewBaseTask(ID, PredictionCommand, "instance-1", "notifier-1", tasksvc.RunByUser),
		appPath:  "dummy",
		executor: mockExecutor,
	}

	msg, _, err := lottoTask.executePrediction()

	assert.Error(t, err)
	// Should be an ExecutionFailed error from extractLogFilePath
	assert.IsType(t, &apperrors.AppError{}, err)
	assert.Empty(t, msg)
}

func TestExecutePrediction_ContextTimeout_Propagation(t *testing.T) {
	// Verify that the context passed to StartCommand actually has a deadline/timeout
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	// We capture the context passed to StartCommand to verify it
	mockExecutor.On("StartCommand", mock.MatchedBy(func(ctx context.Context) bool {
		deadline, ok := ctx.Deadline()
		if !ok {
			return false
		}
		// The timeout is set to 10 minutes in prediction.go
		// We can't check exact time, but we can check if it's roughly in the future
		return time.Until(deadline) > 0 && time.Until(deadline) <= 11*time.Minute
	}), "java", mock.Anything).Return(mockProcess, nil)

	mockProcess.On("Wait").Return(nil)
	// Minimal valid output to pass the rest of the flow (or fail later, doesn't matter for this test)
	// Let's Just fail at extraction to keep it simple
	mockProcess.On("Stdout").Return("Invalid")
	mockProcess.On("Stderr").Return("")

	lottoTask := &task{
		Task:     tasksvc.NewBaseTask(ID, PredictionCommand, "instance-1", "notifier-1", tasksvc.RunByUser),
		appPath:  "dummy",
		executor: mockExecutor,
	}

	_, _, _ = lottoTask.executePrediction()

	mockExecutor.AssertExpectations(t)
}
