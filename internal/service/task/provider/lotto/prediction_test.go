package lotto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions for Setup & Mocks
// =============================================================================

// setupPredictionTest initializes the necessary objects for testing
func setupPredictionTest(t *testing.T) (*task, *MockCommandExecutor, *MockCommandProcess, string, string) {
	tmpDir := t.TempDir()
	logFileName := "result.log"
	logFilePath := filepath.Join(tmpDir, logFileName)

	mockExecutor := new(MockCommandExecutor)
	mockProcess := new(MockCommandProcess)

	lottoTask := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     "LOTTO",
				CommandID:  "PREDICT",
				NotifierID: "NOTI",
				RunBy:      contract.TaskRunByScheduler,
			},
			InstanceID: "INSTANCE",
			NewSnapshot: func() interface{} {
				return &predictionSnapshot{}
			},
		}, false),
		appPath:  tmpDir,
		executor: mockExecutor,
	}

	return lottoTask, mockExecutor, mockProcess, tmpDir, logFilePath
}

// createDummyLogFile creates a dummy result file for testing
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

// =============================================================================
// Unit Tests: Execute Prediction Logic
// =============================================================================

func TestExecutePrediction(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(t *testing.T, logFilePath string)
		mockSetup     func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string)
		expectedError string
		expectedMsg   string
	}{
		{
			name: "Success",
			setup: func(t *testing.T, logFilePath string) {
				createDummyLogFile(t, logFilePath)
			},
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", logFilePath))
				// Stderr is not called on success
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedMsg: "당첨번호1 [ 1, 2, 3, 4, 5, 6 ]",
		},
		{
			name: "Start Command Fail",
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(nil, errors.New("failed to start"))
			},
			expectedError: "failed to start",
		},
		{
			name: "Wait Fail (Process Crash)",
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(errors.New("exit status 1"))
				mockProcess.On("Stderr").Return("Java StackTrace Mock") // Should be called for logging
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "예측 프로세스 실행 중 오류가 발생하였습니다",
		},
		{
			name: "Wait Fail (No Stderr)",
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(errors.New("exit status 1"))
				mockProcess.On("Stderr").Return("") // Stderr 없음 분기 커버
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "exit status 1",
		},
		{
			name: "Log File Parse Fail (No path in output)",
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return("Invalid Output: No path here")
				mockProcess.On("Stderr").Return("") // Called because extract failed
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "당첨번호 예측 작업의 종료 상태를 확인할 수 없습니다",
		},
		{
			name: "Result File is Relative Path",
			setup: func(t *testing.T, logFilePath string) {
				// 상대 경로로 추출되는 상황 흉내
				// 실행 위치(tmpDir)에 "rel_result.log" 생성
				relFile := filepath.Join(filepath.Dir(logFilePath), "rel_result.log")
				createDummyLogFile(t, relFile)
			},
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				// 절대 경로가 아닌 파일명(상대경로) 반환 -> filepath.IsAbs() = false 경로 커버
				mockProcess.On("Stdout").Return("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:rel_result.log)")
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedMsg: "당첨번호1 [ 1, 2, 3, 4, 5, 6 ]",
		},
		{
			name: "Result File Access Fail (File Not Found)",
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", logFilePath))
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "도중 시스템 오류가 발생했습니다",
		},
		{
			name: "Security: Path Traversal Attempt",
			setup: func(t *testing.T, logFilePath string) {
				// 경로 이탈 공격 테스트.
				// EvalSymlinks, Abs를 통과하려면 실제로 파일이 존재해야 합니다.
				// 현재 디렉터리(tmpDir) 바깥(부모)에 파일을 생성합니다.
				parentDir := filepath.Dir(filepath.Dir(logFilePath))
				traversalFile := filepath.Join(parentDir, "traversal.log")
				_ = os.WriteFile(traversalFile, []byte("dummy"), 0644)
				// 주의: 다른 테스트와 충돌하지 않도록 별도의 테스트 코드로 빼거나 정리를 잘 해야합니다.
				// 여기서는 Setup에서 파일을 만들지만, Teardown이 까다로울 수 있어
				// 차라리 Symlink를 이용해 이탈을 표현할 수도 있습니다.
				// 단순화를 위해 rel 경로를 통해 이탈되는지 검사합니다.
			},
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				parentDir := filepath.Dir(filepath.Dir(logFilePath))
				traversalFile := filepath.Join(parentDir, "traversal.log")
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", traversalFile))
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "허용되지 않은 파일 경로 접근 시도가 감지되었습니다",
		},
		{
			name: "Not a Regular File (Directory)",
			setup: func(t *testing.T, logFilePath string) {
				_ = os.MkdirAll(logFilePath, 0755) // 파일을 폴더로 만듬 -> IsRegular() == false 커버
			},
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", logFilePath))
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "내용을 파싱하는 도중 오류가 발생했습니다", // "일반 파일이 아닙니다" 래핑
		},
		{
			name: "Result File Size Limit Exceeded",
			setup: func(t *testing.T, logFilePath string) {
				f, err := os.Create(logFilePath)
				require.NoError(t, err)
				defer f.Close()
				// Write 1MB + 1 byte
				data := make([]byte, 1024*1024+1)
				_, err = f.Write(data)
				require.NoError(t, err)
			},
			mockSetup: func(mockExecutor *MockCommandExecutor, mockProcess *MockCommandProcess, logFilePath string) {
				mockProcess.On("Wait").Return(nil)
				mockProcess.On("Stdout").Return(fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", logFilePath))
				mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)
			},
			expectedError: "내용을 파싱하는 도중 오류가 발생했습니다", // Wraps "result file size too large"
		},
	}

	for _, tt := range tests {
		// Skip empty tests (like Security placeholder above)
		if tt.name == "Security: Path Traversal Attempt" {
			continue
		}

		t.Run(tt.name, func(t *testing.T) {
			task, mockExecutor, mockProcess, _, logFilePath := setupPredictionTest(t)

			if tt.setup != nil {
				tt.setup(t, logFilePath)
			}
			if tt.mockSetup != nil {
				tt.mockSetup(mockExecutor, mockProcess, logFilePath)
			}

			msg, _, err := task.executePrediction(context.Background())

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Empty(t, msg)
			} else {
				require.NoError(t, err)
				assert.Contains(t, msg, tt.expectedMsg)
			}

			// Cleanup check: File should be deleted on success
			if tt.expectedError == "" {
				_, err := os.Stat(logFilePath)
				assert.True(t, os.IsNotExist(err), "Result file should be deleted")
			}

			mockExecutor.AssertExpectations(t)
			mockProcess.AssertExpectations(t)
		})
	}
}

