package naver

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ConfigBuilder AppConfig 생성을 돕는 빌더입니다.
type ConfigBuilder struct {
	config *config.AppConfig
}

func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &config.AppConfig{
			Tasks: []config.TaskConfig{},
		},
	}
}

func (b *ConfigBuilder) WithTask(taskID string, commandID string, data map[string]interface{}) *ConfigBuilder {
	b.config.Tasks = append(b.config.Tasks, config.TaskConfig{
		ID: taskID,
		Commands: []config.CommandConfig{
			{
				ID:   commandID,
				Data: data,
			},
		},
	})
	return b
}

func (b *ConfigBuilder) Build() *config.AppConfig {
	return b.config
}

func defaultTaskData() map[string]interface{} {
	return map[string]interface{}{
		"query": "뮤지컬",
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
	}
}

func TestCreateTask(t *testing.T) {
	mockFetcher := testutil.NewMockHTTPFetcher()

	tests := []struct {
		name          string
		req           *tasksvc.SubmitRequest
		appConfig     *config.AppConfig
		expectedError string
		validate      func(t *testing.T, handler tasksvc.Handler)
	}{
		{
			name: "성공: 정상적인 요청과 설정으로 Task 생성",
			req: &tasksvc.SubmitRequest{
				TaskID:     ID,
				CommandID:  WatchNewPerformancesCommand,
				NotifierID: "telegram",
				RunBy:      tasksvc.RunByUser,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(ID), string(WatchNewPerformancesCommand), defaultTaskData()).
				Build(),
			validate: func(t *testing.T, handler tasksvc.Handler) {
				require.NotNil(t, handler)
				naverTask, ok := handler.(*task)
				require.True(t, ok, "Handler should be of type *task")
				assert.Equal(t, ID, naverTask.GetID())
				assert.Equal(t, WatchNewPerformancesCommand, naverTask.GetCommandID())
				assert.Equal(t, "telegram", naverTask.GetNotifierID())
			},
		},
		{
			name: "실패: 지원하지 않는 Task ID",
			req: &tasksvc.SubmitRequest{
				TaskID:    "INVALID_TASK",
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig:     NewConfigBuilder().Build(),
			expectedError: tasksvc.ErrTaskNotSupported.Error(),
		},
		{
			name: "실패: 지원하지 않는 Command ID",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: "INVALID_COMMAND",
			},
			appConfig:     NewConfigBuilder().Build(),
			expectedError: "지원하지 않는 명령입니다",
		},
		{
			name: "실패: Config 찾을 수 없음 (Task ID 불일치)",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: NewConfigBuilder().
				WithTask("OTHER_TASK", string(WatchNewPerformancesCommand), defaultTaskData()).
				Build(),
			expectedError: "해당 명령 생성에 필요한 구성 정보가 존재하지 않습니다",
		},
		{
			name: "실패: Config 찾을 수 없음 (Command ID 불일치)",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(ID), "OTHER_COMMAND", defaultTaskData()).
				Build(),
			expectedError: "해당 명령 생성에 필요한 구성 정보가 존재하지 않습니다",
		},
		{
			name: "실패: 유효하지 않은 설정 데이터 (Query 누락) - FailFast",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(ID), string(WatchNewPerformancesCommand), map[string]interface{}{
					// query missing
				}).
				Build(),
			expectedError: "query가 입력되지 않았습니다",
		},
		{
			name: "실패: 잘못된 타입의 설정 데이터",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchNewPerformancesCommand,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(ID), string(WatchNewPerformancesCommand), map[string]interface{}{
					"query": 12345, // string expected, int provided
				}).
				Build(),
			expectedError: "명령 데이터가 유효하지 않습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, err := createTask("test_instance", tt.req, tt.appConfig, mockFetcher)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, handler)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, handler)
				}
			}
		})
	}
}
