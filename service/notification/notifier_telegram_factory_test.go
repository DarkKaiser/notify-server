package notification

import (
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
)

func TestTelegramBotAPIClient_GetSelf(t *testing.T) {
	t.Run("GetSelf 함수 테스트", func(t *testing.T) {
		// Setup
		mockBotAPI := &tgbotapi.BotAPI{
			Self: tgbotapi.User{
				ID:        123456,
				UserName:  "test_bot",
				FirstName: "Test",
				LastName:  "Bot",
			},
		}

		client := &telegramBotAPIClient{BotAPI: mockBotAPI}

		// Test
		user := client.GetSelf()

		// Verify
		assert.Equal(t, int64(123456), user.ID, "User ID가 일치해야 합니다")
		assert.Equal(t, "test_bot", user.UserName, "UserName이 일치해야 합니다")
		assert.Equal(t, "Test", user.FirstName, "FirstName이 일치해야 합니다")
		assert.Equal(t, "Bot", user.LastName, "LastName이 일치해야 합니다")
	})
}

func TestNewTelegramNotifierWithBot(t *testing.T) {
	tests := []struct {
		name          string
		appConfig     *config.AppConfig
		checkNotifier func(*testing.T, *telegramNotifier)
	}{
		{
			name:      "기본 설정으로 Notifier 생성",
			appConfig: &config.AppConfig{},
			checkNotifier: func(t *testing.T, n *telegramNotifier) {
				assert.NotNil(t, n, "Notifier가 생성되어야 합니다")
				assert.Equal(t, NotifierID("test-notifier"), n.ID(), "ID가 일치해야 합니다")
				assert.True(t, n.SupportsHTMLMessage(), "HTML 메시지를 지원해야 합니다")
			},
		},
		{
			name: "Task Commands가 있는 설정으로 Notifier 생성",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{
					{
						ID:    "TestTask",
						Title: "테스트 작업",
						Commands: []config.TaskCommandConfig{
							{
								ID:          "Run",
								Title:       "실행",
								Description: "작업을 실행합니다",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: true},
								DefaultNotifierID: "test-notifier",
							},
							{
								ID:          "Stop",
								Title:       "중지",
								Description: "작업을 중지합니다",
								Notifier: struct {
									Usable bool `json:"usable"`
								}{Usable: false}, // Usable이 false인 경우
								DefaultNotifierID: "test-notifier",
							},
						},
					},
				},
			},
			checkNotifier: func(t *testing.T, n *telegramNotifier) {
				assert.NotNil(t, n, "Notifier가 생성되어야 합니다")
				// Usable이 true인 명령어 1개 + help 명령어 = 2개
				assert.Equal(t, 2, len(n.botCommands), "2개의 Bot Command가 등록되어야 합니다")

				// 첫 번째 명령어 확인 (test_task_run)
				assert.Equal(t, "test_task_run", n.botCommands[0].command, "명령어가 일치해야 합니다")
				assert.Equal(t, "테스트 작업 > 실행", n.botCommands[0].commandTitle, "명령어 제목이 일치해야 합니다")
				assert.Equal(t, task.TaskID("TestTask"), n.botCommands[0].taskID, "TaskID가 일치해야 합니다")
				assert.Equal(t, task.TaskCommandID("Run"), n.botCommands[0].taskCommandID, "TaskCommandID가 일치해야 합니다")

				// help 명령어 확인
				assert.Equal(t, "help", n.botCommands[1].command, "help 명령어가 등록되어야 합니다")
			},
		},
		{
			name: "빈 설정으로 Notifier 생성",
			appConfig: &config.AppConfig{
				Tasks: []config.TaskConfig{},
			},
			checkNotifier: func(t *testing.T, n *telegramNotifier) {
				// Verify - help 명령어만 등록되어야 함
				assert.Equal(t, 1, len(n.botCommands), "help 명령어만 등록되어야 합니다")
				assert.Equal(t, "help", n.botCommands[0].command, "help 명령어가 등록되어야 합니다")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockBot := &MockTelegramBot{
				updatesChan: make(chan tgbotapi.Update),
			}
			chatID := int64(12345)

			notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, tt.appConfig).(*telegramNotifier)
			tt.checkNotifier(t, notifier)
		})
	}
}
