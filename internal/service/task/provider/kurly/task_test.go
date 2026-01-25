package kurly

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"
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

	invalidConfig_EmptyFile := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "KURLY",
				Commands: []config.CommandConfig{
					{
						ID: "WatchProductPrice",
						Data: map[string]interface{}{
							"watch_products_file": "   ", // 공백 문자열
						},
					},
				},
			},
		},
	}

	invalidConfig_MissingField := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: "KURLY",
				Commands: []config.CommandConfig{
					{
						ID: "WatchProductPrice",
						Data: map[string]interface{}{
							// "watch_products_file" key missing
							"other_field": "value",
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
				assert.NotNil(t, h)
				// 올바른 타입으로 캐스팅되는지 확인
				taskImpl, ok := h.(*task)
				assert.True(t, ok, "handler should be of type *task")

				// 기본 속성 검증
				assert.Equal(t, TaskID, taskImpl.GetID())
				assert.Equal(t, WatchProductPriceCommand, taskImpl.GetCommandID())
			},
		},
		{
			name: "실패: 지원하지 않는 Task TaskID",
			req: &contract.TaskSubmitRequest{
				TaskID:    "UNKNOWN_TASK",
				CommandID: WatchProductPriceCommand,
			},
			appConfig: validAppConfig,
			wantErr:   true,
			errMsg:    "지원하지 않는 작업입니다",
		},
		{
			name: "실패: 지원하지 않는 Command TaskID",
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
			appConfig: invalidConfig_MissingCommand,
			wantErr:   true,
			errMsg:    "해당 명령 생성에 필요한 설정 데이터가 존재하지 않습니다",
		},
		{
			name: "실패: 설정 유효성 검사 실패 (파일 확장자 오류)",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfig_NoCSV,
			wantErr:   true,
			errMsg:    "watch_products_file 설정에는 .csv 확장자를 가진 파일 경로만 지정할 수 있습니다",
		},
		{
			name: "실패: 설정 유효성 검사 실패 (파일명 공백)",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfig_EmptyFile,
			wantErr:   true,
			errMsg:    "watch_products_file이 입력되지 않았거나 공백입니다",
		},
		{
			name: "실패: 설정 유효성 검사 실패 (필수 필드 누락)",
			req: &contract.TaskSubmitRequest{
				TaskID:    TaskID,
				CommandID: WatchProductPriceCommand,
			},
			appConfig: invalidConfig_MissingField,
			wantErr:   true,
			errMsg:    "watch_products_file이 입력되지 않았거나 공백입니다", // 필수 필드 확인 실패 메시지
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
