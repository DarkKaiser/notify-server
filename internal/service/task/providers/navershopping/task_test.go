package navershopping

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	tasksvc "github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/darkkaiser/notify-server/internal/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTaskSettings_Validate_TableDriven 유효성 검사 테스트를 테이블 기반으로 구조화합니다.
func TestTaskSettings_Validate_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		settings  taskSettings
		wantError string
	}{
		{
			name: "성공: 필수 필드가 모두 존재함",
			settings: taskSettings{
				ClientID:     "valid_id",
				ClientSecret: "valid_secret",
			},
			wantError: "",
		},
		{
			name: "실패: ClientID 누락 (공백)",
			settings: taskSettings{
				ClientID:     "   ",
				ClientSecret: "valid_secret",
			},
			wantError: "client_id",
		},
		{
			name: "실패: ClientID 누락 (빈 문자열)",
			settings: taskSettings{
				ClientID:     "",
				ClientSecret: "valid_secret",
			},
			wantError: "client_id",
		},
		{
			name: "실패: ClientSecret 누락 (공백)",
			settings: taskSettings{
				ClientID:     "valid_id",
				ClientSecret: "   ",
			},
			wantError: "client_secret",
		},
		{
			name: "실패: ClientSecret 누락 (빈 문자열)",
			settings: taskSettings{
				ClientID:     "valid_id",
				ClientSecret: "",
			},
			wantError: "client_secret",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.settings.validate()
			if tt.wantError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCreateTask_TableDriven CreateTask 함수의 다양한 시나리오를 검증합니다.
func TestCreateTask_TableDriven(t *testing.T) {
	t.Parallel()

	mockFetcher := testutil.NewMockHTTPFetcher()

	// 공통적으로 사용될 Constants
	const (
		validTaskID      = TaskID
		validCommandID   = WatchPriceAnyCommand
		invalidTaskID    = contract.TaskID("INVALTaskID_TASK")
		invalidCommandID = contract.TaskCommandID("Invalid_Command")
	)

	tests := []struct {
		name       string
		req        *contract.TaskSubmitRequest
		appConfig  *config.AppConfig
		wantErr    error  // 특정 에러 타입 확인 (errors.Is)
		wantErrMsg string // 에러 메시지 내용 확인 (Contains)
	}{
		{
			name: "성공: 정상적인 요청 및 설정",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "test_id", "test_secret").
				WithCommand(string(validCommandID), "test_query").
				Build(),
			wantErr: nil,
		},
		{
			name: "실패: 지원하지 않는 TaskID",
			req: &contract.TaskSubmitRequest{
				TaskID:    invalidTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", "secret").
				WithCommand(string(validCommandID), "q").
				Build(),
			wantErr: tasksvc.ErrTaskNotSupported,
		},
		{
			name: "실패: AppConfig 내 Task 설정 없음 (빈 Config)",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: &config.AppConfig{}, // Empty config
			wantErr:   tasksvc.ErrTaskSettingsNotFound,
		},
		{
			name: "실패: Task 필수 설정(ClientID) 누락",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "", "secret"). // ClientID Missing
				WithCommand(string(validCommandID), "q").
				Build(),
			wantErrMsg: "client_id",
		},
		{
			name: "실패: Task 필수 설정(ClientSecret) 누락",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", ""). // Secret Missing
				WithCommand(string(validCommandID), "q").
				Build(),
			wantErrMsg: "client_secret",
		},
		{
			name: "실패: 지원하지 않는 CommandID (Prefix 불일치)",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: invalidCommandID, // "WatchPrice_"로 시작하지 않음
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", "secret").
				// Config에는 있어도 Handler 생성 시 Prefix 체크에서 탈락함
				WithCommand(string(invalidCommandID), "q").
				Build(),
			wantErrMsg: "지원하지 않는 명령입니다", // NewErrCommandNotSupported 메시지
		},
		{
			name: "실패: Config에 Command 설정이 존재하지 않음",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", "secret").
				WithCommand("OtherCommand", "q"). // 다른 커맨드만 있음
				Build(),
			wantErr: tasksvc.ErrCommandSettingsNotFound,
		},
		{
			name: "실패: Command 필수 설정(Query) 누락",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", "secret").
				WithCommand(string(validCommandID), ""). // Query Missing
				Build(),
			wantErrMsg: "query", // validate error
		},
		{
			name: "실패: Command 설정 값 오류 (PriceLessThan <= 0)",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: NewConfigBuilder().
				WithTask(string(validTaskID), "id", "secret").
				WithCommand(string(validCommandID), "q", func(m map[string]interface{}) {
					filters := m["filters"].(map[string]interface{})
					filters["price_less_than"] = 0 // Invalid Value
				}).
				Build(),
			wantErrMsg: "price_less_than", // validate error
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler, err := createTask("instance_1", tt.req, tt.appConfig, mockFetcher)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, handler)
			} else if tt.wantErrMsg != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
				assert.Nil(t, handler)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, handler)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Test Helper: ConfigBuilder
// -----------------------------------------------------------------------------

// ConfigBuilder AppConfig 생성을 도와주는 빌더입니다. (Test Helper 패턴)
type ConfigBuilder struct {
	tasks []config.TaskConfig
}

func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{}
}

func (b *ConfigBuilder) WithTask(taskInstanceID, clientTaskID, clientSecret string) *ConfigBuilder {
	b.tasks = append(b.tasks, config.TaskConfig{
		ID: taskInstanceID,
		Data: map[string]interface{}{
			"client_id":     clientTaskID,
			"client_secret": clientSecret,
		},
		Commands: []config.CommandConfig{}, // Initialize empty commands
	})
	return b
}

type CommandOption func(map[string]interface{})

func (b *ConfigBuilder) WithCommand(commandTaskID, query string, opts ...CommandOption) *ConfigBuilder {
	// 마지막으로 추가된 Task에 Command를 추가합니다.
	if len(b.tasks) == 0 {
		panic("WithCommand called before WithTask")
	}

	data := map[string]interface{}{
		"query": query,
		"filters": map[string]interface{}{
			"price_less_than": 10000,
		},
	}

	for _, opt := range opts {
		opt(data)
	}

	lastIdx := len(b.tasks) - 1
	b.tasks[lastIdx].Commands = append(b.tasks[lastIdx].Commands, config.CommandConfig{
		ID:   commandTaskID,
		Data: data,
	})
	return b
}

func (b *ConfigBuilder) Build() *config.AppConfig {
	return &config.AppConfig{
		Tasks: b.tasks,
	}
}
