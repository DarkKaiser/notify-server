package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	taskmocks "github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
)

func TestTelegramNotifier_Regressions(t *testing.T) {

	// Regression Test for: "HTML 태그 깨짐 방지"
	// Ensure that truncating a title does not break HTML entities (like &lt;)
	t.Run("Fix: appendTitle safely truncates and escapes", func(t *testing.T) {
		mockBot := new(MockBotAPI)
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		// Create a title that is longer than titleTruncateLength (200)
		// And ensure it has special characters at the truncation boundary.
		// "A" * 195 + "<Important>"
		// If we truncate the escaped string, "<" becomes "&lt;", length increases.
		longTitle := strings.Repeat("A", 195) + "<Important>"

		req := &notifier.NotifyRequest{
			Message: "Body",
			TaskCtx: task.NewTaskContext().WithTitle(longTitle),
		}

		// The logic is: SafeTitle = html.EscapeString(truncateString(Title))
		// Truncate(195*'A' + "<Important>") -> 195*'A' + "<Imp" (length 199 or similar depending on implementation)
		// Actually Truncate limit is 200.
		// 195 + len("<Important>") = 195 + 11 = 206.
		// So it will be truncated to 200 chars: 195*'A' + "<Impo"
		// Then Escape: 195*'A' + "&lt;Impo"

		// If the OLD buggy logic was used:
		// Escape first: 195*'A' + "&lt;Important&gt;"
		// Truncate(..., 200): 195*'A' + "&lt;I" (Wait, & l t ; is 4 chars)
		// It might cut in the middle of &lt; if we are unlucky.

		// Let's just assert that the sent message contains VALID html entities and no broken ones.
		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			// Check that we don't have broken entities like "&l " or "t;" at the end validation
			// Or more simply, check that it contains the escaped version of the truncated string.

			// We expect the correct behavior:
			// 1. Truncate "AAAA...<Important>" at 200 -> "AAAA...<Impo" (assuming "..." is added if truncated, let's check implementation)
			// Implementation: return string(runes[:limit]) + "..."
			// So "AAAA...<Impo..."
			// 2. Escape -> "AAAA...&lt;Impo..."

			// If we verified the output contains "&lt;" and does NOT contain raw "<", it's good.
			if strings.Contains(msg.Text, "<Impo") { // raw tag, bad
				return false
			}
			if !strings.Contains(msg.Text, "&lt;Impo") { // expected escaped
				return false
			}
			return true
		})).Return(tgbotapi.Message{}, nil).Once()

		n.handleNotifyRequest(context.Background(), req)

		mockBot.AssertExpectations(t)
	})

	// Regression Test for: "Cancel 명령어 파싱 오류 수정"
	// Ensure that /cancel_task_1_inst_1 is parsed correctly as InstanceID="task_1_inst_1"
	t.Run("Fix: handleCancelCommand supports underscores in InstanceID", func(t *testing.T) {
		mockExec := new(taskmocks.MockExecutor)
		n := &telegramNotifier{}

		ctx := context.Background()
		commandWithUnderscores := "/cancel_task_1_instance_123"
		expectedInstanceID := "task_1_instance_123"

		mockExec.On("CancelTask", task.InstanceID(expectedInstanceID)).Return(nil).Once()

		n.handleCancelCommand(ctx, mockExec, commandWithUnderscores)

		mockExec.AssertExpectations(t)
	})

	t.Run("Fix: handleCancelCommand fails gracefully for bad format", func(t *testing.T) {
		mockExec := new(taskmocks.MockExecutor)
		// We need a mockBot here because it sends a message on error
		mockBot := new(MockBotAPI)
		n := &telegramNotifier{
			botAPI: mockBot,
			chatID: 12345,
		}

		ctx := context.Background()
		// Only one part
		badCommand := "/cancel"

		mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
			return strings.Contains(msg.Text, "잘못된 취소 명령어 형식")
		})).Return(tgbotapi.Message{}, nil).Once()

		// Should NOT call CancelTask
		n.handleCancelCommand(ctx, mockExec, badCommand)

		mockExec.AssertExpectations(t)
		mockBot.AssertExpectations(t)
	})
}
