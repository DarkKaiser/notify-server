package notification

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTelegramBot is a mock implementation of TelegramBot interface
type MockTelegramBot struct {
	mock.Mock
	updatesChan chan tgbotapi.Update
}

func (m *MockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	m.Called(config)
	return m.updatesChan
}

func (m *MockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	args := m.Called(c)
	return args.Get(0).(tgbotapi.Message), args.Error(1)
}

func (m *MockTelegramBot) StopReceivingUpdates() {
	m.Called()
}

func (m *MockTelegramBot) GetSelf() tgbotapi.User {
	args := m.Called()
	return args.Get(0).(tgbotapi.User)
}

// MockTaskRunner is a mock implementation of TaskRunner interface
type MockTaskRunner struct {
	mock.Mock
}

func (m *MockTaskRunner) TaskRun(taskID task.TaskID, taskCommandID task.TaskCommandID, notifierID string, manualRun bool, runType task.TaskRunBy) bool {
	args := m.Called(taskID, taskCommandID, notifierID, manualRun, runType)
	return args.Bool(0)
}

func (m *MockTaskRunner) TaskRunWithContext(taskID task.TaskID, taskCommandID task.TaskCommandID, taskCtx task.TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy task.TaskRunBy) (succeeded bool) {
	args := m.Called(taskID, taskCommandID, taskCtx, notifierID, notifyResultOfTaskRunRequest, taskRunBy)
	return args.Bool(0)
}

func (m *MockTaskRunner) TaskCancel(taskInstanceID task.TaskInstanceID) bool {
	args := m.Called(taskInstanceID)
	return args.Bool(0)
}

func TestTelegramNotifier_Run_HelpCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil) // Return value ignored as we use the channel directly
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != "" // Check if help text is sent
	})).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Help Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/help",
		},
	}

	// Wait for processing (simple sleep for test stability)
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_LongMessage(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Create a long message (>3900 chars) with newlines
	longMessage := ""
	for i := 0; i < 400; i++ {
		longMessage += "0123456789\n" // 11 chars * 400 = 4400 chars
	}

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect two messages to be sent
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Times(2)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification
	notifier.Notify(longMessage, task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_HTMLMessage(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	htmlMessage := "<b>Bold</b> and <i>Italic</i> text"

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ParseMode == "HTML"
	})).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send HTML Notification
	notifier.Notify(htmlMessage, task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_SendError(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, errors.New("network error"))
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification (should handle error gracefully)
	notifier.Notify("Test message", task.NewContext())

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions - error is logged but doesn't affect return value
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_SupportHTMLMessage(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Test
	result := notifier.SupportHTMLMessage()

	// Verify
	assert.True(t, result, "Telegram notifier should support HTML messages")
}

func TestTelegramNotifier_Notify_WithTaskContext(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with TaskContext
	taskCtx := task.NewContext().
		WithTask(task.TaskID("TEST"), task.TaskCommandID("TEST_CMD")).
		With(task.TaskCtxKeyTitle, "Test Task")

	notifier.Notify("Test message", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_ErrorContext(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with Error Context
	taskCtx := task.NewContext().
		WithTask(task.TaskID("TEST"), task.TaskCommandID("TEST_CMD")).
		WithError()

	notifier.Notify("Error occurred", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Run_CancelCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskCancel to be called
	mockTaskRunner.On("TaskCancel", task.TaskInstanceID("1234")).Return(true)

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Cancel Command: /cancel_1234
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/cancel_1234",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
	mockTaskRunner.AssertExpectations(t)
}

func TestTelegramNotifier_Run_UnknownCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect a reply about unknown command
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		return ok && msg.ChatID == chatID && msg.Text != "" // Check if text is sent
	})).Return(tgbotapi.Message{}, nil)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Unknown Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/unknown_command",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramNotifier_Run_TaskCommand(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update, 1),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)

	// Construct config with a task command
	appConfig := &g.AppConfig{
		Tasks: []g.TaskConfig{
			{
				ID:    "test_task",
				Title: "Test Task",
				Commands: []g.TaskCommandConfig{
					{
						ID:          "run",
						Title:       "Run Task",
						Description: "Runs the test task",
						Notifier: struct {
							Usable bool `json:"usable"`
						}{Usable: true},
						DefaultNotifierID: "test-notifier",
					},
				},
			},
		},
	}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)
	mockBot.On("StopReceivingUpdates").Return()

	// Expect TaskRun to be called
	// Command format: /task_id_command_id -> /test_task_run
	mockTaskRunner.On("TaskRun", task.TaskID("test_task"), task.TaskCommandID("run"), "test-notifier", true, task.TaskRunByUser).Return(true)

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Trigger Task Command
	mockBot.updatesChan <- tgbotapi.Update{
		Message: &tgbotapi.Message{
			Chat: &tgbotapi.Chat{ID: chatID},
			Text: "/test_task_run",
		},
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
	mockTaskRunner.AssertExpectations(t)
}

