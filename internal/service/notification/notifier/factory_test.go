package notifier_test

import (
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	notificationmocks "github.com/darkkaiser/notify-server/internal/service/notification/mocks"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Interface Compliance Checks
// =============================================================================

// NotifierFactory Implementation
var _ notifier.NotifierFactory = (*notifier.DefaultNotifierFactory)(nil)
var _ notifier.NotifierFactory = (*notificationmocks.MockNotifierFactory)(nil) // Test Mock

// =============================================================================
// Compile-Time Verification Test
// =============================================================================

func TestNotifierFactoryInterface(t *testing.T) {
	tests := []struct {
		name string
		impl interface{}
	}{
		{"DefaultNotifierFactory", &notifier.DefaultNotifierFactory{}},
		{"mockNotifierFactory", &notificationmocks.MockNotifierFactory{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := tt.impl.(notifier.NotifierFactory)
			require.True(t, ok, "%s should implement NotifierFactory interface", tt.name)
		})
	}
}

// =============================================================================
// Notifier Factory Tests
// =============================================================================

// TestDefaultNotifierFactory_CreateNotifiers_Table은 DefaultNotifierFactory의 CreateNotifiers 메서드를 검증합니다.
//
// 검증 항목:
//   - Telegram Notifier 생성 성공
//   - 빈 설정 처리
//   - 여러 프로세서 처리
//   - 프로세서 에러 처리
func TestDefaultNotifierFactory_CreateNotifiers_Table(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *config.AppConfig
		registerProcs  []notifier.NotifierConfigProcessor
		expectHandlers int
		expectError    bool
	}{
		{
			name: "Success Telegram",
			cfg: &config.AppConfig{
				Notifier: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{
						{ID: "t1", BotToken: "tok", ChatID: 1},
						{ID: "t2", BotToken: "tok", ChatID: 2},
					},
				},
			},
			registerProcs: []notifier.NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					var handlers []notifier.NotifierHandler
					for _, t := range cfg.Notifier.Telegrams {
						handlers = append(handlers, &notificationmocks.MockNotifierHandler{IDValue: notifier.NotifierID(t.ID)})
					}
					return handlers, nil
				},
			},
			expectHandlers: 2,
			expectError:    false,
		},
		{
			name: "Empty Config",
			cfg: &config.AppConfig{
				Notifier: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{},
				},
			},
			registerProcs: []notifier.NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{}, nil
				},
			},
			expectHandlers: 0,
			expectError:    false,
		},
		{
			name: "Multiple Processors",
			cfg:  &config.AppConfig{},
			registerProcs: []notifier.NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{&notificationmocks.MockNotifierHandler{IDValue: "h1"}}, nil
				},
				func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					return []notifier.NotifierHandler{&notificationmocks.MockNotifierHandler{IDValue: "h2"}}, nil
				},
			},
			expectHandlers: 2, // 1 + 1
			expectError:    false,
		},
		{
			name: "Processor Error",
			cfg:  &config.AppConfig{},
			registerProcs: []notifier.NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]notifier.NotifierHandler, error) {
					return nil, errors.New("processor error")
				},
			},
			expectHandlers: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := notifier.NewNotifierFactory()
			require.NotNil(t, factory, "Factory should not be nil")

			for _, proc := range tt.registerProcs {
				factory.RegisterProcessor(proc)
			}

			mockExecutor := &taskmocks.MockExecutor{}
			handlers, err := factory.CreateNotifiers(tt.cfg, mockExecutor)

			if tt.expectError {
				require.Error(t, err, "Should return error")
			} else {
				require.NoError(t, err, "Should not return error")
				assert.Len(t, handlers, tt.expectHandlers)
			}
		})
	}
}
