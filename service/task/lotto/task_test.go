package lotto

import (
	"testing"

	appconfig "github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNewTask_Success(t *testing.T) {
	cfgLookup, err := tasksvc.FindConfigForTest(ID, PredictionCommand)
	assert.NoError(t, err)
	assert.NotNil(t, cfgLookup)

	tmpDir := t.TempDir()
	appConfig := &appconfig.AppConfig{
		Tasks: []appconfig.TaskConfig{
			{
				ID:   string(ID),
				Data: map[string]interface{}{"app_path": tmpDir},
			},
		},
	}

	req := &tasksvc.SubmitRequest{
		TaskID:     ID,
		CommandID:  PredictionCommand,
		NotifierID: "telegram",
		RunBy:      tasksvc.RunByUser,
	}

	handler, err := cfgLookup.Task.NewTask("test-instance", req, appConfig)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	lottoTask, ok := handler.(*task)
	assert.True(t, ok)
	assert.Equal(t, tmpDir, lottoTask.appPath)
	assert.Equal(t, ID, lottoTask.GetID())
	assert.Equal(t, PredictionCommand, lottoTask.GetCommandID())

	_, ok = lottoTask.executor.(*defaultCommandExecutor)
	assert.True(t, ok)
}

func TestCreateTask_WithMockExecutor(t *testing.T) {
	mockExecutor := new(MockCommandExecutor)

	req := &tasksvc.SubmitRequest{
		TaskID:     ID,
		CommandID:  PredictionCommand,
		NotifierID: "telegram",
		RunBy:      tasksvc.RunByUser,
	}
	appConfig := &appconfig.AppConfig{} // AppConfig is not used for internal logic validation in this test, but verified in createTask

	// createTask internally expects AppPath to be valid if loaded from config,
	// but here we are injecting executor.
	// Wait, createTask checks AppConfig for AppPath.
	// So we must provide valid AppConfig.

	tmpDir := t.TempDir()
	appConfig = &appconfig.AppConfig{
		Tasks: []appconfig.TaskConfig{
			{
				ID:   string(ID),
				Data: map[string]interface{}{"app_path": tmpDir},
			},
		},
	}

	handler, err := createTask("test-instance", req, appConfig, mockExecutor)
	assert.NoError(t, err)
	assert.NotNil(t, handler)

	lottoTask, ok := handler.(*task)
	assert.True(t, ok)

	// Verify that the executor is the one we passed
	assert.Equal(t, mockExecutor, lottoTask.executor)
}

func TestNewTask_InvalidAppPath(t *testing.T) {
	cfgLookup, _ := tasksvc.FindConfigForTest(ID, PredictionCommand)

	tests := []struct {
		name        string
		appPath     string
		expectedErr string
	}{
		{
			name:        "Empty AppPath",
			appPath:     "",
			expectedErr: "Lotto Task의 AppPath 설정이 비어있습니다", // Fail Fast 재적용 확인
		},
		{
			name:        "Non-existent AppPath",
			appPath:     "C:\\NonExistent\\Path\\For\\Test",
			expectedErr: "설정된 AppPath 경로가 존재하지 않습니다", // Fail Fast 재적용 확인
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appConfig := &appconfig.AppConfig{
				Tasks: []appconfig.TaskConfig{
					{
						ID:   string(ID),
						Data: map[string]interface{}{"app_path": tt.appPath},
					},
				},
			}
			req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}

			_, err := cfgLookup.Task.NewTask("test", req, appConfig)

			// Fail Fast 로직이 복구되었으므로 이제 MUST Error
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestNewTask_ConfigNotFound(t *testing.T) {
	cfgLookup, _ := tasksvc.FindConfigForTest(ID, PredictionCommand)
	req := &tasksvc.SubmitRequest{TaskID: ID, CommandID: PredictionCommand}
	// Empty config (no tasks)
	appConfig := &appconfig.AppConfig{Tasks: []appconfig.TaskConfig{}}

	_, err := cfgLookup.Task.NewTask("test", req, appConfig)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Lotto 작업을 위한 설정을 찾을 수 없습니다")
}

func TestNewTask_RegistrationCheck(t *testing.T) {
	assert.Equal(t, tasksvc.ID("LOTTO"), ID)
	assert.Equal(t, tasksvc.CommandID("Prediction"), PredictionCommand)

	cfgLookup, err := tasksvc.FindConfigForTest(ID, PredictionCommand)
	assert.NoError(t, err)

	snapshot := cfgLookup.Command.NewSnapshot()
	assert.NotNil(t, snapshot)
	_, ok := snapshot.(*predictionSnapshot)
	assert.True(t, ok)
}

func TestNewTask_InvalidCommand(t *testing.T) {
	cfgLookup, _ := tasksvc.FindConfigForTest(ID, PredictionCommand)

	// 유효한 AppConfig 준비 (TaskID 검사는 통과해야 함)
	tmpDir := t.TempDir()
	appConfig := &appconfig.AppConfig{
		Tasks: []appconfig.TaskConfig{
			{
				ID:   string(ID),
				Data: map[string]interface{}{"app_path": tmpDir},
			},
		},
	}

	// 잘못된 CommandID 요청
	req := &tasksvc.SubmitRequest{
		TaskID:    ID,
		CommandID: "InvalidCommandID",
	}

	_, err := cfgLookup.Task.NewTask("test", req, appConfig)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
}
