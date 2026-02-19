package lotto

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appconfig "github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLookPath는 테스트를 위해 lookPath 함수를 교체하는 Helper입니다.
func mockLookPath(mockFunc func(file string) (string, error)) func() {
	original := lookPath
	lookPath = mockFunc
	return func() {
		lookPath = original
	}
}

func TestNewTask_Comprehensive(t *testing.T) {
	// lotto.init()이 이미 실행되었겠지만, 테스트를 위해 명시적으로 확인하거나
	// provider.Register가 중복 호출되면 패닉이 발생하므로 주의해야 합니다.
	// 기본적으로 lotto.init()에서 등록되므로 별도 등록은 필요 없으나,
	// req.TaskID와 req.CommandID가 올바른지 확인해야 합니다.

	// 정상적인 상황을 위한 기본 설정 Helper
	setupValidEnv := func(t *testing.T) (string, *appconfig.AppConfig) {
		tmpDir := t.TempDir()
		// JAR 파일 생성
		f, err := os.Create(filepath.Join(tmpDir, predictionJarName))
		require.NoError(t, err)
		f.Close()

		cfg := &appconfig.AppConfig{
			Tasks: []appconfig.TaskConfig{
				{
					ID:   string(TaskID),
					Data: map[string]interface{}{"app_path": tmpDir},
					Commands: []appconfig.CommandConfig{
						{
							ID: string(PredictionCommand),
						},
					},
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
			expectedError: provider.ErrTaskNotSupported.Error(),
		},
		{
			name: "Config Not Found In AppConfig",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}
				// 빈 설정
				return req, &appconfig.AppConfig{Tasks: []appconfig.TaskConfig{}}, func() {}
			},
			expectedError: provider.ErrTaskNotFound.Error(),
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
			expectedError: fmt.Sprintf("로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", predictionJarName),
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
			expectedError: "Java 런타임(JRE) 환경을 찾을 수 없습니다",
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
		{
			name: "Relative AppPath",
			prepare: func(t *testing.T) (*contract.TaskSubmitRequest, *appconfig.AppConfig, func()) {
				// Windows에서 다른 드라이브 간 상대 경로 생성이 불가능하므로,
				// 현재 디렉토리 하위에 임시 디렉토리를 생성하여 테스트합니다.
				wd, err := os.Getwd()
				require.NoError(t, err)

				localTmpDir := filepath.Join(wd, "temp_rel_test")
				err = os.MkdirAll(localTmpDir, 0755)
				require.NoError(t, err)

				// JAR 파일 생성 (검증 통과를 위해)
				f, err := os.Create(filepath.Join(localTmpDir, predictionJarName))
				require.NoError(t, err)
				f.Close()

				relPath := "temp_rel_test"

				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{
						{
							ID:   string(TaskID),
							Data: map[string]interface{}{"app_path": relPath},
							Commands: []appconfig.CommandConfig{
								{
									ID: string(PredictionCommand),
								},
							},
						},
					},
				}
				req := &contract.TaskSubmitRequest{TaskID: TaskID, CommandID: PredictionCommand}

				restore := mockLookPath(func(file string) (string, error) { return "/bin/java", nil })

				// Cleanup function
				cleanup := func() {
					restore()
					os.RemoveAll(localTmpDir)
				}

				return req, cfg, cleanup
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, cfg, restore := tt.prepare(t)
			if restore != nil {
				defer restore()
			}

			// newTask 사용 (createTask가 아닌 public API 테스트) -> 이제는 Internal이지만 동일 패키지 테스트
			handler, err := newTask(provider.NewTaskParams{
				InstanceID:  "test-instance",
				Request:     req,
				AppConfig:   cfg,
				Storage:     &contractmocks.MockTaskResultStore{},
				Fetcher:     nil,
				NewSnapshot: func() any { return &predictionSnapshot{} },
			})

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, handler)
			} else {
				require.NoError(t, err)
				require.NotNil(t, handler)
				// 핸들러 타입 검증
				lottoTask, ok := handler.(*task)
				require.True(t, ok)
				assert.Equal(t, TaskID, lottoTask.ID())
			}
		})
	}
}

// --- Local Mocks for Test ---

// MockTaskResultStore removed in favor of contractmocks.MockTaskResultStore
