package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// botCommandHelp 봇 사용법을 안내하는 기본 도움말 명령어입니다.
	//
	// 사용자가 "/help"를 입력하면 현재 사용 가능한 모든 명령어 목록과 설명을
	// 대화창에 메시지로 전송합니다. 봇을 처음 사용하는 사용자에게 유용합니다.
	botCommandHelp = "help"

	// botCommandCancel 실행 중이거나 대기 중인 특정 작업을 중단할 때 사용합니다.
	//
	// 이 명령어는 정적으로 사용되지 않고, 각 작업 인스턴스마다 고유한 ID를 붙여 동적으로 생성됩니다.
	// 예: "/cancel_task_123" (여기서 'task_123'은 특정 작업의 컨텍스트 ID)
	botCommandCancel = "cancel"

	// botCommandPrefix 텔레그램 메시지가 '명령어'임을 식별하는 접두어(Prefix)입니다.
	//
	// 텔레그램 봇 API 규약에 따라 모든 명령어는 이 슬래시('/') 문자로 시작해야 합니다.
	// 예: "/start", "/help"
	botCommandPrefix = "/"

	// botCommandSeparator 명령어의 기본 이름과 동적 매개변수(Parameter)를 구분하는 구분자입니다.
	//
	// 하나의 명령어가 특정 리소스를 지칭해야 할 때 사용됩니다.
	// 예: "/cancel_123"에서, "cancel"은 명령어(Action)이고 "123"은 대상(Target)입니다.
	botCommandSeparator = "_"
)

// botCommand 텔레그램 봇이 인식하고 처리할 수 있는 개별 명령어를 정의합니다.
//
// 하나의 botCommand는 특정 작업(Task)의 커맨드와 1:1로 매핑됩니다.
// 사용자가 이 명령어를 입력하면, 연결된 TaskExecutor를 통해 실제 로직이 실행됩니다.
type botCommand struct {
	// name 사용자가 채팅창에 입력하는 실제 명령어 텍스트입니다. (접두어 '/' 제외)
	// 예: "check_price" -> 사용자는 "/check_price"로 입력
	name string

	// title 명령어 목록 등에서 보여질 짧고 직관적인 제목입니다.
	// 예: "가격 조회", "서버 재시작"
	title string

	// description 명령어의 기능에 대한 상세한 설명입니다.
	// "/help" 명령어 호출 시 사용자에게 안내되는 텍스트로 사용됩니다.
	description string

	// taskID 이 명령어와 연결된 작업(Task)의 식별자입니다.
	// 예: "stock-price-checker", "server-monitor"
	taskID contract.TaskID

	// commandID 해당 작업(Task) 내에서 실행할 구체적인 명령어(Command)의 식별자입니다.
	// 하나의 Task가 여러 명령어(예: "start", "stop", "status")를 가질 수 있습니다.
	commandID contract.TaskCommandID
}

// dispatchCommand 사용자가 보낸 명령어 메시지를 분석하여 적절한 처리 로직으로 연결하는 라우팅(Routing) 메서드입니다.
//
// 이 메서드는 다음 순서로 동작합니다:
//  1. 메시지의 유효성을 검증합니다. (텍스트 존재 여부, 명령어 포맷 확인)
//  2. 명령어를 파싱하여 '명령어 이름'과 '매개변수'를 분리합니다.
//  3. 등록된 명령어 목록(botCommandsByName)에서 대응하는 핸들러를 찾습니다.
//  4. 해당 핸들러(TaskExecutor)를 실행하여 비즈니스 로직을 수행합니다.
func (n *telegramNotifier) dispatchCommand(serviceStopCtx context.Context, message *tgbotapi.Message) {
	// 1. 패닉 복구
	// 명령어 처리 로직 중 예상치 못한 런타임 오류(Panic)가 발생하더라도,
	// 전체 봇 서비스(수신 루프)가 중단되지 않도록 여기서 패닉을 포착하고 로그를 남깁니다.
	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"command":     message.Text, // 입력값: 패닉을 유발한 명령어
				"panic":       r,
			}).Error("텔레그램 핸들러 패닉 복구: 명령어 처리 중 예기치 않은 오류가 발생했습니다 (서비스 유지됨)")
		}
	}()

	// 2. 명령어 유효성 검사
	// 텔레그램의 모든 봇 명령어는 '/' 접두어로 시작해야 합니다.
	if len(message.Text) == 0 || !strings.HasPrefix(message.Text, botCommandPrefix) {
		n.replyUnknownCommand(serviceStopCtx, message.Text)
		return
	}

	// 접두어를 제거하여 순수 명령어 이름만 추출합니다. (예: "/start" -> "start")
	commandInput := strings.TrimPrefix(message.Text, botCommandPrefix)

	// 3. 명령어 라우팅

	// Case A: 도움말 명령어 ('/help')
	if commandInput == botCommandHelp {
		n.replyHelpCommand(serviceStopCtx)
		return
	}

	// Case B: 작업 취소 명령어 ('/cancel_{ID}')
	// 작업 인스턴스 ID를 포함하는 동적 명령어이므로 접두어 매칭으로 처리합니다.
	if strings.HasPrefix(commandInput, fmt.Sprintf("%s%s", botCommandCancel, botCommandSeparator)) {
		n.processCancel(serviceStopCtx, commandInput)
		return
	}

	// Case C: 등록된 작업 실행 명령어 (예: '/my_task')
	// 설정 파일에 등록된 사용자 정의 명령어인지 확인하고 작업을 제출합니다.
	if command, found := n.lookupCommand(commandInput); found {
		n.submitTask(serviceStopCtx, command)
		return
	}

	// Case D: 처리할 수 없는 명령어
	// 위의 어떤 케이스에도 해당하지 않는 경우, 사용자에게 안내 메시지를 보냅니다.
	n.replyUnknownCommand(serviceStopCtx, message.Text)
}

