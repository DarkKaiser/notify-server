package lotto

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	appconfig "github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockLookPath는 테스트를 위해 execLookPath 함수를 교체하는 Helper입니다.
func mockLookPath(mockFunc func(file string) (string, error)) func() {
	original := execLookPath
	execLookPath = mockFunc
	return func() {
		execLookPath = original
	}
}

func TestNewTask_Comprehensive(t *testing.T) {
	// 정상적인 상황을 위한 기본 설정 Helper
	setupValidEnv := func(t *testing.T) (string, *appconfig.AppConfig) {
		tmpDir := t.TempDir()
		// JAR 파일 생성
		f, err := os.Create(filepath.Join(tmpDir, jarFileName))
		require.NoError(t, err)
		f.Close()

		cfg := &appconfig.AppConfig{
			Tasks: []appconfig.TaskConfig{
				{
					ID:   string(TaskID),
					Data: map[string]interface{}{"app_path": tmpDir},
				},
			},
		}
		return tmpDir, cfg
	}

	tests := []struct {
		name          string
		prepare       func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) // restore func() 반환
		expectedError string
	}{
		{
			name: "Success",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t)
				req := &contract.TaskSubmitRequest{
					TaskID:     TaskID,
					CommandID:  PredictionCommand,
					NotifierID: "telegram",
					RunBy:      contract.TaskRunByUser,
				}
				// Mock LookPath to succeed
				restore := mockLookPath(func(file string) (string, error) {
					return "/usr/bin/java", nil
				})
				return req, cfg, restore
			},
			expectedError: "",
		},
		{
			name: "Registration Check Mismatch (Invalid TaskID)",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				// TaskID가 다르면 tasksvc.ErrTaskNotSupported 반환
				req := &contract.TaskSubmitRequest{TaskID: "INVALTaskID_TASK", CommandID: PredictionCommand}
				return req, &appconfig.AppConfig{}, func() {}
			},
			expectedError: tasksvc.ErrTaskNotSupported.Error(),
		},
		{
			name: "Config Not Found In AppConfig",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}
				// 빈 설정
				return req, &appconfig.AppConfig{Tasks: []appconfig.TaskConfig{}}, func() {}
			},
			expectedError: tasksvc.ErrTaskSettingsNotFound.Error(),
		},
		{
			name: "Empty AppPath",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(TaskID), Data: map[string]interface{}{"app_path": ""}}},
				}
				return req, cfg, func() {}
			},
			expectedError: "'app_path'가 입력되지 않았거나 공백입니다",
		},
		{
			name: "Non-existent AppPath",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(TaskID), Data: map[string]interface{}{"app_path": "/invalid/path"}}},
				}
				return req, cfg, func() {}
			},
			expectedError: "'app_path'로 지정된 경로가 존재하지 않거나 유효하지 않습니다",
		},
		{
			name: "Missing JAR File",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				// 폴더는 있지만 JAR가 없는 경우
				tmpDir := t.TempDir()
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(TaskID), Data: map[string]interface{}{"app_path": tmpDir}}},
				}
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}
				return req, cfg, func() {}
			},
			expectedError: fmt.Sprintf("로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", jarFileName),
		},
		{
			name: "Missing Java Runtime",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t) // 파일 시스템은 정상이지만
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}

				// Mock LookPath to FAIL
				restore := mockLookPath(func(file string) (string, error) {
					return "", exec.ErrNotFound
				})
				return req, cfg, restore
			},
			expectedError: "호스트 시스템에서 Java 런타임(JRE) 환경을 감지할 수 없습니다",
		},
		{
			name: "Invalid Command TaskID",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t)
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: "INVALTaskID_CMD"} // 잘못된 명령어

				restore := mockLookPath(func(file string) (string, error) { return "/bin/java", nil })
				return req, cfg, restore
			},
			expectedError: "지원하지 않는 명령입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, cfg, restore := tt.prepare(t)
			defer restore()

			// newTask 사용 (createTask가 아닌 public API 테스트) -> 이제는 Internal이지만 동일 패키지 테스트
			handler, err := newTask("test-instance", req, cfg)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, handler)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, handler)
				// 핸들러 타입 검증
				lottoTask, ok := handler.(*task)
				assert.True(t, ok)
				assert.Equal(t, TaskID, lottoTask.GetID())
			}
		})
	}
}

