package lotto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// setupPredictionTest는 테스트에 필요한 객체들을 초기화하는 Helper 함수입니다.
func setupPredictionTest(t *testing.T) (*task, *MockCommandExecutor, *MockCommandProcess, string, string) {
	tmpDir := t.TempDir()
	logFileName := "result.log"
	logFilePath := filepath.Join(tmpDir, logFileName)

	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	lottoTask := &task{
		Base:     tasksvc.NewBaseTask(TaskID, PredictionCommand, "instance-1", "notifier-1", contract.TaskRunByUser),
		appPath:  tmpDir,
		executor: mockExecutor,
	}

	return lottoTask, mockExecutor, mockProcess, tmpDir, logFilePath
}

// createDummyLogFile는 테스트용 더미 결과 파일을 생성합니다.
func createDummyLogFile(t *testing.T, path string) {
	content := `
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
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)
}

func TestExecutePrediction_Success(t *testing.T) {
	task, mockExecutor, mockProcess, _, logFilePath := setupPredictionTest(t)
	createDummyLogFile(t, logFilePath)

	// Mock Process
	mockProcess.On("Wait").Return(nil)
	mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", logFilePath))

	// Mock Executor
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	// Execute
	msg, _, err := task.executePrediction()

	// Verify
	assert.NoError(t, err)
	assert.Contains(t, msg, "당첨번호1 [ 1, 2, 3, 4, 5, 6 ]")

	// File Cleanup Check
	_, err = os.Stat(logFilePath)
	assert.True(t, os.IsNotExist(err), "Log file should be deleted")

	mockExecutor.AssertExpectations(t)
	mockProcess.AssertExpectations(t)
}

func TestExecutePrediction_StartCommandFail(t *testing.T) {
	task, mockExecutor, _, _, _ := setupPredictionTest(t)

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(nil, errors.New("failed to start"))

	msg, _, err := task.executePrediction()

	assert.Error(t, err)
	assert.Equal(t, "failed to start", err.Error())
	assert.Empty(t, msg)
}

func TestExecutePrediction_WaitFail(t *testing.T) {
	task, mockExecutor, mockProcess, _, _ := setupPredictionTest(t)

	mockProcess.On("Wait").Return(errors.New("process crashed"))
	mockProcess.On("Stderr").Return("Java StackTrace...")
	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	msg, _, err := task.executePrediction()

	assert.Error(t, err)
	assert.Equal(t, "process crashed", err.Error())
	assert.Empty(t, msg)
}

func TestExecutePrediction_LogFileParseFail(t *testing.T) {
	task, mockExecutor, mockProcess, _, _ := setupPredictionTest(t)

	mockProcess.On("Wait").Return(nil)
	mockProcess.On("Stdout").Return("Invalid Output")
	// Stderr가 호출되는지 여부는 구현에 따라 다르지만, 에러 발생 시 Stderr 로깅 로직이 있다면 호출될 수 있음.
	// 현재 로직상 extractLogFilePath 실패 시 Stderr를 확인하므로 설정 필요.
	mockProcess.On("Stderr").Return("")

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	msg, _, err := task.executePrediction()

	assert.Error(t, err)
	assert.IsType(t, &apperrors.AppError{}, err) // apperrors 타입인지 확인
	assert.Empty(t, msg)
}

func TestExecutePrediction_PathTraversal(t *testing.T) {
	task, mockExecutor, mockProcess, tmpDir, _ := setupPredictionTest(t)

	// 악의적인 경로 반환 (상위 디렉토리 접근 시도)
	// TempDir의 상위 디렉토리의 파일로 위장
	maliciousPath := filepath.Join(tmpDir, "..", "boot.log")

	mockProcess.On("Wait").Return(nil)
	mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", maliciousPath))
	mockProcess.On("Stderr").Return("") // 에러 처리 흐름에서 호출됨

	mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	msg, _, err := task.executePrediction()

	assert.Error(t, err)
	// Path Traversal 방지 로직에 걸려야 함
	// "허용된 경로 범위를 벗어난 파일 접근" 메시지 확인
	assert.Contains(t, err.Error(), "허용된 경로 범위를 벗어난 파일 접근")
	assert.Empty(t, msg)
}

func TestExecuteCommandIDCheckPredictioncellation(t *testing.T) {
	task, mockExecutor, mockProcess, _, _ := setupPredictionTest(t)

	// Context 취소를 감지하기 위한 채널
	ctxCancelled := make(chan struct{})
	var once sync.Once

	// StartCommand에서 Context를 캡처
	mockExecutor.On("StartCommand", mock.MatchedBy(func(ctx context.Context) bool {
		return true // 단순 매칭
	}), "java", mock.Anything).Run(func(args mock.Arguments) {
		ctx := args.Get(0).(context.Context)
		go func() {
			<-ctx.Done()
			once.Do(func() {
				close(ctxCancelled)
			})
		}()
	}).Return(mockProcess, nil)

	// Wait는 blocking 되어야 함 (취소될 때까지)
	mockProcess.On("Wait").Run(func(args mock.Arguments) {
		<-ctxCancelled
	}).Return(context.Canceled) // 취소되면 context.Canceled 에러를 리턴한다고 가정 (또는 Wait이 에러 뱉음)

	// 별도 고루틴에서 실행
	errCh := make(chan error, 1)
	go func() {
		_, _, err := task.executePrediction()
		errCh <- err
	}()

	// 100ms 후 취소 요청
	time.Sleep(100 * time.Millisecond)
	task.Cancel() // 인스턴스 TaskID가 필요하지만 여기선 내부 플래그만 세팅하면 됨 (mocking level)
	// 하지만 tasksvc.NewBaseTask로 만든 task는 CancelTask 메서드를 통해 canceled 플래그를 세팅함.
	// task.CancelTask implementation logic: sets canceled=true.
	// prediction.go has a polling loop checks t.IsCanceled().

	select {
	case err := <-errCh:
		// 취소된 경우 에러는 nil이어야 함 (prediction.go의 로직: if ctx.Err() == context.Canceled { return nil })
		// 단, Wait이 context.Canceled가 아닌 다른 에러를 뱉으면 에러처리됨.
		// prediction.go:
		// if err != nil { if ctx.Err() == context.Canceled { return nil } ... }
		// 여기서 ctx.Err()는 타임아웃 컨텍스트가 캔슬되었으므로 Canceled 상태일 것임.
		assert.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Prediction task did not cancel in time")
	}

	mockExecutor.AssertExpectations(t)
}
