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

const (
	// botCommandHelp 사용자가 이 봇의 사용법을 알고 싶을 때 입력하는 기본적인 도움말 명령어입니다.
	// 사용자가 "/help"를 입력하면, 등록된 모든 명령어의 목록과 설명을 보여줍니다.
	botCommandHelp = "help"

	// botCommandCancel 현재 실행 중이거나 대기 중인 작업을 취소할 때 사용하는 명령어입니다.
	// 이 명령어는 동적으로 생성되며, 뒤에 인스턴스 ID가 붙습니다. (예: "/cancel_mytask_123")
	botCommandCancel = "cancel"

	// botCommandPrefix 텔레그램에서 이것이 일반 메시지가 아닌 '명령어'임을 나타내는 접두어입니다.
	// 모든 명령어는 이 슬래시('/') 문자로 시작해야 봇이 인식합니다.
	botCommandPrefix = "/"

	// botCommandSeparator 명령어 이름과 추가적인 인자(Parameter)를 구분하는 데 사용되는 문자입니다.
	// 예: "/cancel_123"에서 "cancel"과 "123"을 구분짓는 역할을 합니다.
	botCommandSeparator = "_"
)

// botCommand 텔레그램 봇에서 실행할 수 있는 하나의 '명령어'를 정의하는 메타데이터 구조체입니다.
type botCommand struct {
	// name 사용자가 텔레그램 채팅창에 입력할 명령어 이름입니다. (접두어 제외, 예: "start")
	name string

	// title 명령어 목록 등에서 보여질 명령어의 짧은 제목입니다.
	title string

	// description 명령어에 대한 상세한 설명입니다. 도움말(/help) 요청 시 사용자에게 표시됩니다.
	description string

	// taskID 이 명령어와 연결된 백엔드 작업(Task)의 고유 식별자입니다. (예: "scrapping-news")
	taskID contract.TaskID

	// commandID 해당 작업(Task) 내에서 구체적으로 실행할 커맨드의 ID입니다. (예: "run-daily")
	commandID contract.TaskCommandID
}

// dispatchCommand 텔레그램 봇으로 수신된 사용자의 명령어 메시지를 분석하고, 적절한 핸들러로 라우팅하는 메인 진입점입니다.
func (n *telegramNotifier) dispatchCommand(ctx context.Context, message *tgbotapi.Message) {
	// 1. 안전한 실행 시간 보장
	// 모든 명령어 처리에 타임아웃을 설정하여, 텔레그램 API 지연 등 외부 요인으로 인해 고루틴이 무한 대기하는 것을 방지합니다.
	// 부모 컨텍스트가 취소되면(서비스 종료 등) 이 작업도 즉시 중단됩니다.
	ctx, cancel := context.WithTimeout(ctx, constants.TelegramCommandTimeout)
	defer cancel()

	// 2. 패닉 복구
	// 명령어 처리 로직 중 예상치 못한 런타임 오류(Panic)가 발생하더라도,
	// 전체 봇 서비스(수신 루프)가 중단되지 않도록 여기서 에러를 포착하고 로그를 남깁니다.
	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"command":     message.Text, // 입력값: 패닉을 유발한 명령어
				"panic":       r,
			}).Error(constants.LogMsgTelegramCommandHandlingPanicRecovered)
		}
	}()

	// 3. 명령어 유효성 검사
	// 텔레그램의 모든 봇 명령어는 '/' 접두어로 시작해야 합니다.
	if len(message.Text) == 0 || !strings.HasPrefix(message.Text, botCommandPrefix) {
		n.replyUnknownCommand(ctx, message.Text)
		return
	}

	// 접두어를 제거하여 순수 명령어 이름만 추출합니다. (예: "/start" -> "start")
	commandInput := strings.TrimPrefix(message.Text, botCommandPrefix)

	// 4. 명령어 라우팅

	// Case A: 도움말 명령어 ('/help')
	if commandInput == botCommandHelp {
		n.replyHelpCommand(ctx)
		return
	}

	// Case B: 작업 취소 명령어 ('/cancel_{ID}')
	// 작업 인스턴스 ID를 포함하는 동적 명령어이므로 접두어 매칭으로 처리합니다.
	if strings.HasPrefix(commandInput, fmt.Sprintf("%s%s", botCommandCancel, botCommandSeparator)) {
		n.processCancel(ctx, commandInput)
		return
	}

	// Case C: 등록된 작업 실행 명령어 (예: '/my_task')
	// 설정 파일에 등록된 사용자 정의 명령어인지 확인하고 작업을 제출합니다.
	if command, found := n.lookupCommand(commandInput); found {
		n.submitTask(command)
		return
	}

	// Case D: 처리할 수 없는 명령어
	// 위의 어떤 케이스에도 해당하지 않는 경우, 사용자에게 안내 메시지를 보냅니다.
	n.replyUnknownCommand(ctx, message.Text)
}

// lookupCommand 수신된 명령어 문자열(이름)과 일치하는 등록된 봇 명령어를 고속 조회(Map Lookup)합니다.
func (n *telegramNotifier) lookupCommand(commandName string) (botCommand, bool) {
	command, exists := n.botCommandsByName[commandName]
	return command, exists
}

