package kurly

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestAppConfig 테스트용 AppConfig를 생성하는 헬퍼 함수입니다.
func newTestAppConfig(taskID string, commandID string, data map[string]interface{}) *config.AppConfig {
	return &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: taskID,
				Commands: []config.CommandConfig{
					{
						ID:   commandID,
						Data: data,
					},
				},
			},
		},
	}
}

func TestNewTask(t *testing.T) {
	t.Parallel()

	// -------------------------------------------------------------------------
	// Fixtures
	// -------------------------------------------------------------------------

	validAppConfig := newTestAppConfig(
		string(TaskID),
		string(WatchProductPriceCommand),
		map[string]interface{}{
			"watch_list_file": "test_products.csv",
		},
	)

	invalidConfigNoCSV := newTestAppConfig(
		string(TaskID),
		string(WatchProductPriceCommand),
		map[string]interface{}{
			"watch_list_file": "invalid_extension.txt", // .csv required
		},
	)

	invalidConfigEmptyFile := newTestAppConfig(
		string(TaskID),
		string(WatchProductPriceCommand),
		map[string]interface{}{
			"watch_list_file": "   ", // 공백 문자열
		},
	)

	invalidConfigMissingField := newTestAppConfig(
		string(TaskID),
		string(WatchProductPriceCommand),
		map[string]interface{}{
			"wrong_field": "value",
		},
	)

	invalidConfigMissingCommand := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID:       string(TaskID),
				Commands: []config.CommandConfig{}, // Empty commands
			},
		},
	}

	// -------------------------------------------------------------------------
	// Table-Driven Tests
	// -------------------------------------------------------------------------

	tests := []struct {
		name      string
		req       *contract.TaskSubmitRequest
		appConfig *config.AppConfig
		wantErr   bool
		errMsg    string
		checkTask func(*testing.T, provider.Task)
	}{
		{
			name: "성공: 정상적인 작업 생성 (WatchProductPrice)",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: validAppConfig,
			wantErr:   false,
			checkTask: func(t *testing.T, h provider.Task) {
				require.NotNil(t, h)

				taskImpl, ok := h.(*task)
				require.True(t, ok, "반환된 Task는 *task 타입이어야 합니다")

				assert.Equal(t, TaskID, taskImpl.ID())
				assert.Equal(t, WatchProductPriceCommand, taskImpl.CommandID())
			},
		},
		{
			name: "실패: 지원하지 않는 Task ID",
			req: &contract.TaskSubmitRequest{
				TaskID:    "UNKNOWN_TASK",
				CommandID: WatchProductPriceCommand,
			},
			appConfig: validAppConfig,
			wantErr:   true,
			errMsg:    "지원하지 않는 작업입니다",
		},
		{
			name: "실패: 지원하지 않는 Command ID",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: "UnknownCommand",
			},
			appConfig: validAppConfig,
			wantErr:   true,
			errMsg:    "지원하지 않는 명령입니다",
		},
		{
			name: "실패: 설정에서 Command 정보를 찾을 수 없음",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfigMissingCommand,
			wantErr:   true,
			errMsg:    "해당 명령을 찾을 수 없습니다",
		},
		{
			name: "실패: watch_list_file 확장자 오류",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfigNoCSV,
			wantErr:   true,
			errMsg:    "watch_list_file은 .csv 파일 경로여야 합니다",
		},
		{
			name: "실패: watch_list_file 설정 공백 오류",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfigEmptyFile,
			wantErr:   true,
			errMsg:    "watch_list_file이 설정되지 않았거나 공백입니다",
		},
		{
			name: "실패: watch_list_file 설정 누락 오류",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfigMissingField,
			wantErr:   true,
			errMsg:    "watch_list_file이 설정되지 않았거나 공백입니다",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel execution
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()

			params := provider.NewTaskParams{
				InstanceID: "test_instance",
				Request:    tt.req,
				AppConfig:  tt.appConfig,
				Storage:    nil, // 테스트 시 Storage는 nil로 주입
				Fetcher:    mockFetcher,
				NewSnapshot: func() any {
					return &watchProductPriceSnapshot{}
				},
			}

			got, err := newTask(params)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, got)

				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				require.NoError(t, err)
				if tt.checkTask != nil {
					tt.checkTask(t, got)
				}
			}
		})
	}
}
