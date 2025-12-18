package lotto

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	appconfig "github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
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
		f, err := os.Create(filepath.Join(tmpDir, lottoJarFileName))
		require.NoError(t, err)
		f.Close()

		cfg := &appconfig.AppConfig{
			Tasks: []appconfig.TaskConfig{
				{
					ID:   string(ID),
					Data: map[string]interface{}{"app_path": tmpDir},
				},
			},
		}
		return tmpDir, cfg
	}

	tests := []struct {
		name          string
		prepare       func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) // Teardown 함수 반환
		expectedError string
	}{
		{
			name: "Success",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t)
				req := &tasksvc.SubmitRequest{
					TaskID:     ID,
					CommandID:  PredictionCommand,
					NotifierID: "telegram",
					RunBy:      tasksvc.RunByUser,
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
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				// TaskID가 다르면 tasksvc.ErrTaskNotSupported 반환
				req := &tasksvc.SubmitRequest{TaskID: "INVALID_TASK", CommandID: PredictionCommand}
				return req, &appconfig.AppConfig{}, func() {}
			},
			expectedError: tasksvc.ErrTaskNotSupported.Error(),
		},
		{
			name: "Config Not Found In AppConfig",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}
				// 빈 설정
				return req, &appconfig.AppConfig{Tasks: []appconfig.TaskConfig{}}, func() {}
			},
			expectedError: tasksvc.ErrTaskConfigNotFound.Error(),
		},
		{
			name: "Empty AppPath",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(ID), Data: map[string]interface{}{"app_path": ""}}},
				}
				return req, cfg, func() {}
			},
			expectedError: "필수 구성 항목인 'app_path' 값이 설정되지 않았습니다",
		},
		{
			name: "Non-existent AppPath",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(ID), Data: map[string]interface{}{"app_path": "/invalid/path"}}},
				}
				return req, cfg, func() {}
			},
			expectedError: "'app_path'로 지정된 경로가 존재하지 않거나 유효하지 않습니다",
		},
		{
			name: "Missing JAR File",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				// 폴더는 있지만 JAR가 없는 경우
				tmpDir := t.TempDir()
				cfg := &appconfig.AppConfig{
					Tasks: []appconfig.TaskConfig{{ID: string(ID), Data: map[string]interface{}{"app_path": tmpDir}}},
				}
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}
				return req, cfg, func() {}
			},
			expectedError: fmt.Sprintf("로또 당첨번호 예측 프로그램(%s)을 찾을 수 없습니다", lottoJarFileName),
		},
		{
			name: "Missing Java Runtime",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t) // 파일 시스템은 정상이지만
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}

				// Mock LookPath to FAIL
				restore := mockLookPath(func(file string) (string, error) {
					return "", exec.ErrNotFound
				})
				return req, cfg, restore
			},
			expectedError: "호스트 시스템에서 Java 런타임(JRE) 환경을 감지할 수 없습니다",
		},
		{
			name: "Invalid Command ID",
			prepare: func(t *testing.T) (*tasksvc.SubmitRequest, *appconfig.AppConfig, func()) {
				_, cfg := setupValidEnv(t)
				req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: "INVALID_CMD"} // 잘못된 명령어

				restore := mockLookPath(func(file string) (string, error) { return "/bin/java", nil })
				return req, cfg, restore
			},
			expectedError: "지원하지 않는 명령입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, cfg, teardown := tt.prepare(t)
			defer teardown()

			// newTask 테스트 (createTask가 아닌 public API 테스트)
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
				assert.Equal(t, ID, lottoTask.GetID())
			}
		})
	}
}

func TestInitRegistration(t *testing.T) {
	// init() 함수에 의해 ID가 잘 등록되었는지 확인
	cfgLookup, err := tasksvc.FindConfigForTest(ID, PredictionCommand)
	assert.NoError(t, err)
	assert.NotNil(t, cfgLookup)
	// ConfigLookup.Task (Config) -> NewTask
	assert.NotNil(t, cfgLookup.Task)
	assert.NotNil(t, cfgLookup.Task.NewTask)

	// Snapshot 타입 확인
	snap := cfgLookup.Command.NewSnapshot()
	_, ok := snap.(*predictionSnapshot)
	assert.True(t, ok)
}