func TestTask_Run(t *testing.T) {
	// 실제 run 메서드가 커버되는 통합 테스트 성격의 유닛 테스트
	tmpDir := t.TempDir()

	// 가짜 분석 결과 파일 내용
	fakeAnalysisContent := `
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
	fakeLogFile := filepath.Join(tmpDir, "result_12345.log")

	// Helper to setup fresh environment for each test
	setup := func() (*task, *MockCommandExecutor, *MockCommandProcess, *MockNotificationSender, *MockTaskResultStorage) {
		mockExecutor := new(MockCommandExecutor)
		mockProcess := new(MockCommandProcess)
		mockSender := new(MockNotificationSender)
		mockStorage := new(MockTaskResultStorage)

		task := &task{
			Task:     tasksvc.NewBaseTask(TaskID, PredictionCommand, "test-instance", "telegram", contract.TaskRunByUser),
			appPath:  tmpDir,
			executor: mockExecutor,
		}
		task.SetStorage(mockStorage)
		task.SetExecute(func(_ interface{}, _ bool) (string, interface{}, error) {
			return task.executePrediction()
		})

		// Common Mock Setup
		mockSender.On("SupportsHTML", mock.Anything).Return(true)
		mockStorage.On("Load", mock.Anything, mock.Anything, mock.Anything).Return(nil)
		mockStorage.On("Save", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		return task, mockExecutor, mockProcess, mockSender, mockStorage
	}

	// Test Cases
	t.Run("Success Path", func(t *testing.T) {
		task, mockExecutor, mockProcess, mockSender, _ := setup()

		// MockProcess 설정
		mockProcess.On("Wait").Return(nil)
		stdout := fmt.Sprintf("로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:%s)", fakeLogFile)
		mockProcess.On("Stdout").Return(stdout)
		// Stderr is not called in success path

		mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(mockProcess, nil)

		err := os.WriteFile(fakeLogFile, []byte(fakeAnalysisContent), 0644)
		require.NoError(t, err)

		mockSender.On("Notify", mock.Anything, mock.Anything, mock.MatchedBy(func(msg string) bool {
			return assert.Contains(t, msg, "당첨 확률이 높은 당첨번호 목록")
		})).Return(nil)

		var wg sync.WaitGroup
		doneC := make(chan contract.TaskInstanceID, 1)
		wg.Add(1)

		task.Run(contract.NewTaskContext(), mockSender, &wg, doneC)
		wg.Wait()

		mockProcess.AssertExpectations(t)
		mockExecutor.AssertExpectations(t)
		mockSender.AssertExpectations(t)
	})

	t.Run("Execution Failed (StartCommand Error)", func(t *testing.T) {
		task, mockExecutor, _, mockSender, _ := setup()

		mockExecutor.On("StartCommand", mock.Anything, "java", mock.Anything).Return(nil, fmt.Errorf("fail to start java"))

		mockSender.On("Notify", mock.MatchedBy(func(ctx contract.TaskContext) bool {
			return true
		}), mock.Anything, mock.MatchedBy(func(msg string) bool {
			return assert.Contains(t, msg, "작업 진행중 오류가 발생하여 작업이 실패하였습니다")
		})).Return(nil)

		var wg sync.WaitGroup
		doneC := make(chan contract.TaskInstanceID, 1)
		wg.Add(1)

		task.Run(contract.NewTaskContext(), mockSender, &wg, doneC)
		wg.Wait()

		mockExecutor.AssertExpectations(t)
		mockSender.AssertExpectations(t)
	})
}

// --- Local Mocks for Test ---

type MockNotificationSender struct {
	mock.Mock
}

func (m *MockNotificationSender) NotifyDefault(message string) error {
	args := m.Called(message)
	return args.Error(0)
}

func (m *MockNotificationSender) Notify(taskCtx contract.TaskContext, notifierTaskID types.NotifierID, message string) error {
	args := m.Called(taskCtx, notifierTaskID, message)
	return args.Error(0)
}

func (m *MockNotificationSender) SupportsHTML(notifierTaskID types.NotifierID) bool {
	args := m.Called(notifierTaskID)
	return args.Bool(0)
}

func (m *MockNotificationSender) NotifyWithTitle(notifierTaskID types.NotifierID, title string, message string, errorOccurred bool) error {
	args := m.Called(notifierTaskID, title, message, errorOccurred)
	return args.Error(0)
}

func (m *MockNotificationSender) NotifyDefaultWithError(message string) error {
	args := m.Called(message)
	return args.Error(0)
}

type MockTaskResultStorage struct {
	mock.Mock
}

func (m *MockTaskResultStorage) Get(taskTaskID contract.TaskID, commandTaskID contract.TaskCommandID) (string, error) {
	args := m.Called(taskTaskID, commandTaskID)
	return args.String(0), args.Error(1)
}

func (m *MockTaskResultStorage) Save(taskTaskID contract.TaskID, commandTaskID contract.TaskCommandID, data interface{}) error {
	args := m.Called(taskTaskID, commandTaskID, data)
	return args.Error(0)
}

func (m *MockTaskResultStorage) SetStorage(storage tasksvc.TaskResultStorage) {
	m.Called(storage)
}

func (m *MockTaskResultStorage) Load(taskTaskID contract.TaskID, commandTaskID contract.TaskCommandID, data interface{}) error {
	args := m.Called(taskTaskID, commandTaskID, data)
	return args.Error(0)
}