// lookupCommand 사용자가 입력한 명령어 이름으로 등록된 봇 명령어를 조회합니다.
func (n *telegramNotifier) lookupCommand(commandName string) (botCommand, bool) {
	command, exists := n.botCommandsByName[commandName]
	return command, exists
}

// submitTask 사용자가 요청한 작업을 TaskExecutor에 제출하여 백그라운드에서 비동기 실행을 시작합니다.
//
// 작업 제출 과정:
//  1. TaskExecutor.Submit()을 호출하여 작업을 대기열에 등록합니다.
//  2. 제출이 성공하면 사용자에게 "작업이 시작되었습니다" 메시지를 전송합니다.
//  3. 제출이 실패하면(예: 대기열 포화) 사용자에게 실패 안내 메시지를 전송합니다.
func (n *telegramNotifier) submitTask(serviceStopCtx context.Context, command botCommand) {
	// TaskExecutor에게 작업 실행을 요청합니다.
	// 이 호출은 작업을 대기열에 등록하는 것이며, 실제 작업은 별도의 워커 풀에서 비동기로 실행됩니다.
	if err := n.executor.Submit(serviceStopCtx, &contract.TaskSubmitRequest{
		TaskID:        command.taskID,
		CommandID:     command.commandID,
		NotifierID:    n.ID(),
		NotifyOnStart: true,
		RunBy:         contract.TaskRunByUser,
	}); err != nil {
		// 제출 실패 처리: 작업을 대기열에 등록하지 못한 경우 사용자에게 알림니다.
		//
		// 실패 원인:
		//  - 작업 대기열이 가득 차서 더 이상 작업을 받을 수 없음 (Backpressure)
		//  - 시스템 과부하 또는 리소스 부족
		//  - TaskExecutor가 종료 중이거나 비활성 상태
		//
		// 사용자에게 작업이 실행되지 않았음을 명확히 알려 혼란을 방지합니다.
		notification := contract.Notification{
			TaskID:        command.taskID,
			CommandID:     command.commandID,
			Message:       "작업 실행 요청이 실패했습니다.\n\n잠시 후 다시 시도해 주세요.\n(원인: 시스템 과부하 또는 대기열 포화)",
			ErrorOccurred: true,
		}
		if err := n.TrySend(serviceStopCtx, notification); err != nil {
			// 실패 알림조차 전송하지 못한 경우(예: 알림 발송 큐가 가득 참), 더 이상 재시도하지 않고 로그만 남깁니다.
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"task_id":     command.taskID,
				"command_id":  command.commandID,
				"command":     command.name,
				"error":       err,
			}).Warn("텔레그램 알림 누락: 작업 실패 안내 메시지 전송이 차단되었습니다 (Queue Full 또는 Timeout)")
		}
	}
}

