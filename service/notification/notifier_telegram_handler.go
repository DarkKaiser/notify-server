package notification

import (
	"fmt"
	"strings"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/service/task"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	log "github.com/sirupsen/logrus"
)

// handleCommand 사용자 텔레그램 명령어 처리
func (n *telegramNotifier) handleCommand(taskRunner task.TaskRunner, message *tgbotapi.Message) {
	// 텔레그램 명령어는 '/'로 시작해야 합니다. 그렇지 않은 경우 안내 메시지 전송.
	if message.Text[:1] != telegramBotCommandInitialCharacter {
		m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '%s%s'을 입력하세요.", message.Text, telegramBotCommandInitialCharacter, telegramBotCommandHelp)
		n.sendMessage(m)
		return
	}

	command := message.Text[1:] // '/' 제거

	// '/help' 명령어 처리
	if command == telegramBotCommandHelp {
		n.sendHelpMessage()
		return
	}

	// '/cancel_{ID}' 명령어 처리 (작업 취소)
	if strings.HasPrefix(command, fmt.Sprintf("%s%s", telegramBotCommandCancel, telegramBotCommandSeparator)) {
		n.handleCancelCommand(taskRunner, command)
		return
	}

	// 등록된 작업 실행 명령어인지 확인 후 처리
	for _, botCommand := range n.botCommands {
		if command == botCommand.command {
			// TaskRunner를 통해 작업을 비동기로 실행 요청
			// 실행 요청이 큐에 가득 차는 등의 이유로 실패하면 false 반환
			if !taskRunner.TaskRun(botCommand.taskID, botCommand.taskCommandID, string(n.ID()), true, task.TaskRunByUser) {
				// 실행 실패 알림 발송
				n.requestC <- &notifyRequest{
					message: "사용자가 요청한 작업의 실행 요청이 실패하였습니다.",
					taskCtx: task.NewContext().WithTask(botCommand.taskID, botCommand.taskCommandID).WithError(),
				}
			}
			return
		}
	}

	// 매칭되는 명령어가 없는 경우
	m := fmt.Sprintf("'%s'는 등록되지 않은 명령어입니다.\n명령어를 모르시면 '%s%s'을 입력하세요.", message.Text, telegramBotCommandInitialCharacter, telegramBotCommandHelp)
	n.sendMessage(m)
}

// sendHelpMessage 사용 가능한 명령어 목록을 도움말 메시지로 전송
func (n *telegramNotifier) sendHelpMessage() {
	m := "입력 가능한 명령어는 아래와 같습니다:\n\n"
	for i, botCommand := range n.botCommands {
		if i != 0 {
			m += "\n\n" // 명령어 간 줄바꿈
		}
		m += fmt.Sprintf("%s%s\n%s", telegramBotCommandInitialCharacter, botCommand.command, botCommand.commandDescription)
	}
	n.sendMessage(m)
}

// handleCancelCommand 작업 취소 요청 처리
func (n *telegramNotifier) handleCancelCommand(taskRunner task.TaskRunner, command string) {
	// 취소명령 형식 : /cancel_nnnn (구분자로 분리)
	commandSplit := strings.Split(command, telegramBotCommandSeparator)

	// 올바른 형식인지 확인 (2부분으로 나뉘어야 함)
	if len(commandSplit) == 2 {
		taskInstanceID := commandSplit[1]
		// TaskRunner에 취소 요청
		if !taskRunner.TaskCancel(task.TaskInstanceID(taskInstanceID)) {
			// 취소 실패 시 알림
			n.requestC <- &notifyRequest{
				message: fmt.Sprintf("작업취소 요청이 실패하였습니다.(ID:%s)", taskInstanceID),
				taskCtx: task.NewContext().WithError(),
			}
		}
	} else {
		m := fmt.Sprintf("'%s'는 잘못된 취소 명령어 형식입니다.\n올바른 형식: '%s%s%s[작업인스턴스ID]'", command, telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator)
		n.sendMessage(m)
	}
}

// handleNotifyRequest 시스템 알림 전송 요청을 처리하고, 작업 컨텍스트 정보를 메시지에 추가하여 텔레그램으로 발송합니다.
func (n *telegramNotifier) handleNotifyRequest(req *notifyRequest) {
	m := req.message

	// 작업 실행과 관련된 컨텍스트 정보(작업명, 경과시간 등)가 있다면 메시지에 덧붙입니다.
	if req.taskCtx != nil {
		m = n.enrichMessageWithContext(m, req.taskCtx)
	}

	// 최종 메시지 전송
	n.sendMessage(m)
}

// enrichMessageWithContext TaskContext 정보를 메시지에 추가 (제목, 시간, 에러 등)
func (n *telegramNotifier) enrichMessageWithContext(message string, taskCtx task.TaskContext) string {
	// 1. 작업 제목 추가
	title, ok := taskCtx.Value(task.TaskCtxKeyTitle).(string)
	if ok && len(title) > 0 {
		message = fmt.Sprintf("<b>【 %s 】</b>\n\n%s", title, message)
	} else {
		// 제목이 없으면 ID를 기반으로 lookup하여 제목을 찾음
		taskID, ok1 := taskCtx.Value(task.TaskCtxKeyTaskID).(task.TaskID)
		taskCommandID, ok2 := taskCtx.Value(task.TaskCtxKeyTaskCommandID).(task.TaskCommandID)
		if ok1 && ok2 {
			for _, botCommand := range n.botCommands {
				if botCommand.taskID == taskID && botCommand.taskCommandID == taskCommandID {
					message = fmt.Sprintf("<b>【 %s 】</b>\n\n%s", botCommand.commandTitle, message)
					break
				}
			}
		}
	}

	// 2. 작업 인스턴스 ID가 있으면 취소 명령어 안내 추가
	if taskInstanceID, ok := taskCtx.Value(task.TaskCtxKeyTaskInstanceID).(task.TaskInstanceID); ok {
		message += fmt.Sprintf("\n%s%s%s%s", telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator, taskInstanceID)

		// 3. 작업 실행 경과 시간 추가 (실행 완료된 경우)
		if elapsedTimeAfterRun, ok := taskCtx.Value(task.TaskCtxKeyElapsedTimeAfterRun).(int64); ok && elapsedTimeAfterRun > 0 {
			message += formatElapsedTime(elapsedTimeAfterRun)
		}
	}

	// 4. 오류 발생 시 강조 표시 추가
	if errorOccurred, ok := taskCtx.Value(task.TaskCtxKeyErrorOccurred).(bool); ok && errorOccurred {
		message = fmt.Sprintf("%s\n\n*** 오류가 발생하였습니다. ***", message)
	}

	return message
}

// formatElapsedTime 초 단위 시간을 읽기 쉬운 문자열로 변환 (예: 1시간 30분 10초)
func formatElapsedTime(seconds int64) string {
	s := seconds % 60
	m := (seconds / 60) % 60
	h := seconds / 3600

	var result string
	if h > 0 {
		result = fmt.Sprintf("%d시간 ", h)
	}
	if m > 0 {
		result += fmt.Sprintf("%d분 ", m)
	}
	if s > 0 {
		result += fmt.Sprintf("%d초 ", s)
	}

	if len(result) > 0 {
		return fmt.Sprintf(" (%s지남)", result)
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
		// 현재 청크 + 새 라인이 최대 길이를 넘으면 현재 청크를 먼저 전송
		if len(messageChunk)+len(line)+1 > telegramMessageMaxLength {
			n.sendSingleMessage(messageChunk)
			messageChunk = line
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
