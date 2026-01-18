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

// dispatchCommand 사용자 텔레그램 명령어 처리 (Router)
func (n *telegramNotifier) dispatchCommand(ctx context.Context, message *tgbotapi.Message) {
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
	if len(message.Text) == 0 || message.Text[:1] != botCommandInitialCharacter {
		n.replyUnknownCommand(ctx, message.Text)
		return
	}

	commandName := message.Text[1:] // '/' 제거

	// '/help' 명령어 처리
	if commandName == botCommandHelp {
		n.replyHelpCommand(ctx)
		return
	}

	// '/cancel_{ID}' 명령어 처리 (작업 취소)
	if strings.HasPrefix(commandName, fmt.Sprintf("%s%s", botCommandCancel, botCommandSeparator)) {
		n.processCancel(ctx, commandName)
		return
	}

	// 등록된 작업 실행 명령어인지 확인 후 처리
	if botCommand, found := n.lookupCommand(commandName); found {
		n.submitTask(botCommand)
		return
	}

	// 매칭되는 명령어가 없는 경우
	n.replyUnknownCommand(ctx, message.Text)
}

// lookupCommand 주어진 명령어 문자열과 일치하는 봇 명령어를 찾아 반환합니다.
func (n *telegramNotifier) lookupCommand(commandName string) (botCommand, bool) {
	botCommand, exists := n.botCommandsByName[commandName]
	return botCommand, exists
}

// submitTask 주어진 봇 명령어를 Executor를 통해 실행(제출)합니다.
func (n *telegramNotifier) submitTask(command botCommand) {
	// Executor를 통해 작업을 비동기로 실행 요청
	// 실행 요청이 큐에 가득 차는 등의 이유로 실패하면 error 반환
	if err := n.executor.Submit(&contract.TaskSubmitRequest{
		TaskID:        command.taskID,
		CommandID:     command.commandID,
		TaskContext:   contract.NewTaskContext(),
		NotifierID:    n.ID(),
		NotifyOnStart: true,
		RunBy:         contract.TaskRunByUser,
	}); err != nil {
		// 실행 실패 알림 발송
		// Receiver Loop Hang 방지: 대기열이 가득 차면 실패 알림은 과감히 생략(Drop) 하거나, 별도 고루틴으로 처리
		// 여기서는 Notify 메서드(Non-blocking/Timeout)를 사용하여 안전하게 처리합니다.
		ctx := contract.NewTaskContext().WithTask(command.taskID, command.commandID).WithError()
		if !n.Notify(ctx, msgTaskExecutionFailed) {
			// Notify 실패 시(큐 가득 참 등) 로그 남김
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"command":     command.name,
			}).Warn(constants.LogMsgTelegramCmdFailNotifyFail)
		}
	}
}

// processCancel 작업 취소 요청 처리
func (n *telegramNotifier) processCancel(ctx context.Context, commandName string) {
	// 취소명령 형식 : /cancel_nnnn (구분자로 분리)
	// strings.SplitN을 사용하여 명령어와 인자 두 부분으로만 나눕니다.
	// 이를 통해 InstanceID에 구분자(_)가 포함되어 있어도 정상적으로 파싱할 수 있습니다.
	commandSplit := strings.SplitN(commandName, botCommandSeparator, 2)

	// 올바른 형식인지 확인 (2부분으로 나뉘어야 함)
	if len(commandSplit) == 2 {
		instanceID := commandSplit[1]
		// Executor에 취소 요청
		if err := n.executor.Cancel(contract.TaskInstanceID(instanceID)); err != nil {
			// 취소 실패 시 알림 (Receiver Hang 방지: Notify 사용)
			n.Notify(contract.NewTaskContext().WithError(), fmt.Sprintf(msgTaskCancelFailed, instanceID))
		}
	} else {
		escapedCommand := html.EscapeString(commandName)
		message := fmt.Sprintf(msgInvalidCancelCommandFormat, escapedCommand, botCommandInitialCharacter, botCommandCancel, botCommandSeparator)
		n.sendMessage(ctx, message)
	}
}

// replyHelpCommand 사용 가능한 명령어 목록을 도움말 메시지로 응답합니다.
func (n *telegramNotifier) replyHelpCommand(ctx context.Context) {
	message := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, command := range n.botCommands {
		if i != 0 {
			message += "\n\n" // 명령어 간 줄바꿈
		}
		message += fmt.Sprintf("%s%s\n%s", botCommandInitialCharacter, command.name, command.description)
	}
	n.sendMessage(ctx, message)
}

// replyUnknownCommand 알 수 없는 명령어 메시지에 응답합니다.
func (n *telegramNotifier) replyUnknownCommand(ctx context.Context, input string) {
	// 텔레그램은 HTML 모드로 동작하므로, 사용자 입력값에 포함된 특수문자(<, > 등)를 이스케이프해야 합니다.
	escapedInput := html.EscapeString(input)
	message := fmt.Sprintf(msgUnknownCommand, escapedInput, botCommandInitialCharacter, botCommandHelp)
	n.sendMessage(ctx, message)
}
