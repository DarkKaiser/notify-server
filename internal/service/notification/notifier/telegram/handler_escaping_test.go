package telegram

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockBotAPI struct {
	mock.Mock
}

func (m *MockBotAPI) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	args := m.Called(config)
	return args.Get(0).(tgbotapi.UpdatesChannel)
}

func (m *MockBotAPI) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockBotAPI) StopReceivingUpdates() {
	m.Called()
}

func (m *MockBotAPI) GetSelf() tgbotapi.User {
	args := m.Called()
	return args.Get(0).(tgbotapi.User)
}

func TestTelegramNotifier_Escaping(t *testing.T) {
	t.Run("HandleNotifyRequest escapes message", func(t *testing.T) {
		mockBot := new(MockBotAPI)
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		req := &notifier.NotifyRequest{
			Message: "Price < 1000 & Name > Foo",
		}

		expectedMessage := "Price &lt; 1000 &amp; Name &gt; Foo"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return msg.Text == expectedMessage
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(req)

		mockBot.AssertExpectations(t)
	})

	t.Run("HandleNotifyRequest escapes title in context", func(t *testing.T) {
		mockBot := new(MockBotAPI)
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		req := &notifier.NotifyRequest{
			Message: "Body",
			TaskCtx: task.NewTaskContext().WithTitle("<Important>"),
		}

		// Expected: <b>【 &lt;Important&gt; 】</b>\n\nBody
		expectedPartial := "<b>【 &lt;Important&gt; 】</b>"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return assert.Contains(t, msg.Text, expectedPartial)
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(req)

		mockBot.AssertExpectations(t)
	})
}
