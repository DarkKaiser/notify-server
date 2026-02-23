package naver

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
				Commands: []config.CommandConfig{
					{
						ID: string(WatchNewPerformancesCommand),
						Data: map[string]interface{}{
							"query":               "뮤지컬",
							"max_pages":           50,
							"page_fetch_delay_ms": 100,
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
		errType     error // 에러 타입 검증용 (Errors.Is 사용)
	}{
		{
			name: "성공: 정상적인 TaskID와 CommandID",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchNewPerformancesCommand,
				},
				AppConfig: baseAppConfig,
				Fetcher:   mockFetcher,
				Storage:   mockStorage,
			},
			expectError: false,
		},
		{
			name: "실패: 지원하지 않는 TaskID",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    "INVALID_TASK_ID",
					CommandID: WatchNewPerformancesCommand,
				},
				AppConfig: baseAppConfig,
				Fetcher:   mockFetcher,
				Storage:   mockStorage,
			},
			expectError: true,
			errType:     provider.NewErrTaskNotSupported("INVALID_TASK_ID"),
		},
		{
			name: "실패: 지원하지 않는 CommandID",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: "INVALID_COMMAND_ID",
				},
				AppConfig: baseAppConfig,
				Fetcher:   mockFetcher,
				Storage:   mockStorage,
			},
			expectError: true,
			errType:     provider.NewErrCommandNotSupported("INVALID_COMMAND_ID", []contract.TaskCommandID{WatchNewPerformancesCommand}),
		},
		{
			name: "실패: AppConfig에 Command 설정 누락",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchNewPerformancesCommand,
				},
				AppConfig: &config.AppConfig{}, // 설정 없음
				Fetcher:   mockFetcher,
				Storage:   mockStorage,
			},
			expectError: true,
			// 구체적인 에러 타입은 provider.ErrCommandSettingsNotFound 계열이나, 여기서는 에러 발생 자체를 중요하게 봅니다.
		},
		{
			name: "실패: Command 설정은 있으나 유효성 검증(Validate) 실패",
			params: provider.NewTaskParams{
				Request: &contract.TaskSubmitRequest{
					TaskID:    TaskID,
					CommandID: WatchNewPerformancesCommand,
				},
				AppConfig: &config.AppConfig{
					Tasks: []config.TaskConfig{
						{
							ID: string(TaskID),
							Commands: []config.CommandConfig{
								{
									ID:   string(WatchNewPerformancesCommand),
									Data: map[string]interface{}{
										// 필수값 "query" 누락으로 Validate() 에러 유도
									},
								},
							},
						},
					},
				},
				Fetcher: mockFetcher,
				Storage: mockStorage,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			createdTask, err := newTask(tt.params)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, createdTask)
				if tt.errType != nil {
					// 에러 메시지 내용으로 타입 비교 수렴
					assert.Contains(t, err.Error(), tt.errType.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, createdTask)
				// 생성된 Task가 *naver.task 타입인지 확인
				assert.IsType(t, &task{}, createdTask)
				// Base가 올바르게 초기화되었는지 TaskID로 간접 검증
				assert.Equal(t, TaskID, createdTask.ID())
			}
		})
	}
}