func TestTelegramNotifier_Notify_ElapsedTime(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{
		updatesChan: make(chan tgbotapi.Update),
	}
	mockTaskRunner := &MockTaskRunner{}
	chatID := int64(12345)
	appConfig := &g.AppConfig{}

	notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

	// Expectations
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "test_bot"})
	mockBot.On("GetUpdatesChan", mock.Anything).Return(nil)

	// Expect message with elapsed time
	mockBot.On("Send", mock.MatchedBy(func(c tgbotapi.Chattable) bool {
		msg, ok := c.(tgbotapi.MessageConfig)
		// Check for elapsed time string "1시간 1분 1초" (3661 seconds)
		return ok && msg.ParseMode == "HTML" &&
			(strings.Contains(msg.Text, "1시간") || strings.Contains(msg.Text, "1분") || strings.Contains(msg.Text, "1초"))
	})).Return(tgbotapi.Message{}, nil)

	mockBot.On("StopReceivingUpdates").Return()

	// Run
	ctx, cancel := context.WithCancel(context.Background())
	wg := &sync.WaitGroup{}
	wg.Add(1)

	go notifier.Run(mockTaskRunner, ctx, wg)

	// Send Notification with Elapsed Time
	taskCtx := task.NewContext().
		With(task.TaskCtxKeyTaskInstanceID, task.TaskInstanceID("1234")).
		With(task.TaskCtxKeyElapsedTimeAfterRun, int64(3661)) // 1h 1m 1s

	notifier.Notify("Task Completed", taskCtx)

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Cleanup
	cancel()
	wg.Wait()

	// Assertions
	mockBot.AssertExpectations(t)
}

func TestTelegramBotWrapper_GetSelf(t *testing.T) {
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

		wrapper := &telegramBotWrapper{BotAPI: mockBotAPI}

		// Test
		user := wrapper.GetSelf()

		// Verify
		assert.Equal(t, int64(123456), user.ID, "User ID가 일치해야 합니다")
		assert.Equal(t, "test_bot", user.UserName, "UserName이 일치해야 합니다")
		assert.Equal(t, "Test", user.FirstName, "FirstName이 일치해야 합니다")
		assert.Equal(t, "Bot", user.LastName, "LastName이 일치해야 합니다")
	})
}

func TestNewTelegramNotifierWithBot(t *testing.T) {
	t.Run("기본 설정으로 Notifier 생성", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update),
		}
		chatID := int64(12345)
		appConfig := &g.AppConfig{}

		notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig)

		// Verify
		assert.NotNil(t, notifier, "Notifier가 생성되어야 합니다")
		assert.Equal(t, NotifierID("test-notifier"), notifier.ID(), "ID가 일치해야 합니다")
		assert.True(t, notifier.SupportHTMLMessage(), "HTML 메시지를 지원해야 합니다")
	})

	t.Run("Task Commands가 있는 설정으로 Notifier 생성", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update),
		}
		chatID := int64(12345)

		appConfig := &g.AppConfig{
			Tasks: []g.TaskConfig{
				{
					ID:    "TestTask",
					Title: "테스트 작업",
					Commands: []g.TaskCommandConfig{
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
		}

		notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig).(*telegramNotifier)

		// Verify
		assert.NotNil(t, notifier, "Notifier가 생성되어야 합니다")
		// Usable이 true인 명령어 1개 + help 명령어 = 2개
		assert.Equal(t, 2, len(notifier.botCommands), "2개의 Bot Command가 등록되어야 합니다")

		// 첫 번째 명령어 확인 (test_task_run)
		assert.Equal(t, "test_task_run", notifier.botCommands[0].command, "명령어가 일치해야 합니다")
		assert.Equal(t, "테스트 작업 > 실행", notifier.botCommands[0].commandTitle, "명령어 제목이 일치해야 합니다")
		assert.Equal(t, task.TaskID("TestTask"), notifier.botCommands[0].taskID, "TaskID가 일치해야 합니다")
		assert.Equal(t, task.TaskCommandID("Run"), notifier.botCommands[0].taskCommandID, "TaskCommandID가 일치해야 합니다")

		// help 명령어 확인
		assert.Equal(t, "help", notifier.botCommands[1].command, "help 명령어가 등록되어야 합니다")
	})

	t.Run("빈 설정으로 Notifier 생성", func(t *testing.T) {
		mockBot := &MockTelegramBot{
			updatesChan: make(chan tgbotapi.Update),
		}
		chatID := int64(12345)
		appConfig := &g.AppConfig{
			Tasks: []g.TaskConfig{},
		}

		notifier := newTelegramNotifierWithBot("test-notifier", mockBot, chatID, appConfig).(*telegramNotifier)

		// Verify - help 명령어만 등록되어야 함
		assert.Equal(t, 1, len(notifier.botCommands), "help 명령어만 등록되어야 합니다")
		assert.Equal(t, "help", notifier.botCommands[0].command, "help 명령어가 등록되어야 합니다")
	})
}
