package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/types"
	"github.com/darkkaiser/notify-server/internal/service/task"
	"github.com/stretchr/testify/assert"
)

func TestNewNotifier_CommandCollision(t *testing.T) {
	// given
	// 충돌을 유발하는 설정 생성:
	// 1. Task: "foo_bar", Command: "baz" -> /foo_bar_baz
	// 2. Task: "foo", Command: "bar_baz" -> /foo_bar_baz
	appConfig := &config.AppConfig{
		Notifier: config.NotifierConfig{
			Telegrams: []config.TelegramConfig{
				{
					ID:       "telegram-1",
					BotToken: "test-token",
					ChatID:   12345,
				},
			},
		},
		Tasks: []config.TaskConfig{
			{
				ID:    "foo_bar",
				Title: "Task 1",
				Commands: []config.CommandConfig{
					{
						ID:       "baz",
						Title:    "Command 1",
						Notifier: config.CommandNotifierConfig{Usable: true},
					},
				},
			},
			{
				ID:    "foo",
				Title: "Task 2",
				Commands: []config.CommandConfig{
					{
						ID:       "bar_baz",
						Title:    "Command 2",
						Notifier: config.CommandNotifierConfig{Usable: true},
					},
				},
			},
		},
	}

	mockExecutor := &mockExecutor{}
	mockBot := &MockTelegramBot{}

	// when
	_, err := newTelegramNotifierWithBot(types.NotifierID("telegram-1"), mockBot, 12345, appConfig, mockExecutor)

	// then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "텔레그램 명령어 충돌이 감지되었습니다")
	assert.Contains(t, err.Error(), "/foo_bar_baz")
}

type mockExecutor struct{}

func (m *mockExecutor) SubmitTask(req *task.SubmitRequest) error    { return nil }
func (m *mockExecutor) CancelTask(instanceID task.InstanceID) error { return nil }
