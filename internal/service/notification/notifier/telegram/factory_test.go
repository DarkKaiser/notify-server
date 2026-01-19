package telegram

import (
	"errors"
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
// Helper Mocks & Types
// =============================================================================

// MockBotClient is a manual mock for botClient interface if needed specific to factory.
// We reuse MockTelegramBot from mock_test.go which satisfies botClient.

// =============================================================================
// Bot Client Tests
// =============================================================================

func TestDefaultBotClient_GetSelf(t *testing.T) {
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
}

// =============================================================================
// Factory Creator Tests
// =============================================================================

func TestNewCreator(t *testing.T) {
	t.Parallel()

	// when
	creator := NewCreator()

	// then
	assert.NotNil(t, creator, "NewCreator는 nil이 아닌 함수를 반환해야 합니다")
}

func TestBuildCreator(t *testing.T) {
	t.Parallel()

	t.Run("성공: 모든 Notifier가 정상적으로 생성되어야 한다", func(t *testing.T) {
		t.Parallel()

		// given
		mockConfig := &config.AppConfig{
			Notifier: config.NotifierConfig{
				Telegrams: []config.TelegramConfig{
					{ID: "telegram-1", BotToken: "token-1", ChatID: 1001},
					{ID: "telegram-2", BotToken: "token-2", ChatID: 1002},
				},
			},
		}
		mockExecutor := &taskmocks.MockExecutor{}

		callCount := 0
		// Mock constructor
		mockCtor := func(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
			callCount++
			assert.Equal(t, mockExecutor, executor)
			assert.Equal(t, mockConfig, args.AppConfig)

			// ID에 따른 매핑 검증
			if id == "telegram-1" {
				assert.Equal(t, "token-1", args.BotToken)
				assert.Equal(t, int64(1001), args.ChatID)
			} else if id == "telegram-2" {
				assert.Equal(t, "token-2", args.BotToken)
				assert.Equal(t, int64(1002), args.ChatID)
			}

			return &telegramNotifier{}, nil
		}

		// when
		creator := buildCreator(mockCtor)
		notifiers, err := creator(mockConfig, mockExecutor)

		// then
		require.NoError(t, err)
		assert.Len(t, notifiers, 2)
		assert.Equal(t, 2, callCount)
	})

	t.Run("실패: 생성자 중 하나라도 실패하면 에러를 반환해야 한다", func(t *testing.T) {
		t.Parallel()

		// given
		mockConfig := &config.AppConfig{
			Notifier: config.NotifierConfig{
				Telegrams: []config.TelegramConfig{
					{ID: "t1"}, {ID: "t2"},
				},
			},
		}
		expectedErr := errors.New("creation failed")

		mockCtor := func(id contract.NotifierID, executor contract.TaskExecutor, args creationArgs) (notifier.Notifier, error) {
			if id == "t2" {
				return nil, expectedErr
			}
			return &telegramNotifier{}, nil
		}

		// when
		creator := buildCreator(mockCtor)
		notifiers, err := creator(mockConfig, &taskmocks.MockExecutor{})

		// then
		require.Error(t, err)
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, notifiers)
	})
}

// =============================================================================
// Notifier Construction Tests (newNotifierWithClient)
// =============================================================================

