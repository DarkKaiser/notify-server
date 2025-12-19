package naver_shopping

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewTask_InvalidCommand(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()
	req := &tasksvc.SubmitRequest{
		TaskID:    ID,
		CommandID: "InvalidCommandID",
	}
	// 유효한 설정 제공 (설정 검증 통과용)
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(ID),
				Data: map[string]interface{}{
					"client_id":     "test_id",
					"client_secret": "test_secret",
				},
			},
		},
	}

	_, err := createTask("test_instance", req, appConfig, mockFetcher)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
}

func TestNaverShoppingConfig_Validate(t *testing.T) {
	t.Run("정상적인 데이터", func(t *testing.T) {
		taskConfig := &taskConfig{
			ClientID:     "test_client_id",
			ClientSecret: "test_client_secret",
		}

		err := taskConfig.validate()
		assert.NoError(t, err, "정상적인 데이터는 검증을 통과해야 합니다")
	})

	t.Run("ClientID가 비어있는 경우", func(t *testing.T) {
		taskConfig := &taskConfig{
			ClientID:     "",
			ClientSecret: "test_client_secret",
		}

		err := taskConfig.validate()
		assert.Error(t, err, "ClientID가 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "client_id", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("ClientSecret이 비어있는 경우", func(t *testing.T) {
		taskConfig := &taskConfig{
			ClientID:     "test_client_id",
			ClientSecret: "",
		}

		err := taskConfig.validate()
		assert.Error(t, err, "ClientSecret이 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "client_secret", "적절한 에러 메시지를 반환해야 합니다")
	})
}
