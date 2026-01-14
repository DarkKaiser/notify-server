package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// titleTruncateLength 제목이 너무 길 경우 텔레그램 메시지 분할 시 HTML 태그 깨짐 방지를 위해 자를 길이
	titleTruncateLength = 200

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
	// 모든 명령어 처리에 대해 10초의 타임아웃을 설정합니다.
	// 이를 통해 외부 API 호출(텔레그램 전송) 지연 등으로 인한 고루틴 무한 대기(Leak)를 방지합니다.
	ctx, cancel := context.WithTimeout(context.Background(), constants.TelegramCommandTimeout)
	defer cancel()

	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields("notification.telegram", applog.Fields{
				"notifier_id": n.ID(),
				"panic":       r,
			}).Error("봇 명령어 처리 중 패닉 발생 (Recovered)")
		}
	}()

	// 텔레그램 명령어는 '/'로 시작해야 합니다. 그렇지 않은 경우 안내 메시지 전송.
	if message.Text[:1] != telegramBotCommandInitialCharacter {
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
		// Receiver Loop Hang 방지: 대기열이 가득 차면 실패 알림은 과감히 생략(Drop) 하거나, 별도 고루틴으로 처리
		// 여기서는 Notify 메서드(Non-blocking/Timeout)를 사용하여 안전하게 처리합니다.
		taskCtx := task.NewTaskContext().WithTask(botCommand.taskID, botCommand.commandID).WithError()
		if !n.Notify(taskCtx, msgTaskExecutionFailed) {
			// Notify 실패 시(큐 가득 참 등) 로그 남김
			applog.WithComponentAndFields("notification.telegram", applog.Fields{
				"notifier_id": n.ID(),
				"command":     botCommand.command,
			}).Warn("실행 실패 메시지 알림 전송 실패")
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
func (n *telegramNotifier) handleCancelCommand(ctx context.Context, executor task.Executor, command string) {
	// 취소명령 형식 : /cancel_nnnn (구분자로 분리)
	// strings.SplitN을 사용하여 명령어와 인자 두 부분으로만 나눕니다.
	// 이를 통해 InstanceID에 구분자(_)가 포함되어 있어도 정상적으로 파싱할 수 있습니다.
	commandSplit := strings.SplitN(command, telegramBotCommandSeparator, 2)

	// 올바른 형식인지 확인 (2부분으로 나뉘어야 함)
	if len(commandSplit) == 2 {
		instanceID := commandSplit[1]
		// Executor에 취소 요청
		if err := executor.CancelTask(task.InstanceID(instanceID)); err != nil {
			// 취소 실패 시 알림 (Receiver Hang 방지: Notify 사용)
			n.Notify(task.NewTaskContext().WithError(), fmt.Sprintf(msgTaskCancelFailed, instanceID))
		}
	} else {
		escapedCommand := html.EscapeString(command)
		message := fmt.Sprintf(msgInvalidCancelCommandFormat, escapedCommand, telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator)
		n.sendMessage(ctx, message)
	}
}

// handleNotifyRequest 시스템 알림 전송 요청을 처리하고, 작업 컨텍스트 정보를 메시지에 추가하여 텔레그램으로 발송합니다.
func (n *telegramNotifier) handleNotifyRequest(ctx context.Context, req *notifier.NotifyRequest) {
	// 텔레그램은 HTML 모드로 동작하므로, 메시지 내용에 포함된 특수문자(<, > 등)를 이스케이프해야 합니다.
	message := html.EscapeString(req.Message)

	// 작업 실행과 관련된 컨텍스트 정보(작업명, 경과시간 등)가 있다면 메시지에 덧붙입니다.
	if req.TaskCtx != nil {
		message = n.enrichMessageWithContext(req.TaskCtx, message)
	}

	// 최종 메시지 전송
	n.sendMessage(ctx, message)
}

// enrichMessageWithContext TaskContext 정보를 메시지에 추가 (제목, 시간, 에러 등)
func (n *telegramNotifier) enrichMessageWithContext(taskCtx task.TaskContext, message string) string {
	// 1. 작업 제목 추가
	message = n.appendTitle(taskCtx, message)

	// 2. 작업 인스턴스 ID가 있으면 취소 명령어 안내 및 경과 시간 추가
	message = n.appendCancelCommand(taskCtx, message)
	message = n.appendElapsedTime(taskCtx, message)

	// 3. 오류 발생 시 강조 표시 추가
	if taskCtx.IsErrorOccurred() {
		message = fmt.Sprintf(msgContextError, message)
	}

	return message
}

// appendTitle TaskContext에서 제목 정보를 추출하여 메시지에 추가합니다.
func (n *telegramNotifier) appendTitle(taskCtx task.TaskContext, message string) string {
	if title := taskCtx.GetTitle(); len(title) > 0 {
		// 긴 제목으로 인해 HTML 태그가 닫히지 않은 채 메시지가 분할되는 등의 문제를 방지하기 위해 Truncate 처리
		// 중요: Truncate를 먼저 수행한 후 이스케이프해야 안전합니다.
		// 이스케이프된 문자열을 자르면 '&lt;' 따위가 잘려서 '&l' 처럼 되어 HTML 파싱 에러를 유발할 수 있습니다.
		safeTitle := html.EscapeString(truncateString(title, titleTruncateLength))
		return fmt.Sprintf(msgContextTitle, safeTitle, message)
	}

	// 제목이 없으면 ID를 기반으로 lookup하여 제목을 찾음
	taskID := taskCtx.GetID()
	commandID := taskCtx.GetCommandID()

	if !taskID.IsEmpty() && !commandID.IsEmpty() {
		// O(1) Map 조회로 성능 개선 (중첩 맵 사용)
		if commands, ok := n.botCommandsByTaskAndCommand[string(taskID)]; ok {
			if botCommand, exists := commands[string(commandID)]; exists {
				return fmt.Sprintf(msgContextTitle, html.EscapeString(botCommand.commandTitle), message)
			}
		}
	}

	return message
}

// appendCancelCommand 메시지 하단에 해당 작업을 즉시 취소할 수 있는 명령어 링크를 추가합니다.
//
// 이 기능은 TaskContext의 IsCancelable() 상태가 true일 때만 활성화됩니다.
// 주로 사용자가 직접 실행한 장기 실행 작업에 대해, 알림 메시지 자체를 통해 손쉽게 작업을
// 취소할 수 있는 UX를 제공하기 위함입니다.
//
// 생성되는 명령어 형식: /cancel_{InstanceID} (예: /cancel_inst_12345)
func (n *telegramNotifier) appendCancelCommand(taskCtx task.TaskContext, message string) string {
	if !taskCtx.IsCancelable() {
		return message
	}

	instanceID := taskCtx.GetInstanceID()
	if instanceID.IsEmpty() {
		return message
	}

	return fmt.Sprintf("%s\n\n%s%s%s%s", message, telegramBotCommandInitialCharacter, telegramBotCommandCancel, telegramBotCommandSeparator, instanceID)
}

// appendElapsedTime 실행 경과 시간을 메시지에 추가합니다.
func (n *telegramNotifier) appendElapsedTime(taskCtx task.TaskContext, message string) string {
	if elapsedTimeAfterRun := taskCtx.GetElapsedTimeAfterRun(); elapsedTimeAfterRun > 0 {
		return message + formatElapsedTime(elapsedTimeAfterRun)
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
// 컨텍스트가 취소되면 전송을 중단하고 즉시 반환합니다.
func (n *telegramNotifier) sendMessage(ctx context.Context, message string) {
	// 메시지 길이가 제한 이내라면 한 번에 전송
	if len(message) <= telegramMessageMaxLength {
		n.sendSingleMessage(ctx, message)
		return
	}

	// 제한을 초과하는 경우, 줄바꿈(\n) 단위로 메시지를 나눕니다.
	var messageChunk string
	lines := strings.SplitSeq(message, "\n")
	for line := range lines {
		// 컨텍스트 취소 확인 (긴 루프 중간에 탈출)
		if ctx.Err() != nil {
			return
		}

		neededSpace := len(line)
		if len(messageChunk) > 0 {
			neededSpace += 1 // 줄바꿈 문자 공간
		}

		// 현재 청크 + (줄바꿈) + 새 라인이 최대 길이를 넘으면
		if len(messageChunk)+neededSpace > telegramMessageMaxLength {
			// 현재까지 모은 청크가 있다면 전송
			if len(messageChunk) > 0 {
				n.sendSingleMessage(ctx, messageChunk)
				messageChunk = ""
			}

			// 현재 라인 자체가 최대 길이보다 길다면 강제로 자름 (Chunking)
			// 중요: 한글 등 멀티바이트 문자가 깨지지 않도록 Safe Split 수행
			if len(line) > telegramMessageMaxLength {
				currentLine := line
				for len(currentLine) > telegramMessageMaxLength {
					if ctx.Err() != nil {
						return
					}

					chunk, remainder := safeSplit(currentLine, telegramMessageMaxLength)
					n.sendSingleMessage(ctx, chunk)
					currentLine = remainder
				}
				// 자르고 남은 뒷부분을 새로운 청크의 시작으로 설정
				messageChunk = currentLine
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
		n.sendSingleMessage(ctx, messageChunk)
	}
}

// sendSingleMessage 단일 메시지 전송
// 컨텍스트 취소(종료 시그널)를 감지하면 즉시 중단합니다.
func (n *telegramNotifier) sendSingleMessage(ctx context.Context, message string) {
	messageConfig := tgbotapi.NewMessage(n.chatID, message)
	messageConfig.ParseMode = tgbotapi.ModeHTML

	// 텔레그램 API Rate Limit 준수를 위해 발송 속도를 제어합니다.
	// 지정된 속도(Limit)를 초과하면 토큰이 확보될 때까지 대기합니다.
	// 컨텍스트가 취소되면 Wait는 즉시 에러를 반환합니다.
	if n.limiter != nil {
		if err := n.limiter.Wait(ctx); err != nil {
			applog.WithComponentAndFields("notification.telegram", applog.Fields{
				"notifier_id": n.ID(),
				"error":       err,
			}).Debug("RateLimiter 대기 중 컨텍스트 취소됨 (전송 중단)")
			return
		}
	}

	const maxRetries = 3

	var err error
	for i := 0; i < maxRetries; i++ {
		// 전송 전 컨텍스트 확인
		select {
		case <-ctx.Done():
			return
		default:
		}

		if _, err = n.botAPI.Send(messageConfig); err == nil {
			// 성공 시 루프 탈출
			applog.WithComponentAndFields("notification.telegram", applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"attempt":     i + 1,
			}).Info("알림메시지 발송 성공")

			return
		}

		// 실패 시 로그 남기고 대기
		applog.WithComponentAndFields("notification.telegram", applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"attempt":     i + 1,
			"error":       err,
		}).Warn("알림메시지 발송 실패, 재시도 대기중...")

		// 4xx 에러(Client Error)인 경우 재시도해도 실패할 것이 확실하므로 중단
		var errCode int
		if apiErr, ok := err.(tgbotapi.Error); ok {
			errCode = apiErr.Code
		} else if apiErrPtr, ok := err.(*tgbotapi.Error); ok {
			errCode = apiErrPtr.Code
		}

		if errCode >= 400 && errCode < 500 {
			// Rate Limit (429) 에러는 라이브러리/tgbotapi가 처리해주지 않는 경우 재시도 해야하나,
			// tgbotapi Error 구조체에는 RetryAfter가 포함되어 있음.
			// 429 Too Many Requests는 400번대지만 일시적 오류일 수 있음.
			// 그러나 위에서 n.limiter를 사용하므로 429 발생 가능성은 낮음.
			// 일반적인 400(Bad Request), 401(Unauthorized) 등은 즉시 중단.
			if errCode != 429 {
				applog.WithComponentAndFields("notification.telegram", applog.Fields{
					"notifier_id": n.ID(),
					"error":       err,
					"code":        errCode,
				}).Error("치명적인 API 오류 발생, 재시도 중단")
				return
			}
		}

		// 안전한 대기: 컨텍스트 취소 시 즉시 반환
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.retryDelay):
			// 재시도 대기 완료, 다음 루프 진행
		}
	}

	// 모든 재시도 실패 시 에러 로그
	applog.WithComponentAndFields("notification.telegram", applog.Fields{
		"notifier_id": n.ID(),
		"chat_id":     n.chatID,
		"error":       err,
		"max_retries": maxRetries,
	}).Error("알림메시지 발송 최종 실패")
}

// safeSplit UTF-8 문자열을 지정된 바이트 길이(limit) 내에서 안전하게 자릅니다.
// 문자가 깨지지 않도록 가장 마지막 유효한 룬 경계에서 자릅니다.
func safeSplit(s string, limit int) (chunk, remainder string) {
	if len(s) <= limit {
		return s, ""
	}

	// limit 위치에서 자를 때 해당 위치가 문자의 중간이라면,
	// 앞쪽으로 이동하여 온전한 글자까지만 포함합니다.
	// utf8.RuneStart 함수는 해당 바이트가 룬의 시작 바이트인지 확인합니다.
	splitIndex := limit
	for splitIndex > 0 && !utf8.RuneStart(s[splitIndex]) {
		splitIndex--
	}

	// 만약 splitIndex가 0까지 갔다면(매우 드문 경우지만),
	// limit 이후의 첫 번째 룬 시작점을 찾거나, 포기하고 limit로 자릅니다.
	// 그러나 limit가 충분히 크다면(예: 3900), 이런 경우는 발생하지 않아야 합니다.
	if splitIndex == 0 {
		return s[:limit], s[limit:]
	}

	return s[:splitIndex], s[splitIndex:]
}

// truncateString 문자열을 지정된 rune 길이로 자르고 "..."을 붙입니다.
func truncateString(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}

	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return s
}
