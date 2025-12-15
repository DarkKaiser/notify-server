package notification

import (
	"fmt"
	"strings"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

const (
	msgUnknownCommand             = "'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '%s%s'을 입력하세요."
	msgInvalidCancelCommandFormat = "'%s'는 잘못된 취소 명령어 형식입니다.\n올바른 형식: '%s%s%s[작업인스턴스ID]'"
	msgTaskExecutionFailed        = "사용자가 요청한 작업의 실행 요청이 실패하였습니다."
	msgTaskCancelFailed           = "작업취소 요청이 실패하였습니다.(ID:%s)"
	msgContextTitle               = "<b>【 %s 】</b>\n\n%s"
	msgContextError               = "%s\n\n*** 오류가 발생하였습니다. ***"
	msgElapsedTime                = " (%s지남)"
)

// handleCommand 사용자 텔레그램 명령어 처리
func (n *telegramNotifier) handleCommand(executor task.Executor, message *tgbotapi.Message) {
	// 텔레그램 명령어는 '/'로 시작해야 합니다. 그렇지 않은 경우 안내 메시지 전송.
	if message.Text[:1] != telegramBotCommandInitialCharacter {
		n.sendUnknownCommandMessage(message.Text)
		return
	}

	command := message.Text[1:] // '/' 제거

	// '/help' 명령어 처리
	if command == telegramBotCommandHelp {
		n.sendHelpCommandMessage()
		return
	}

	// '/cancel_{ID}' 명령어 처리 (작업 취소)
	if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) {
		n.handleCancelCommand(executor, command)
		return
	}

	// 등록된 작업 실행 명령어인지 확인 후 처리
	if botCommand, found := n.findBotCommand(command); found {
		n.executeCommand(executor, botCommand)
		return
	}

	// 매칭되는 명령어가 없는 경우
	n.sendUnknownCommandMessage(message.Text)
}

// findBotCommand 주어진 명령어 문자열과 일치하는 봇 명령어를 찾아 반환합니다.
func (n *telegramNotifier) findBotCommand(command string) (telegramBotCommand, bool) {
	for _, botCommand := range n.botCommands {
		if command == botCommand.command {
			return botCommand, true
		}
	}
	return telegramBotCommand{}, false
}

// executeCommand 주어진 봇 명령어를 Executor를 통해 실행합니다.
func (n *telegramNotifier) executeCommand(executor task.Executor, botCommand telegramBotCommand) {
	// Executor를 통해 작업을 비동기로 실행 요청
	// 실행 요청이 큐에 가득 차는 등의 이유로 실패하면 error 반환
	if err := executor.SubmitTask(&task.SubmitRequest{
		TaskID:        botCommand.taskID,
		CommandID:     botCommand.commandID,
		NotifierID:    string(n.ID()),
		NotifyOnStart: true,
		RunBy:         task.RunByUser,
	}); err != nil {
		// 실행 실패 알림 발송
		n.requestC <- &notifyRequest{
			taskCtx: task.NewTaskContext().WithTask(botCommand.taskID, botCommand.commandID).WithError(),
			message: msgTaskExecutionFailed,
		}
	}
}

// sendUnknownCommandMessage 알 수 없는 명령어 메시지를 전송합니다.
func (n *telegramNotifier) sendUnknownCommandMessage(input string) {
	message := fmt.Sprintf(msgUnknownCommand, input, telegramBotCommandInitialCharacter, telegramBotCommandHelp)
	n.sendMessage(message)
}

// sendHelpCommandMessage 사용 가능한 명령어 목록을 도움말 메시지로 전송합니다.
func (n *telegramNotifier) sendHelpCommandMessage() {
	message := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, botCommand := range n.botCommands {
		if i != 0 {
			message += "\n\n" // 명령어 간 줄바꿈
		}
		message += fmt.Sprintf("%s%s\n%s", telegramBotCommandInitialCharacter, botCommand.command, botCommand.commandDescription)
	}
	n.sendMessage(message)
}

// handleCancelCommand 작업 취소 요청 처리
func (n *telegramNotifier) handleCancelCommand(executor task.Executor, command string) {
	// 취소명령 형식 : /cancel_nnnn (구분자로 분리)
	commandSplit := strings.Split(command, telegramBotCommandSeparator)

	// 올바른 형식인지 확인 (2부분으로 나뉘어야 함)
	if len(commandSplit) == 2 {
		instanceID := commandSplit[1]
		// Executor에 취소 요청
		if err := executor.CancelTask(task.InstanceID(instanceID)); err != nil {
			// 취소 실패 시 알림
			n.requestC <- &notifyRequest{
				taskCtx: task.NewTaskContext().WithError(),
				message: fmt.Sprintf(msgTaskCancelFailed, instanceID),
			}
		}
	} else {
		message := fmt.Sprintf(msgInvalidCancelCommandFormat, command, telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator)
		n.sendMessage(message)
	}
}

// handleNotifyRequest 시스템 알림 전송 요청을 처리하고, 작업 컨텍스트 정보를 메시지에 추가하여 텔레그램으로 발송합니다.
func (n *telegramNotifier) handleNotifyRequest(req *notifyRequest) {
	message := req.message

	// 작업 실행과 관련된 컨텍스트 정보(작업명, 경과시간 등)가 있다면 메시지에 덧붙입니다.
	if req.taskCtx != nil {
		message = n.enrichMessageWithContext(req.taskCtx, message)
	}

	// 최종 메시지 전송
	n.sendMessage(message)
}

