package naver

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupValidAppConfig Helper 함수: 유효한 AppConfig를 생성합니다.
func setupValidAppConfig(query string) *config.AppConfig {
	return &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(ID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchNewPerformancesCommand),
						Data: map[string]interface{}{
							"query": query,
							"filters": map[string]interface{}{
								"title": map[string]interface{}{
									"included_keywords": "",
									"excluded_keywords": "",
								},
								"place": map[string]interface{}{
									"included_keywords": "",
									"excluded_keywords": "",
								},
							},
						},
					},
				},
			},
		},
	}
}

func TestCreateTask_Comprehensive(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()

	tests := []struct {
		name          string
		req           *tasksvc.SubmitRequest
		appConfig     *config.AppConfig
		expectedError string
	}{
		{
			name: "Success",
			req: &tasksvc.SubmitRequest{
				TaskID:     ID,
				CommandID:  WatchNewPerformancesCommand,
				NotifierID: "telegram",
				RunBy:      tasksvc.RunByUser,
			},
			appConfig:     setupValidAppConfig("뮤지컬"),
			expectedError: "",
		},
		{
			name: "Invalid TaskID",
			req: &tasksvc.SubmitRequest{
				TaskID:    "INVALID_TASK",
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig:     &config.AppConfig{},
			expectedError: tasksvc.ErrTaskNotSupported.Error(),
		},
		{
			name: "Invalid CommandID",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: "InvalidCommandID",
			},
			appConfig:     &config.AppConfig{},
			expectedError: "지원하지 않는 명령입니다",
		},
		{
			name: "Config Not Found",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig:     &config.AppConfig{Tasks: []config.TaskConfig{}},
			expectedError: "해당 명령 생성에 필요한 구성 정보가 존재하지 않습니다",
		},
		{
			name: "Empty Query (Validation Failure)",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(ID),
						Commands: []config.CommandConfig{
							{
								ID: string(WatchNewPerformancesCommand),
								Data: map[string]interface{}{
									"query": "", // Empty query
								},
							},
						},
					},
				},
			},
			expectedError: "query가 입력되지 않았습니다",
		},
		{
			name: "Invalid Command Data Format",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: string(ID),
						Commands: []config.CommandConfig{
							{
								ID: string(WatchNewPerformancesCommand),
								Data: map[string]interface{}{
									"query": 12345, // Invalid type (should be string)
								},
							},
						},
					},
				},
			},
			expectedError: "명령 데이터가 유효하지 않습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := createTask("test_instance", tt.req, tt.appConfig, mockFetcher)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, handler)
			} else {
				require.NoError(t, err)
				require.NotNil(t, handler)

				// Handler 타입 검증
				naverTask, ok := handler.(*task)
				assert.True(t, ok)
				assert.Equal(t, ID, naverTask.GetID())
				assert.Equal(t, WatchNewPerformancesCommand, naverTask.GetCommandID())
			}
		})
	}
}

func TestCreateTask_FailFast_ConfigLookup(t *testing.T) {
	t.Run("Config Lookup은 Task 생성 시점에 수행됨 (Fail-Fast)", func(t *testing.T) {
		mockFetcher := testutil.NewMockHTTPFetcher()
		req := &tasksvc.SubmitRequest{
			TaskID:    ID,
			CommandID: WatchNewPerformancesCommand,
		}

		// 잘못된 설정을 가진 AppConfig
		invalidConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{
				{
					ID: string(ID),
					Commands: []config.CommandConfig{
						{
							ID:   string(WatchNewPerformancesCommand),
							Data: map[string]interface{}{}, // query 누락
						},
					},
				},
			},
		}

		// createTask 호출 시점에 즉시 에러 발생 (Fail-Fast)
		_, err := createTask("test_instance", req, invalidConfig, mockFetcher)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "query가 입력되지 않았습니다")
	})
}

func TestNewTask_InvalidCommand(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()
	req := &tasksvc.SubmitRequest{
		TaskID:    ID,
		CommandID: "InvalidCommandID",
	}
	appConfig := &config.AppConfig{}

	_, err := createTask("test_instance", req, appConfig, mockFetcher)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
}
