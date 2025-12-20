package navershopping

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskSettings_Validate(t *testing.T) {
	tests := []struct {
		name        string
		settings    taskSettings
		expectedErr string
	}{
		{
			name: "정상적인 설정",
			settings: taskSettings{
				ClientID:     "test_id",
				ClientSecret: "test_secret",
			},
			expectedErr: "",
		},
		{
			name: "ClientID 누락",
			settings: taskSettings{
				ClientID:     "",
				ClientSecret: "test_secret",
			},
			expectedErr: "client_id",
		},
		{
			name: "ClientSecret 누락",
			settings: taskSettings{
				ClientID:     "test_id",
				ClientSecret: "",
			},
			expectedErr: "client_secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.validate()
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateTask(t *testing.T) {
	// 정상적인 AppConfig 설정
	validAppConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(ID),
				Data: map[string]interface{}{
					"client_id":     "test_id",
					"client_secret": "test_secret",
				},
				Commands: []config.CommandConfig{
					{
						ID: "WatchPrice_Item1",
						Data: map[string]interface{}{
							"query": "test_query",
							"filters": map[string]interface{}{
								"price_less_than": 10000,
							},
						},
					},
				},
			},
		},
	}

	mockFetcher := testutil.NewMockHTTPFetcher()

	t.Run("정상 생성 (WatchPrice 커맨드)", func(t *testing.T) {
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "WatchPrice_Item1",
		}

		handler, err := createTask("instance_1", req, validAppConfig, mockFetcher)
		require.NoError(t, err)
		assert.NotNil(t, handler)
	})

	t.Run("실패: 지원하지 않는 TaskID", func(t *testing.T) {
		req := &tasksvc.SubmitRequest{
			TaskID:    "INVALID_TASK",
			CommandID: "WatchPrice_Item1",
		}

		_, err := createTask("instance_1", req, validAppConfig, mockFetcher)
		assert.ErrorIs(t, err, tasksvc.ErrTaskNotSupported)
	})

	t.Run("실패: Task 설정 누락 (AppConfig에 해당 Task 없음)", func(t *testing.T) {
		emptyConfig := &config.AppConfig{}
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "WatchPrice_Item1",
		}

		_, err := createTask("instance_1", req, emptyConfig, mockFetcher)
		assert.Error(t, err)
		// settings가 zero value일 때 validate 실패 메시지 확인
		assert.Contains(t, err.Error(), "client_id")
	})

	t.Run("실패: Task 설정 유효성 검사 실패 (필수값 누락)", func(t *testing.T) {
		invalidConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID: string(ID),
					Data: map[string]interface{}{
						// client_id 누락
						"client_secret": "test_secret",
					},
				},
			},
		}
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "WatchPrice_Item1",
		}

		_, err := createTask("instance_1", req, invalidConfig, mockFetcher)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client_id")
	})

	t.Run("실패: 지원하지 않는 CommandID (prefix 불일치)", func(t *testing.T) {
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "UnknownAndUnprefixed",
		}

		_, err := createTask("instance_1", req, validAppConfig, mockFetcher)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "지원하지 않는 명령")
	})

	t.Run("실패: Command 설정 누락", func(t *testing.T) {
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "WatchPrice_Item_Not_In_Config",
		}

		_, err := createTask("instance_1", req, validAppConfig, mockFetcher)
		assert.ErrorIs(t, err, tasksvc.ErrCommandSettingsNotFound)
	})

	t.Run("실패: Command 설정 유효성 검사 실패 (Query 누락)", func(t *testing.T) {
		invalidCmdConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID: string(ID),
					Data: map[string]interface{}{
						"client_id":     "test_id",
						"client_secret": "test_secret",
					},
					Commands: []config.CommandConfig{
						{
							ID: "WatchPrice_Invalid",
							Data: map[string]interface{}{
								"query": "", // Query 누락
								"filters": map[string]interface{}{
									"price_less_than": 10000,
								},
							},
						},
					},
				},
			},
		}

		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: "WatchPrice_Invalid",
		}

		_, err := createTask("instance_1", req, invalidCmdConfig, mockFetcher)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query")
	})
}