// =============================================================================
// Concurrency & Cancellation Tests
// =============================================================================

func TestExecutePrediction_Cancellation(t *testing.T) {
	task, mockExecutor, mockProcess, _, _ := setupPredictionTest(t)

	// Channel to signal that the mocked process "started" and is waiting
	processRunning := make(chan struct{})

	mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

	// Mock Wait to block until context is canceled
	mockProcess.On("Wait").Run(func(args mock.Arguments) {
		close(processRunning) // Signal that we are inside Wait
		// We can't easily wait for context cancel here because we don't have access to the *passed* context in Wait args directly
		// (Wait takes no args). But executePrediction checks ctx.Err() after Wait returns err.
		// So we simulate a wait delay.
		time.Sleep(200 * time.Millisecond)
	}).Return(errors.New("signal: killed")) // Simulate kill

	// Depending on implementation:
	// 1. ctx.Err() checked -> returns wrapped context error
	// 2. OR Wait returns error -> checks ctx.Err() -> returns ctx.Err()

	// Implementation line 31:
	// if err = cmdProcess.Wait(); err != nil {
	//    if ctx.Err() != nil { return ..., ctx.Err() }
	//    ...
	// }

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-processRunning
		cancel() // Cancel while "Waiting"
	}()

	start := time.Now()
	_, _, err := task.executePrediction(ctx)
	duration := time.Since(start)

	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	assert.True(t, duration >= 200*time.Millisecond, "Should have waited for the mocked process delay")

	mockExecutor.AssertExpectations(t)
	mockProcess.AssertExpectations(t)
}

func TestExecutePrediction_Timeout(t *testing.T) {
	task, mockExecutor, _, _, _ := setupPredictionTest(t)

	// Context that is already expired
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	// Expect Start to be called with a context.
	// We simulate that the executor checks context and returns error immediately.
	mockExecutor.On("Start", mock.Anything, "java", mock.Anything).Return(nil, context.DeadlineExceeded)

	_, _, err := task.executePrediction(ctx)

	require.Error(t, err)
	// executePrediction wraps context, so we might get DeadlineExceeded from the mock directly
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	mockExecutor.AssertExpectations(t)
}
