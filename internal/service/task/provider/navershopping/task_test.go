package navershopping

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	provider.ClearForTest()
	defer provider.ClearForTest()

	// 테스트용 기본 AppConfig 설정
	baseAppConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Data: map[string]interface{}{
					"client_id":     "test_id",
					"client_secret": "test_secret",
				},
				Commands: []config.CommandConfig{
					{
						ID: string(WatchPriceAnyCommand),
						Data: map[string]interface{}{
							"query": "테스트상품",
							"filters": map[string]interface{}{
								"price_less_than": 10000,
							},
						},
					},
				},
			},
		},
	}

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockStorage := &contractmocks.MockTaskResultStore{}

	tests := []struct {
		name        string
		params      provider.NewTaskParams
		expectError bool
		errType     error  // 에러 타입 검증용 (Errors.Is 사용)
		errMsg      string // 에러 메시지 포함 여부 (Contains 사용)
	}{
		{
			name: "성공: 정상적인 TaskID와 CommandID (설정 포함)",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig:   baseAppConfig,
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: false,
		},
		{
			name: "실패: 지원하지 않는 TaskID",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    "INVALID_TASK_ID",
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig:   baseAppConfig,
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errType:     provider.NewErrTaskNotSupported("INVALID_TASK_ID"),
		},
		{
			name: "실패: 지원하지 않는 CommandID (Prefix 불일치)",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: "INVALID_COMMAND_PREFIX",
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Data: map[string]interface{}{
								"client_id":     "id",
								"client_secret": "secret",
							},
							Commands: []config.CommandConfig{
								{ID: "INVALID_COMMAND_PREFIX", Data: map[string]interface{}{"query": "test"}}, // 설정 파일엔 존재하지만 프리픽스에서 탈락
							},
						},
					},
				},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errType:     provider.NewErrCommandNotSupported("INVALID_COMMAND_PREFIX", []contract.TaskCommandID{WatchPriceAnyCommand}),
		},
		{
			name: "실패: 설정 파일 내 Task 설정 누락(빈 Config)",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig:   &config.AppConfig{},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errType:     provider.ErrTaskNotFound,
		},
		{
			name: "실패: Task 설정 내 필수값(client_id) 누락",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Data: map[string]interface{}{
								// client_id 누락
								"client_secret": "secret",
							},
							Commands: []config.CommandConfig{
								{ID: string(WatchPriceAnyCommand), Data: map[string]interface{}{"query": "q", "filters": map[string]interface{}{"price_less_than": 1}}},
							},
						},
					},
				},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errMsg:      "client_id",
		},
		{
			name: "실패: Task 설정 내 필수값(client_secret) 누락",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Data: map[string]interface{}{
								"client_id": "id",
								// client_secret 누락
							},
							Commands: []config.CommandConfig{
								{ID: string(WatchPriceAnyCommand), Data: map[string]interface{}{"query": "q", "filters": map[string]interface{}{"price_less_than": 1}}},
							},
						},
					},
				},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errMsg:      "client_secret",
		},
		{
			name: "실패: Command 설정 내 필수값(query) 누락",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Data: map[string]interface{}{
								"client_id":     "id",
								"client_secret": "secret",
							},
							Commands: []config.CommandConfig{
								{
									ID: string(WatchPriceAnyCommand),
									Data: map[string]interface{}{
										// query 누락
										"filters": map[string]interface{}{"price_less_than": 100},
									},
								},
							},
						},
					},
				},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errMsg:      "query",
		},
		{
			name: "실패: Command 설정 내 필터 오류 (price_less_than <= 0)",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchPriceAnyCommand,
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Data: map[string]interface{}{
								"client_id":     "id",
								"client_secret": "secret",
							},
							Commands: []config.CommandConfig{
								{
									ID: string(WatchPriceAnyCommand),
									Data: map[string]interface{}{
										"query": "test_query",
										"filters": map[string]interface{}{
											"price_less_than": 0, // Invalid
										},
									},
								},
							},
						},
					},
				},
				Fetcher:     mockFetcher,
				Storage:     mockStorage,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			},
			expectError: true,
			errMsg:      "price_less_than",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdTask, err := newTask(tt.params)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, createdTask)
				if tt.errType != nil {
					assert.Contains(t, err.Error(), tt.errType.Error())
				}
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdTask)
				// 생성된 Task가 *navershopping.task 타입인지 확인
				assert.IsType(t, &task{}, createdTask)
				// Base가 올바르게 초기화되었는지 TaskID로 간접 검증
				assert.Equal(t, TaskID, createdTask.ID())
			}
		})
	}
}
