package lotto

import (
	"os"
	"path/filepath"
	"testing"

	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecutePrediction_ParseSuccess(t *testing.T) {
	// 1. 임시 로그 파일 생성
	resultData := testutil.LoadTestDataAsString(t, "prediction_result.log")
	tmpDir := t.TempDir()
	logFilePath := filepath.Join(tmpDir, "prediction_result.log")
	err := os.WriteFile(logFilePath, []byte(resultData), 0644)
	assert.NoError(t, err)

	// 2. Mock 설정
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	mockProcess.On("Wait").Return(nil)
	outputStr := "로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:" + logFilePath + ")"
	mockProcess.On("Output").Return(outputStr)

	// StartCommand signature check (mock.Anything for Context)
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  tmpDir,
		executor: mockExecutor,
	}

	msg, _, err := taskInstance.executePrediction()

	assert.NoError(t, err)
	assert.Contains(t, msg, "당첨 확률이 높은 당첨번호 목록")
	assert.Contains(t, msg, "• 당첨번호5")

	mockExecutor.AssertExpectations(t)
	mockProcess.AssertExpectations(t)
}

func TestExecutePrediction_StartCommandError(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(nil, assert.AnError)

	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  "/tmp",
		executor: mockExecutor,
	}

	msg, _, err := taskInstance.executePrediction()

	assert.Error(t, err)
	assert.Empty(t, msg)
}

func TestExecutePrediction_WaitError(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	mockProcess.On("Wait").Return(assert.AnError)
	mockProcess.On("Stderr").Return("Java Exception Occurred...") // Stderr mock

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  "/tmp",
		executor: mockExecutor,
	}

	_, _, err := taskInstance.executePrediction()
	assert.Error(t, err)
}

func TestExecutePrediction_InvalidOutput(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	mockProcess.On("Wait").Return(nil)
	mockProcess.On("Output").Return("Invalid output format")

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  "/tmp",
		executor: mockExecutor,
	}

	_, _, err := taskInstance.executePrediction()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "정상적으로 완료되었는지 확인할 수 없습니다")
}
