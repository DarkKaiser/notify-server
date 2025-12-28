package notification

import (
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		registerProcs  []NotifierConfigProcessor
		expectHandlers int
		expectError    bool
	}{
		{
			name: "Success Telegram",
			cfg: &config.AppConfig{
				Notifiers: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{
						{ID: "t1", BotToken: "tok", ChatID: 1},
						{ID: "t2", BotToken: "tok", ChatID: 2},
					},
				},
			},
			registerProcs: []NotifierConfigProcessor{
				NewTelegramConfigProcessor(func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (NotifierHandler, error) {
					return &mockNotifierHandler{id: id}, nil
				}),
			},
			expectHandlers: 2,
			expectError:    false,
		},
		{
			name: "Empty Config",
			cfg: &config.AppConfig{
				Notifiers: config.NotifierConfig{
					Telegrams: []config.TelegramConfig{},
				},
			},
			registerProcs: []NotifierConfigProcessor{
				NewTelegramConfigProcessor(func(id NotifierID, botToken string, chatID int64, appConfig *config.AppConfig, executor task.Executor) (NotifierHandler, error) {
					return &mockNotifierHandler{id: id}, nil
				}),
			},
			expectHandlers: 0,
			expectError:    false,
		},
		{
			name: "Multiple Processors",
			cfg:  &config.AppConfig{},
			registerProcs: []NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return []NotifierHandler{&mockNotifierHandler{id: "h1"}}, nil
				},
				func(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return []NotifierHandler{&mockNotifierHandler{id: "h2"}}, nil
				},
			},
			expectHandlers: 2, // 1 + 1
			expectError:    false,
		},
		{
			name: "Processor Error",
			cfg:  &config.AppConfig{},
			registerProcs: []NotifierConfigProcessor{
				func(cfg *config.AppConfig, executor task.Executor) ([]NotifierHandler, error) {
					return nil, errors.New("processor error")
				},
			},
			expectHandlers: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewNotifierFactory()
			require.NotNil(t, factory, "Factory should not be nil")

			for _, proc := range tt.registerProcs {
				factory.RegisterProcessor(proc)
			}

			mockExecutor := &MockExecutor{}
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
