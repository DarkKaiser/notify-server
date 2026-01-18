package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Telegram Bot API Client Tests
// =============================================================================

func TestTelegramBotAPIClient_GetSelf(t *testing.T) {
	t.Parallel()

	t.Run("GetSelf는 봇 자신의 정보를 반환해야 한다", func(t *testing.T) {
		t.Parallel()

		// given
		expectedUser := tgbotapi.User{
			ID:        123456,
			UserName:  "test_bot",
			FirstName: "Test",
			LastName:  "Bot",
		}
		mockBotAPI := &tgbotapi.BotAPI{
			Self: expectedUser,
		}

		client := &defaultBotClient{BotAPI: mockBotAPI}

		// when
		user := client.GetSelf()

		// then
		assert.Equal(t, expectedUser, user)
		assert.Equal(t, int64(123456), user.ID)
	})
}

// =============================================================================
// Telegram Notifier Factory Tests
// =============================================================================

func TestNewNotifierWithBot_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                 string
		appConfig            *config.AppConfig
		expectedCommandCount int
		expectedFirstCmd     string
	}{
		{
			name:                 "기본 설정: 도움말 명령어만 포함되어야 한다",
			appConfig:            &config.AppConfig{},
			expectedCommandCount: 1, // Help
			expectedFirstCmd:     "help",
		},
		{
			name: "단일 작업 설정: 도움말과 작업 명령어가 포함되어야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "TestTask",
						Title: "Test Task",
						Commands: []config.CommandConfig{
							{
								ID:          "Run",
								Title:       "Run",
								Description: "Run task",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: true},
								DefaultNotifierID: "test-notifier",
							},
						},
					},
				},
			},
			expectedCommandCount: 2, // Task Command + Help
			expectedFirstCmd:     "test_task_run",
		},
		{
			name: "사용 불가능한(Unusable) 명령어: 무시되어야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "TestTask",
						Title: "Test Task",
						Commands: []config.CommandConfig{
							{
								ID:    "Stop",
								Title: "Stop",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: false}, // Disabled
							},
						},
					},
				},
			},
			expectedCommandCount: 1, // Only Help
			expectedFirstCmd:     "help",
		},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			mockBot := &MockTelegramBot{updatesChan: make(chan tgbotapi.Update)}
			mockExecutor := &taskmocks.MockExecutor{}
			p := params{
				BotToken:  "test-token",
				ChatID:    12345,
				AppConfig: tt.appConfig,
			}

			// when
			n, err := newNotifierWithBot("test-notifier", mockBot, mockExecutor, p)

			// then
			require.NoError(t, err)

			notifier, ok := n.(*telegramNotifier)
			require.True(t, ok, "반환된 인스턴스는 *telegramNotifier 타입이어야 한다")
			require.NotNil(t, notifier)

			assert.Len(t, notifier.botCommands, tt.expectedCommandCount)
			if tt.expectedCommandCount > 0 {
				assert.Equal(t, tt.expectedFirstCmd, notifier.botCommands[0].name)
			}

			// 상수 값 검증
			assert.Equal(t, constants.TelegramNotifierBufferSize, cap(notifier.RequestC()), "버퍼 크기가 상수와 일치해야 한다")
		})
	}
}

func TestNewNotifierWithBot_Failure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		appConfig     *config.AppConfig
		expectedError string // 에러 메시지 일부 검증
	}{
		{
			name: "TaskID 누락: NewErrInvalidCommandIDs 에러가 발생해야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: "", // Missing
						Commands: []config.CommandConfig{
							{ID: "Run", Notifier: config.CommandNotifierConfig{Usable: true}},
						},
					},
				},
			},
			expectedError: "TaskID와 CommandID는 필수입니다",
		},
		{
			name: "CommandID 누락: NewErrInvalidCommandIDs 에러가 발생해야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: "TestTask",
						Commands: []config.CommandConfig{
							{ID: "", Notifier: config.CommandNotifierConfig{Usable: true}}, // Missing
						},
					},
				},
			},
			expectedError: "TaskID와 CommandID는 필수입니다",
		},
		{
			name: "명령어 이름 충돌: NewErrDuplicateCommandName 에러가 발생해야 한다",
			// 충돌 시나리오:
			// 1. Task: "foo_bar", Command: "baz" -> /foo_bar_baz
			// 2. Task: "foo", Command: "bar_baz" -> /foo_bar_baz
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: "foo_bar", Commands: []config.CommandConfig{{ID: "baz", Notifier: config.CommandNotifierConfig{Usable: true}}},
					},
					{
						ID: "foo", Commands: []config.CommandConfig{{ID: "bar_baz", Notifier: config.CommandNotifierConfig{Usable: true}}},
					},
				},
			},
			expectedError: "명령어 충돌: /foo_bar_baz",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			mockBot := &MockTelegramBot{updatesChan: make(chan tgbotapi.Update)}
			mockExecutor := &taskmocks.MockExecutor{}
			p := params{
				BotToken:  "test-token",
				ChatID:    12345,
				AppConfig: tt.appConfig,
			}

			// when
			n, err := newNotifierWithBot("test-notifier", mockBot, mockExecutor, p)

			// then
			require.Error(t, err)
			assert.Nil(t, n)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestBuildCreator(t *testing.T) {
	t.Parallel()

	// given
	mockConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			Telegrams: []config.TelegramConfig{
				{ID: "t1"},
				{ID: "t2"},
			},
		},
	}
	mockExecutor := &taskmocks.MockExecutor{}

	callCount := 0
	// factory.go에 정의된 constructor 타입 시그니처와 정확히 일치해야 합니다.
	mockCons := func(id contract.NotifierID, executor contract.TaskExecutor, p params) (notifier.Notifier, error) {
		callCount++
		// 테스트용 더미 Notifier 반환 (*telegramNotifier는 Notifier 인터페이스를 구현함)
		return &telegramNotifier{}, nil
	}

	// when
	// buildCreator는 constructor 타입을 인자로 받습니다.
	creator := buildCreator(mockCons)

	notifiers, err := creator(mockConfig, mockExecutor)

	// then
	require.NoError(t, err)
	assert.Len(t, notifiers, 2, "2개의 텔레그램 설정이 있으므로 2개의 Notifier가 생성되어야 한다")
	assert.Equal(t, 2, callCount, "생성자가 2번 호출되어야 한다")
}