// submitTask 사용자가 요청한 작업을 TaskExecutor에 제출하여 비동기 실행을 시작합니다.
// 작업 제출이 실패(예: 대기열 가득 참)하는 경우, 사용자에게 실패 알림을 전송합니다.
func (n *telegramNotifier) submitTask(command botCommand) {
	// TaskExecutor에게 작업 실행을 요청합니다.
	// 이 요청은 비동기로 처리되며, 실제 작업은 별도의 고루틴 또는 워커 풀에서 실행됩니다.
	if err := n.executor.Submit(&contract.TaskSubmitRequest{
		TaskID:        command.taskID,
		CommandID:     command.commandID,
		TaskContext:   contract.NewTaskContext(),
		NotifierID:    n.ID(),
		NotifyOnStart: true,
		RunBy:         contract.TaskRunByUser,
	}); err != nil {
		// 작업 제출에 실패한 경우(예: 작업 대기열이 가득 참, 시스템 과부하 등),
		// 사용자에게 작업이 실행되지 않았음을 알리는 실패 메시지를 전송합니다.
		taskCtx := contract.NewTaskContext().WithTask(command.taskID, command.commandID).WithError()
		if err := n.Send(taskCtx, msgTaskExecutionFailed); err != nil {
			// 실패 알림조차 전송하지 못한 경우(예: 알림 발송 큐가 가득 참), 더 이상 재시도하지 않고 로그만 남깁니다.
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"task_id":     command.taskID,
				"command_id":  command.commandID,
				"command":     command.name,
				"error":       err,
			}).Warn(constants.LogMsgTelegramTaskSubmitFailNotificationDropped)
		}
	}
}

// processCancel '/cancel_{instanceID}' 형식의 작업 취소 명령어를 파싱하고, 실행 중인 작업을 중단시킵니다.
// 명령어 형식이 올바르지 않거나 취소 요청이 실패할 경우 적절한 안내 메시지를 전송합니다.
func (n *telegramNotifier) processCancel(_ context.Context, commandInput string) {
	// 취소 명령어는 "/cancel_{InstanceID}" 형식을 따릅니다. (예: /cancel_task_123)
	// 여기서 strings.SplitN(..., 2)를 사용하는 이유는 InstanceID 자체에 구분자('_')가 포함될 수 있기 때문입니다.
	// 이를 통해 명령어 접두어와 실제 ID를 안전하게 분리하여, ID 내부의 특수문자로 인한 파싱 오류를 방지합니다.
	commandSplit := strings.SplitN(commandInput, botCommandSeparator, 2)

	// 명령어가 접두어와 ID 두 부분으로 올바르게 분리되었는지 확인합니다.
	if len(commandSplit) == 2 {
		instanceID := commandSplit[1]

		// TaskExecutor에게 해당 작업의 실행 취소를 요청합니다.
		if err := n.executor.Cancel(contract.TaskInstanceID(instanceID)); err != nil {
			// 취소 요청이 실패한 경우(예: 이미 종료된 작업, 존재하지 않는 ID 등), 사용자에게 실패 사유를 알립니다.
			if err := n.Send(contract.NewTaskContext().WithError(), fmt.Sprintf(msgTaskCancelFailed, instanceID)); err != nil {
				applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
					"notifier_id": n.ID(),
					"chat_id":     n.chatID,
					"instance_id": instanceID,
					"error":       err,
				}).Warn(constants.LogMsgTelegramCancelFailNotificationDropped)
			}
		}
	} else {
		// 잘못된 형식의 명령어가 입력된 경우, 올바른 취소 명령어 형식을 안내하는 메시지를 전송합니다.
		escapedInput := html.EscapeString(commandInput)
		message := fmt.Sprintf(msgInvalidCancelCommandFormat, escapedInput, botCommandPrefix, botCommandCancel, botCommandSeparator)

		if err := n.Send(contract.NewTaskContext(), message); err != nil {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"command":     commandInput,
				"error":       err,
			}).Warn(constants.LogMsgTelegramInvalidCancelReplyDropped)
		}
	}
}

// replyHelpCommand 현재 봇에 등록된 모든 사용 가능한 명령어 목록을 포맷팅하여 사용자에게 도움말 메시지로 전송합니다.
func (n *telegramNotifier) replyHelpCommand(_ context.Context) {
	message := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, command := range n.botCommands {
		if i != 0 {
			message += "\n\n" // 명령어 간 줄바꿈
		}
		message += fmt.Sprintf("%s%s\n%s", botCommandPrefix, command.name, command.description)
	}

	if err := n.Send(contract.NewTaskContext(), message); err != nil {
		applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"error":       err,
		}).Warn(constants.LogMsgTelegramHelpReplyDropped)
	}
}

// replyUnknownCommand 등록되지 않았거나 잘못된 명령어가 입력되었을 때, 올바른 사용법을 안내하는 메시지를 전송합니다.
func (n *telegramNotifier) replyUnknownCommand(_ context.Context, input string) {
	// 텔레그램 메시지는 HTML 모드로 전송되므로, 사용자 입력값에 포함된 특수문자(<, > 등)가 HTML 태그로 오인될 수 있습니다.
	// 이로 인해 메시지 형식이 깨지거나 전송이 실패하는 것을 방지하기 위해, 사용자 입력값은 반드시 이스케이프 처리하여 안전하게 변환해야 합니다.
	escapedInput := html.EscapeString(input)
	message := fmt.Sprintf(msgUnknownCommand, escapedInput, botCommandPrefix, botCommandHelp)

	if err := n.Send(contract.NewTaskContext(), message); err != nil {
		applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"command":     input,
			"error":       err,
		}).Warn(constants.LogMsgTelegramUnknownCmdReplyDropped)
	}
}
