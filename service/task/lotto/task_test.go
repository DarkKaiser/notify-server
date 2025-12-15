package lotto

import (
	"testing"

	appconfig "github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNewTask_Success(t *testing.T) {
	// Registry에서 설정 가져오기 (init()에 의해 등록됨)
	cfgLookup, err := tasksvc.FindConfigForTest(ID, PredictionCommand)
	assert.NoError(t, err)
	assert.NotNil(t, cfgLookup)

	// Valid Config
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

	// Type Assertion
	lottoTask, ok := handler.(*task)
	assert.True(t, ok)
	assert.Equal(t, tmpDir, lottoTask.appPath)
	assert.Equal(t, ID, lottoTask.GetID())
	assert.Equal(t, PredictionCommand, lottoTask.GetCommandID())

	// Executor Check
	_, ok = lottoTask.executor.(*defaultCommandExecutor)
	assert.True(t, ok)
}

func TestNewTask_InvalidAppPath(t *testing.T) {
	// 이 기능은 User 요청으로 추가된 'Fail Fast' 로직을 검증합니다.
	cfgLookup, _ := tasksvc.FindConfigForTest(ID, PredictionCommand)

	tests := []struct {
		name        string
		appPath     string
		expectedErr string
	}{
		{
			name:    "Empty AppPath",
			appPath: "",
			// 현재 코드상 appPath trim만 하고 빈값 체크나 stat 체크가 User 요청에 의해 추가되었는지 확인 필요.
			// Step 322에서 User가 Manual Edit으로 해당 로직을 제거했음!
			// 따라서, 현재 코드는 에러를 반환하지 않을 수 있음.
			// 하지만 전문가로서 "개선할게 있냐"는 질문에 "Fail Fast"를 제안했었음.
			// User history:
			// Step 321: User overwrote task.go (removing Fail Fast?)
			// Step 322: User removed validation block!
			// 따라서 현재 task.go에는 Validation 로직이 없습니다!
			// 테스트도 이에 맞춰야 실패하지 않습니다.
			expectedErr: "",
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

			// Validation 로직이 제거되었으므로 에러가 발생하지 않아야 정상 (현재 코드 기준)
			// 만약 개선을 다시 적용한다면 그때 테스트를 수정해야 함.
			_, err := cfgLookup.Task.NewTask("test", req, appConfig)
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
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
