package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TODO 미완료

// handleCommand 사용자 텔레그램 명령어 처리
func (n *telegramNotifier) handleCommand(ctx context.Context, executor contract.TaskExecutor, message *tgbotapi.Message) {
	// 모든 명령어 처리에 대해 10초의 타임아웃을 설정합니다.
	// 이를 통해 외부 API 호출(텔레그램 전송) 지연 등으로 인한 고루틴 무한 대기(Leak)를 방지합니다.
	// 부모 컨텍스트(notificationStopCtx)를 상속받아 서비스 종료 시 즉시 취소되도록 합니다.
	ctx, cancel := context.WithTimeout(ctx, constants.TelegramCommandTimeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"panic":       r,
			}).Error(constants.LogMsgTelegramCommandPanicRecovered)
		}
	}()

	// 텔레그램 명령어는 '/'로 시작해야 합니다. 그렇지 않은 경우 안내 메시지 전송.
	if len(message.Text) == 0 || message.Text[:1] != telegramBotCommandInitialCharacter {
		n.sendUnknownCommandMessage(ctx, message.Text)
		return
	}

	command := message.Text[1:] // '/' 제거

	// '/help' 명령어 처리
	if command == telegramBotCommandHelp {
		n.sendHelpCommandMessage(ctx)
		return
	}

	// '/cancel_{ID}' 명령어 처리 (작업 취소)
	if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) {
		n.handleCancelCommand(ctx, executor, command)
		return
	}

	// 등록된 작업 실행 명령어인지 확인 후 처리
	if botCommand, found := n.findBotCommand(command); found {
		n.executeCommand(executor, botCommand)
		return
	}

	// 매칭되는 명령어가 없는 경우
	n.sendUnknownCommandMessage(ctx, message.Text)
}

// findBotCommand 주어진 명령어 문자열과 일치하는 봇 명령어를 찾아 반환합니다.
func (n *telegramNotifier) findBotCommand(command string) (telegramBotCommand, bool) {
	botCommand, exists := n.botCommandsByCommand[command]
	return botCommand, exists
}

// executeCommand 주어진 봇 명령어를 Executor를 통해 실행합니다.
func (n *telegramNotifier) executeCommand(executor contract.TaskExecutor, botCommand telegramBotCommand) {
	// Executor를 통해 작업을 비동기로 실행 요청
	// 실행 요청이 큐에 가득 차는 등의 이유로 실패하면 error 반환
	if err := executor.Submit(&contract.TaskSubmitRequest{
		TaskID:        botCommand.taskID,
		CommandID:     botCommand.commandID,
		TaskContext:   contract.NewTaskContext(),
		NotifierID:    n.ID(),
		NotifyOnStart: true,
		RunBy:         contract.TaskRunByUser,
	}); err != nil {
		// 실행 실패 알림 발송
		// Receiver Loop Hang 방지: 대기열이 가득 차면 실패 알림은 과감히 생략(Drop) 하거나, 별도 고루틴으로 처리
		// 여기서는 Notify 메서드(Non-blocking/Timeout)를 사용하여 안전하게 처리합니다.
		ctx := contract.NewTaskContext().WithTask(botCommand.taskID, botCommand.commandID).WithError()
		if !n.Notify(ctx, msgTaskExecutionFailed) {
			// Notify 실패 시(큐 가득 참 등) 로그 남김
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"command":     botCommand.command,
			}).Warn(constants.LogMsgTelegramCmdFailNotifyFail)
		}
	}
}

// sendUnknownCommandMessage 알 수 없는 명령어 메시지를 전송합니다.
func (n *telegramNotifier) sendUnknownCommandMessage(ctx context.Context, input string) {
	// 텔레그램은 HTML 모드로 동작하므로, 사용자 입력값에 포함된 특수문자(<, > 등)를 이스케이프해야 합니다.
	escapedInput := html.EscapeString(input)
	message := fmt.Sprintf(msgUnknownCommand, escapedInput, telegramBotCommandInitialCharacter, telegramBotCommandHelp)
	n.sendMessage(ctx, message)
}

// sendHelpCommandMessage 사용 가능한 명령어 목록을 도움말 메시지로 전송합니다.
func (n *telegramNotifier) sendHelpCommandMessage(ctx context.Context) {
	message := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, botCommand := range n.botCommands {
		if i != 0 {
			message += "\n\n" // 명령어 간 줄바꿈
		}
		message += fmt.Sprintf("%s%s\n%s", telegramBotCommandInitialCharacter, botCommand.command, botCommand.commandDescription)
	}
	n.sendMessage(ctx, message)
}

// handleCancelCommand 작업 취소 요청 처리
func (n *telegramNotifier) handleCancelCommand(ctx context.Context, executor contract.TaskExecutor, command string) {
	// 취소명령 형식 : /cancel_nnnn (구분자로 분리)
	// strings.SplitN을 사용하여 명령어와 인자 두 부분으로만 나눕니다.
	// 이를 통해 InstanceID에 구분자(_)가 포함되어 있어도 정상적으로 파싱할 수 있습니다.
	commandSplit := strings.SplitN(command, telegramBotCommandSeparator, 2)

	// 올바른 형식인지 확인 (2부분으로 나뉘어야 함)
	if len(commandSplit) == 2 {
		instanceID := commandSplit[1]
		// Executor에 취소 요청
		if err := executor.Cancel(contract.TaskInstanceID(instanceID)); err != nil {
			// 취소 실패 시 알림 (Receiver Hang 방지: Notify 사용)
			n.Notify(contract.NewTaskContext().WithError(), fmt.Sprintf(msgTaskCancelFailed, instanceID))
		}
	} else {
		escapedCommand := html.EscapeString(command)
		message := fmt.Sprintf(msgInvalidCancelCommandFormat, escapedCommand, telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator)
		n.sendMessage(ctx, message)
	}
}
