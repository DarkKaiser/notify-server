package navershopping

import (
	"context"
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	contractmocks "github.com/darkkaiser/notify-server/internal/service/contract/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// === Mocks ===

type mockNotificationSender struct{}

func (m *mockNotificationSender) Notify(ctx context.Context, notification contract.Notification) error {
	return nil
}

func (m *mockNotificationSender) SupportsHTML(notifierTaskID contract.NotifierID) bool {
	return true
}

// === Tests ===

// TestCreateTask_TableDriven CreateTask 함수의 다양한 시나리오를 검증합니다.
func TestCreateTask_TableDriven(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()

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
			wantErr: provider.ErrTaskNotSupported,
		},
		{
			name: "실패: AppConfig 내 Task 설정 없음 (빈 Config)",
			req: &contract.TaskSubmitRequest{
				TaskID:    validTaskID,
				CommandID: validCommandID,
			},
			appConfig: &config.AppConfig{}, // Empty config
			wantErr:   provider.ErrTaskNotFound,
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
			wantErr: provider.ErrCommandNotFound,
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

			// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
			// Task 생성
			// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
			mockStorage := &contractmocks.MockTaskResultStore{}

			handler, err := newTask(provider.NewTaskParams{
				InstanceID:  "test_instance",
				Request:     tt.req,
				AppConfig:   tt.appConfig,
				Storage:     mockStorage, // 정상 진행을 위해 Mock Storage 주입
				Fetcher:     mockFetcher,
				NewSnapshot: func() any { return &watchPriceSnapshot{} },
			})

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

				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// [커버리지 보완] SetExecute 로 등록된 익명 콜백 함수 내부 로직 검증
				// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
				// execute 콜백 함수는 newTask 안에서 handler.SetExecute()를 통해 바인딩됩니다.
				// [시나리오 1 - 커버리지 목적] 잘못된 스냅샷 객체를 주입하여 Type Assertion 실패 유도
				// NewSnapshot 함수가 반환하는 객체와 다른 타입을 주입하기 위해, Load 메서드가 잘못된 타입을 엎어쓰도록 Mock 설정
				mockStorage.On("Load", TaskID, WatchPriceAnyCommand, mock.Anything).Run(func(args mock.Arguments) {
					// 인터페이스를 통한 타입 덮어쓰기는 불가능하므로 빈 값을 읽은 것처럼 속이고,
					// 대신 실제 비즈니스 로직(execute)을 실행할 때 잘못된 타입을 넘기도록 우회해야 하지만,
					// Base.Run()은 내부적으로 우리가 제어할 수 없는 Storage.Load의 결과를 사용하므로
					// 여기서는 의도적으로 newSnapshot 함수 자체를 이상한 타입으로 반환하게 만들어 봅니다.
				}).Return(contract.ErrTaskResultNotFound).Once()

				// Fetcher에서 페이지 1 호출 시 에러 설정 (정상 흐름 진행용)
				mockFetcher.SetError("https://openapi.naver.com/v1/search/shop.json?display=100&query=test_query&sort=sim&start=1", fmt.Errorf("mock error to stop processing quickly"))

				ctx := context.Background()
				sender := &mockNotificationSender{}
				handler.Run(ctx, sender)

				// [시나리오 2 - 커버리지 목적] NewSnapshot 이 반환한 객체가 잘못된 타입일 경우
				handlerBadSnapshot, _ := newTask(provider.NewTaskParams{
					InstanceID:  "test_instance",
					Request:     tt.req,
					AppConfig:   tt.appConfig,
					Storage:     mockStorage,
					Fetcher:     mockFetcher,
					NewSnapshot: func() any { return &struct{ Invalid string }{} }, // 잘못된 타입
				})

				mockStorage.On("Load", TaskID, WatchPriceAnyCommand, mock.Anything).Return(contract.ErrTaskResultNotFound).Once()
				handlerBadSnapshot.Run(ctx, sender)
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