// enrichMessageWithContext TaskContext 정보를 메시지에 추가 (제목, 시간, 에러 등)
func (n *telegramNotifier) enrichMessageWithContext(taskCtx task.TaskContext, message string) string {
	// 1. 작업 제목 추가
	message = n.appendTitle(taskCtx, message)

	// 2. 작업 인스턴스 ID가 있으면 취소 명령어 안내 및 경과 시간 추가
	message = n.appendCancelCommandAndElapsedTime(taskCtx, message)

	// 3. 오류 발생 시 강조 표시 추가
	if taskCtx.IsErrorOccurred() {
		message = fmt.Sprintf(msgContextError, message)
	}

	return message
}

// appendTitle TaskContext에서 제목 정보를 추출하여 메시지에 추가합니다.
func (n *telegramNotifier) appendTitle(taskCtx task.TaskContext, message string) string {
	if title := taskCtx.GetTitle(); len(title) > 0 {
		return fmt.Sprintf(msgContextTitle, title, message)
	}

	// 제목이 없으면 ID를 기반으로 lookup하여 제목을 찾음
	taskID := taskCtx.GetID()
	commandID := taskCtx.GetCommandID()

	if !taskID.IsEmpty() && !commandID.IsEmpty() {
		for _, botCommand := range n.botCommands {
			if botCommand.taskID == taskID && botCommand.commandID == commandID {
				return fmt.Sprintf(msgContextTitle, botCommand.commandTitle, message)
			}
		}
	}

	return message
}

// appendCancelCommandAndElapsedTime TaskContext에서 작업 인스턴스 ID를 기반으로 취소 명령어를 메시지에 추가하고, 실행 경과 시간을 추가합니다.
func (n *telegramNotifier) appendCancelCommandAndElapsedTime(taskCtx task.TaskContext, message string) string {
	instanceID := taskCtx.GetInstanceID()
	if instanceID.IsEmpty() {
		return message
	}

	message += fmt.Sprintf("\n%s%s%s%s", telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator, instanceID)

	// 작업 실행 경과 시간 추가 (실행 완료된 경우)
	if elapsedTimeAfterRun := taskCtx.GetElapsedTimeAfterRun(); elapsedTimeAfterRun > 0 {
		message += formatElapsedTime(elapsedTimeAfterRun)
	}

	return message
}

// formatElapsedTime 초 단위 시간을 읽기 쉬운 문자열로 변환 (예: 1시간 30분 10초)
func formatElapsedTime(seconds int64) string {
	s := seconds % 60
	m := (seconds / 60) % 60
	h := seconds / 3600

	var sb strings.Builder
	if h > 0 {
		fmt.Fprintf(&sb, "%d시간 ", h)
	}
	if m > 0 {
		fmt.Fprintf(&sb, "%d분 ", m)
	}
	if s > 0 {
		fmt.Fprintf(&sb, "%d초 ", s)
	}

	if sb.Len() > 0 {
		return fmt.Sprintf(msgElapsedTime, sb.String())
	}
	return ""
}

// sendMessage 텔레그램 메시지 전송
// API 제한(4096자)을 초과하는 메시지는 자동으로 분할하여 전송합니다.
func (n *telegramNotifier) sendMessage(message string) {
	// 메시지 길이가 제한 이내라면 한 번에 전송
	if len(message) <= telegramMessageMaxLength {
		n.sendSingleMessage(message)
		return
	}

	// 제한을 초과하는 경우, 줄바꿈(\n) 단위로 메시지를 나눕니다.
	var messageChunk string
	lines := strings.SplitSeq(message, "\n")
	for line := range lines {
		neededSpace := len(line)
		if len(messageChunk) > 0 {
			neededSpace += 1 // 줄바꿈 문자 공간
		}

		// 현재 청크 + (줄바꿈) + 새 라인이 최대 길이를 넘으면
		if len(messageChunk)+neededSpace > telegramMessageMaxLength {
			// 현재까지 모은 청크가 있다면 전송
			if len(messageChunk) > 0 {
				n.sendSingleMessage(messageChunk)
				messageChunk = ""
			}

			// 현재 라인 자체가 최대 길이보다 길다면 강제로 자름 (Chunking)
			if len(line) > telegramMessageMaxLength {
				for len(line) > telegramMessageMaxLength {
					chunk := line[:telegramMessageMaxLength]
					n.sendSingleMessage(chunk)
					line = line[telegramMessageMaxLength:]
				}
				// 자르고 남은 뒷부분을 새로운 청크의 시작으로 설정
				messageChunk = line
			} else {
				// 현재 라인은 최대 길이 이내이므로 새로운 청크로 설정
				messageChunk = line
			}
		} else {
			// 청크에 라인 추가
			if len(messageChunk) > 0 {
				messageChunk += "\n"
			}
			messageChunk += line
		}
	}

	// 마지막 남은 청크 전송
	if len(messageChunk) > 0 {
		n.sendSingleMessage(messageChunk)
	}
}

// sendSingleMessage 단일 메시지 전송
func (n *telegramNotifier) sendSingleMessage(message string) {
	messageConfig := tgbotapi.NewMessage(n.chatID, message)
	messageConfig.ParseMode = tgbotapi.ModeHTML

	if _, err := n.botAPI.Send(messageConfig); err != nil {
		applog.WithComponentAndFields("notification.telegram", log.Fields{
			"notifier_id": n.ID(),
			"error":       err,
		}).Error("알림메시지 발송 실패")
	} else {
		applog.WithComponentAndFields("notification.telegram", log.Fields{
			"notifier_id": n.ID(),
		}).Info("알림메시지 발송 성공")
	}
}
