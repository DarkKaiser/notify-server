package lotto

import (
	"os"
	"path/filepath"
	"regexp"
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

	// 2. Mock 설정 (Shared Mocks from mock_test.go)
	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	// Wait 성공
	mockProcess.On("Wait").Return(nil)

	// Output: 로그 파일 경로를 포함한 완료 메시지 반환
	// Windows 경로 문제 회피를 위해 filepath.Join 결과 사용
	outputStr := "로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:" + logFilePath + ")"
	mockProcess.On("Output").Return(outputStr)

	// StartCommand 호출 기대
	mockExecutor.On("StartCommand", "java", mock.Anything).Return(mockProcess, nil)

	// 3. Task 인스턴스 생성
	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  tmpDir,
		executor: mockExecutor,
	}

	// 4. 실행
	msg, _, err := taskInstance.executePrediction()

	// 5. 검증
	assert.NoError(t, err)
	assert.Contains(t, msg, "당첨 확률이 높은 당첨번호 목록")
	assert.Contains(t, msg, "• 당첨번호1")
	assert.Contains(t, msg, "• 당첨번호5")

	mockExecutor.AssertExpectations(t)
	mockProcess.AssertExpectations(t)
}

func TestExecutePrediction_StartCommandError(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)
	mockExecutor.On("StartCommand", "java", mock.Anything).Return(nil, assert.AnError)

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
	mockExecutor.On("StartCommand", "java", mock.Anything).Return(mockProcess, nil)

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
	mockProcess.On("Output").Return("Invalid output format") // 잘못된 출력

	mockExecutor.On("StartCommand", "java", mock.Anything).Return(mockProcess, nil)

	taskInstance := &task{
		Task:     tasksvc.NewBaseTask("LOTTO", "Prediction", "test-instance", "test-notifier", tasksvc.RunByUser),
		appPath:  "/tmp",
		executor: mockExecutor,
	}

	_, _, err := taskInstance.executePrediction()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "정상적으로 완료되었는지 확인할 수 없습니다")
}

func TestExecutePrediction_RegexLogic(t *testing.T) {
	// 별도의 통합 테스트없이 정규식 로직만 빠르게 검증
	t.Run("FilePathExtraction", func(t *testing.T) {
		output := "로또 당첨번호 예측작업이 종료되었습니다. 100개의 대상 당첨번호가 추출되었습니다.(경로:C:\\test\\result.log)"
		re := regexp.MustCompile(`경로:(.*?)\.log`) // prediction.go의 로직 복제 테스트
		match := re.FindStringSubmatch(output)

		// 주의: 현재 prediction.go는 FindString + Slicing 방식을 사용중임 (User가 개선 안함 선택)
		// 따라서 실제 로직 테스트는 executePrediction 통합 테스트가 더 정확함.
		// 여기서는 정규식 유효성만 체크
		assert.NotNil(t, match)
	})
}