// processCancel 사용자가 보낸 작업 취소 명령어를 처리합니다.
//
// 명령어 형식: "/cancel_{instanceID}" (예: "/cancel_task_12345")
//
// 처리 흐름:
//  1. 명령어를 파싱하여 instanceID를 추출합니다. (구분자 '_' 기준으로 분리)
//  2. TaskExecutor.Cancel()을 호출하여 해당 작업 인스턴스의 취소를 요청합니다.
//  3. 취소 성공 시 사용자에게 "작업이 취소되었습니다" 메시지를 전송합니다.
//  4. 취소 실패 시 사용자에게 실패 사유를 안내합니다.
//  5. 명령어 형식이 잘못된 경우 올바른 사용법을 안내합니다.
//
// 참고: instanceID에 구분자('_')가 포함될 수 있으므로 strings.SplitN(..., 2)를 사용하여 안전하게 파싱합니다.
func (n *telegramNotifier) processCancel(serviceStopCtx context.Context, commandInput string) {
	// 취소 명령어 파싱: "/cancel_{InstanceID}" 형식에서 InstanceID를 안전하게 추출합니다.
	//
	// 예시: "/cancel_task_12345" → instanceID = "task_12345"
	//
	// strings.SplitN(commandInput, botCommandSeparator, 2)를 사용하는 이유:
	//  - InstanceID 자체에 구분자('_')가 포함될 수 있습니다. (예: "task_abc_123")
	//  - Split()을 사용하면 ["cancel", "task", "abc", "123"]으로 잘못 분리됩니다.
	//  - SplitN(..., 2)는 첫 번째 구분자만 기준으로 ["cancel", "task_abc_123"]으로 정확히 2개로 분리합니다.
	//
	// 이를 통해 ID 내부의 구분자로 인한 파싱 오류를 방지하고, 복잡한 ID 형식도 안전하게 처리할 수 있습니다.
	commandSplit := strings.SplitN(commandInput, botCommandSeparator, 2)

	// 명령어가 접두어와 ID 두 부분으로 올바르게 분리되었는지 확인합니다.
	// 그리고 추출된 InstanceID가 유효한지(빈 문자열이 아닌지) 검사합니다.
	if len(commandSplit) == 2 && len(strings.TrimSpace(commandSplit[1])) > 0 {
		instanceID := strings.TrimSpace(commandSplit[1])

		// TaskExecutor에게 해당 작업의 실행 취소를 요청합니다.
		if err := n.executor.Cancel(contract.TaskInstanceID(instanceID)); err != nil {
			// 취소 요청이 실패한 경우(예: 이미 종료된 작업, 존재하지 않는 ID 등), 사용자에게 실패 사유를 알립니다.
			if err := n.TrySend(serviceStopCtx, contract.NewErrorNotification(
				fmt.Sprintf(
					"작업 취소 요청이 실패했습니다.\n\n"+
						"작업 ID: %s\n\n"+
						"이미 완료되었거나 존재하지 않는 작업일 수 있습니다.",
					instanceID,
				),
			)); err != nil {
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id": n.ID(),
					"chat_id":     n.chatID,
					"instance_id": instanceID,
					"error":       err,
				}).Warn("텔레그램 알림 누락: 작업 취소 실패 안내 메시지 전송이 차단되었습니다 (Queue Full 또는 Timeout)")
			}
		}
	} else {
		// 잘못된 형식의 명령어가 입력된 경우, 올바른 취소 명령어 형식을 안내하는 메시지를 전송합니다.
		message := fmt.Sprintf(
			"입력하신 명령어 '%s'는 올바른 형식이 아닙니다.\n\n"+
				"올바른 형식:\n"+
				"%s%s%s작업인스턴스ID\n\n"+
				"예시:\n"+
				"%s%s%stask_12345",
			html.EscapeString(commandInput),
			botCommandPrefix, botCommandCancel, botCommandSeparator,
			botCommandPrefix, botCommandCancel, botCommandSeparator,
		)

		if err := n.Send(serviceStopCtx, contract.NewNotification(message)); err != nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"command":     commandInput,
				"error":       err,
			}).Warn("텔레그램 알림 누락: 명령어 형식 오류 안내 메시지 전송이 차단되었습니다 (Queue Full 또는 Timeout)")
		}
	}
}

// replyHelpCommand 현재 봇에 등록된 모든 사용 가능한 명령어 목록을 포맷팅하여 사용자에게 도움말 메시지로 전송합니다.
func (n *telegramNotifier) replyHelpCommand(serviceStopCtx context.Context) {
	message := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, command := range n.botCommands {
		if i != 0 {
			message += "\n\n" // 명령어 간 줄바꿈
		}
		message += fmt.Sprintf("%s%s\n%s", botCommandPrefix, command.name, command.description)
	}

	if err := n.Send(serviceStopCtx, contract.NewNotification(message)); err != nil {
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id":   n.ID(),
			"chat_id":       n.chatID,
			"command_count": len(n.botCommands),
			"error":         err,
		}).Warn("텔레그램 알림 누락: 도움말 안내 메시지 전송이 차단되었습니다 (Queue Full 또는 Timeout)")
	}
}

// replyUnknownCommand 등록되지 않았거나 잘못된 명령어가 입력되었을 때, 올바른 사용법을 안내하는 메시지를 전송합니다.
func (n *telegramNotifier) replyUnknownCommand(serviceStopCtx context.Context, input string) {
	// 텔레그램 메시지는 HTML 모드로 전송되므로, 사용자 입력값에 포함된 특수문자(<, > 등)가 HTML 태그로 오인될 수 있습니다.
	// 이로 인해 메시지 형식이 깨지거나 전송이 실패하는 것을 방지하기 위해, 사용자 입력값은 반드시 이스케이프 처리하여 안전하게 변환해야 합니다.
	message := fmt.Sprintf(
		"입력하신 명령어 '%s'는 등록되지 않은 명령어입니다.\n\n"+
			"사용 가능한 명령어 목록을 확인하시려면 '%s%s'를 입력해 주세요.",
		html.EscapeString(input),
		botCommandPrefix, botCommandHelp,
	)

	if err := n.Send(serviceStopCtx, contract.NewNotification(message)); err != nil {
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"command":     input,
			"error":       err,
		}).Warn("텔레그램 알림 누락: 미등록 명령어 안내 메시지 전송이 차단되었습니다 (Queue Full 또는 Timeout)")
	}
}
