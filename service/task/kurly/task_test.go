package kurly

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTask_TableDriven(t *testing.T) {
	t.Parallel()

	// -------------------------------------------------------------------------
	// Fixtures & Helpers
	// -------------------------------------------------------------------------
	validAppConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "KURLY",
				Commands: []config.CommandConfig{
					{
						ID: "WatchProductPrice",
						Data: map[string]interface{}{
							"watch_products_file": "test_products.csv",
						},
					},
				},
			},
		},
	}

	invalidConfig_NoCSV := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "KURLY",
				Commands: []config.CommandConfig{
					{
						ID: "WatchProductPrice",
						Data: map[string]interface{}{
							"watch_products_file": "invalid_extension.txt", // .csv required
						},
					},
				},
			},
		},
	}

	invalidConfig_MissingCommand := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID:       "KURLY",
				Commands: []config.CommandConfig{}, // Empty commands
			},
		},
	}

	// -------------------------------------------------------------------------
	// Test Cases
	// -------------------------------------------------------------------------
	tests := []struct {
		name      string
		req       *tasksvc.SubmitRequest
		appConfig *config.AppConfig
		wantErr   bool
		errMsg    string
		checkTask func(*testing.T, tasksvc.Handler)
	}{
		{
			name: "성공: 정상적인 작업 생성 (WatchProductPrice)",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: validAppConfig,
			wantErr:   false,
			checkTask: func(t *testing.T, h tasksvc.Handler) {
				assert.NotNil(t, h)
				// 올바른 타입으로 캐스팅되는지 확인
				_, ok := h.(*task)
				assert.True(t, ok, "handler should be of type *task")

				// 실행 함수가 설정되었는지 확인
				// (Integration Test가 아니므로 실제 실행 로직까지는 검증하지 않음)
			},
		},
		{
			name: "실패: 지원하지 않는 Task ID",
			req: &tasksvc.SubmitRequest{
				TaskID:    "UNKNOWN_TASK",
				CommandID: WatchProductPriceCommand,
			},
			appConfig: validAppConfig,
			wantErr:   true,
			errMsg:    "지원하지 않는 작업입니다",
		},
		{
			name: "실패: 지원하지 않는 Command ID",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: "UnknownCommand",
			},
			appConfig: validAppConfig,
			wantErr:   true,
			errMsg:    "지원하지 않는 명령입니다",
		},
		{
			name: "실패: 설정에서 Command 정보를 찾을 수 없음",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfig_MissingCommand,
			wantErr:   true,
			errMsg:    "해당 명령 생성에 필요한 설정 데이터가 존재하지 않습니다",
		},
		{
			name: "실패: 설정 유효성 검사 실패 (파일 확장자)",
			req: &tasksvc.SubmitRequest{
				TaskID:    ID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfig_NoCSV,
			wantErr:   true,
			errMsg:    "명령 설정 데이터가 유효하지 않습니다",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := testutil.NewMockHTTPFetcher()
			got, err := createTask("test_instance", tt.req, tt.appConfig, mockFetcher)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, got)
			} else {
				require.NoError(t, err)
				if tt.checkTask != nil {
					tt.checkTask(t, got)
				}
			}
		})
	}
}