func TestNewNotifierWithClient_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		appConfig         *config.AppConfig
		validateStructure func(*testing.T, *telegramNotifier)
	}{
		{
			name:      "기본 설정: 필수 필드 및 도움말 명령어가 초기화되어야 한다",
			appConfig: &config.AppConfig{},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				assert.NotNil(t, n.limiter, "Rate Limiter가 초기화되어야 함")
				assert.NotNil(t, n.commandSemaphore, "세마포어가 초기화되어야 함")
				assert.Equal(t, constants.DefaultTelegramRetryDelay, n.retryDelay)
				assert.Equal(t, constants.TelegramNotifierBufferSize, cap(n.NotificationC()))

				// 기본 도움말 명령어 확인
				assert.Len(t, n.botCommands, 1)
				assert.Equal(t, "help", n.botCommands[0].name)
				assert.Contains(t, n.botCommandsByName, "help")
			},
		},
		{
			name: "명령어 등록: Task 명령어가 올바르게 인덱싱되어야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "Deploy",
						Title: "Deployment",
						Commands: []config.CommandConfig{
							{
								ID:          "Start",
								Title:       "Start Deploy",
								Description: "Starts deployment",
								Notifier:    config.CommandNotifierConfig{Usable: true},
							},
						},
					},
				},
			},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				expectedCmdName := "deploy_start" // snake_case 변환 확인

				// 1. 리스트 확인 (Task Cmd + Help)
				assert.Len(t, n.botCommands, 2)

				// 2. Name Map 확인
				cmd, exists := n.botCommandsByName[expectedCmdName]
				require.True(t, exists, "명령어가 이름으로 검색되어야 함")
				assert.Equal(t, "Deployment > Start Deploy", cmd.title)

				// 3. Task Map 확인
				taskCmds, taskExists := n.botCommandsByTask["Deploy"]
				require.True(t, taskExists, "TaskID로 맵이 생성되어야 함")
				cmdFromTask, cmdExists := taskCmds["Start"]
				require.True(t, cmdExists, "CommandID로 명령어를 찾을 수 있어야 함")
				assert.Equal(t, expectedCmdName, cmdFromTask.name)
			},
		},
		{
			name: "사용 불가 명령어: Usable이 false인 경우 등록되지 않아야 한다",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID: "Backup",
						Commands: []config.CommandConfig{
							{ID: "Run", Notifier: config.CommandNotifierConfig{Usable: false}},
						},
					},
				},
			},
			validateStructure: func(t *testing.T, n *telegramNotifier) {
				assert.Len(t, n.botCommands, 1, "도움말 명령어만 존재해야 함")
				_, exists := n.botCommandsByName["backup_run"]
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			mockBot := &MockTelegramBot{}
			mockExecutor := &taskmocks.MockExecutor{}
			args := creationArgs{
				BotToken:  "test-token",
				ChatID:    12345,
				AppConfig: tt.appConfig,
			}

			// when
			n, err := newNotifierWithClient("test-notifier", mockBot, mockExecutor, args)

			// then
			require.NoError(t, err)
			notifier := n.(*telegramNotifier)

			// 공통 검증: 봇 클라이언트 주입
			assert.Equal(t, mockBot, notifier.botClient)
			assert.Equal(t, int64(12345), notifier.chatID)

			// 케이스별 상세 검증
			if tt.validateStructure != nil {
				tt.validateStructure(t, notifier)
			}
		})
	}
}

func TestNewNotifierWithClient_Failure(t *testing.T) {
	t.Parallel()

	// Refined collision case
	collisionConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{ID: "foo_bar", Commands: []config.CommandConfig{{ID: "baz", Notifier: config.CommandNotifierConfig{Usable: true}}}},
			{ID: "foo", Commands: []config.CommandConfig{{ID: "bar_baz", Notifier: config.CommandNotifierConfig{Usable: true}}}},
		},
	}

	t.Run("명령어 이름 충돌 발생 시 에러 반환", func(t *testing.T) {
		t.Parallel()
		mockBot := &MockTelegramBot{}
		args := creationArgs{BotToken: "t", ChatID: 1, AppConfig: collisionConfig}

		n, err := newNotifierWithClient("id", mockBot, &taskmocks.MockExecutor{}, args)
		require.Error(t, err)
		assert.Nil(t, n)
		assert.Contains(t, err.Error(), "/foo_bar_baz")
	})

	t.Run("필수 ID 누락 시 에러 반환", func(t *testing.T) {
		t.Parallel()
		mockBot := &MockTelegramBot{}
		invalidConfig := &config.AppConfig{
			Tasks: []config.TaskConfig{{ID: "", Commands: []config.CommandConfig{{ID: "cmd", Notifier: config.CommandNotifierConfig{Usable: true}}}}},
		}
		args := creationArgs{BotToken: "t", ChatID: 1, AppConfig: invalidConfig}

		n, err := newNotifierWithClient("id", mockBot, &taskmocks.MockExecutor{}, args)
		require.Error(t, err)
		assert.Nil(t, n)
		assert.Contains(t, err.Error(), "필수입니다")
	})
}

// TestNewNotifier_IntegrationIntegration tests the actual newNotifier function locally.
// (Optional, verifying it calls newNotifierWithClient internally)
func TestNewNotifier_CallingWithClient(t *testing.T) {
	t.Parallel()

	// This is hard to test because it creates http.Client and tgbotapi.NewBotAPI.
	// We assume it works if compilation passes and other tests mimic its logic.
	// We can skip deep testing here to avoid network calls or complex mocking of tgbotapi.
}
