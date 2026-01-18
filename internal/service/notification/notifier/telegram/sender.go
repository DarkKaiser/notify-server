package telegram

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// TODO 미완료

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

// handleNotifyRequest 시스템 알림 전송 요청을 처리하고, 작업 컨텍스트 정보를 메시지에 추가하여 텔레그램으로 발송합니다.
func (n *telegramNotifier) handleNotifyRequest(ctx context.Context, req *notifier.Request) {
	// 텔레그램 Notifier는 SupportsHTML=true이므로, 사용자 메시지를 이스케이프하지 않고 그대로 허용합니다.
	// 사용자는 <b>Bold</b> 등의 태그를 사용하여 메시지를 서식화할 수 있습니다.
	message := req.Message

	// 작업 실행과 관련된 컨텍스트 정보(작업명, 경과시간 등)가 있다면 메시지에 덧붙입니다.
	if req.TaskContext != nil {
		message = n.enrichMessageWithContext(req.TaskContext, message)
	}

	// 최종 메시지 전송
	n.sendMessage(ctx, message)
}

// enrichMessageWithContext TaskContext 정보를 메시지에 추가 (제목, 시간, 에러 등)
func (n *telegramNotifier) enrichMessageWithContext(taskCtx contract.TaskContext, message string) string {
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
func (n *telegramNotifier) appendTitle(taskCtx contract.TaskContext, message string) string {
	if title := taskCtx.GetTitle(); len(title) > 0 {
		// 긴 제목으로 인해 HTML 태그가 닫히지 않은 채 메시지가 분할되는 등의 문제를 방지하기 위해 Truncate 처리
		// 중요: Truncate를 먼저 수행한 후 이스케이프해야 안전합니다.
		// 이스케이프된 문자열을 자르면 '&lt;' 따위가 잘려서 '&l' 처럼 되어 HTML 파싱 에러를 유발할 수 있습니다.
		safeTitle := html.EscapeString(strutil.Truncate(title, titleTruncateLength))
		return fmt.Sprintf(msgContextTitle, safeTitle, message)
	}

	// 제목이 없으면 ID를 기반으로 lookup하여 제목을 찾음
	taskID := taskCtx.GetTaskID()
	commandID := taskCtx.GetTaskCommandID()

	if !taskID.IsEmpty() && !commandID.IsEmpty() {
		// O(1) Map 조회로 성능 개선 (중첩 맵 사용)
		if commands, ok := n.botCommandsByTaskAndCommand[string(taskID)]; ok {
			if botCommand, exists := commands[string(commandID)]; exists {
				return fmt.Sprintf(msgContextTitle, html.EscapeString(botCommand.title), message)
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
func (n *telegramNotifier) appendCancelCommand(taskCtx contract.TaskContext, message string) string {
	if !taskCtx.IsCancelable() {
		return message
	}

	instanceID := taskCtx.GetTaskInstanceID()
	if instanceID.IsEmpty() {
		return message
	}

	return fmt.Sprintf("%s\n\n%s%s%s%s", message, botCommandInitialCharacter, botCommandCancel, botCommandSeparator, instanceID)
}

// appendElapsedTime 실행 경과 시간을 메시지에 추가합니다.
func (n *telegramNotifier) appendElapsedTime(taskCtx contract.TaskContext, message string) string {
	if elapsedTimeAfterRun := taskCtx.GetElapsedTimeAfterRun(); elapsedTimeAfterRun > 0 {
		return message + formatElapsedTime(elapsedTimeAfterRun)
	}
	return message
}

// formatElapsedTime 초 단위 시간을 읽기 쉬운 문자열로 변환 (예: 1시간 30분 10초)
// 모든 값이 0일 때는 "0초"를 표시하고, 시간/분이 있을 때는 0초를 생략합니다.
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

	// 모든 값이 0인 경우 "0초" 표시
	if sb.Len() == 0 {
		sb.WriteString("0초 ")
	}

	return fmt.Sprintf(msgElapsedTime, sb.String())
}

// sendMessage 텔레그램 메시지 전송
// API 제한(4096자)을 초과하는 메시지는 자동으로 분할하여 전송합니다.
// 컨텍스트가 취소되거나 전송 중 오류가 발생하면 즉시 중단하고 반환합니다.
func (n *telegramNotifier) sendMessage(ctx context.Context, message string) {
	// 메시지 길이가 제한 이내라면 한 번에 전송
	if len(message) <= telegramMessageMaxLength {
		_ = n.sendSingleMessage(ctx, message)
		return
	}

	// 제한을 초과하는 경우, 줄바꿈(\n) 단위로 메시지를 나눕니다.
	var sb strings.Builder
	// strings.Builder는 초기 용량을 설정할 수 없지만, 대략적인 크기를 알면 Grow를 쓸 수 있음.
	// 여기서는 매번 Reset되므로 큰 의미는 없을 수 있으나, 빈번한 재할당을 줄이기 위해 사용.
	sb.Grow(telegramMessageMaxLength)

	lines := strings.SplitSeq(message, "\n")
	for line := range lines {
		// 컨텍스트 취소 확인 (긴 루프 중간에 탈출)
		if ctx.Err() != nil {
			return
		}

		neededSpace := len(line)
		if sb.Len() > 0 {
			neededSpace += 1 // 줄바꿈 문자 공간
		}

		// 현재 청크 + (줄바꿈) + 새 라인이 최대 길이를 넘으면
		if sb.Len()+neededSpace > telegramMessageMaxLength {
			// 현재까지 모은 청크가 있다면 전송
			if sb.Len() > 0 {
				if err := n.sendSingleMessage(ctx, sb.String()); err != nil {
					return // 전송 실패 시 중단
				}
				sb.Reset()
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
					if err := n.sendSingleMessage(ctx, chunk); err != nil {
						return // 전송 실패 시 중단
					}
					currentLine = remainder
				}
				// 자르고 남은 뒷부분을 새로운 청크의 시작으로 설정
				sb.WriteString(currentLine)
			} else {
				// 현재 라인은 최대 길이 이내이므로 새로운 청크로 설정
				sb.WriteString(line)
			}
		} else {
			// 청크에 라인 추가
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(line)
		}
	}

	// 마지막 남은 청크 전송
	if sb.Len() > 0 {
		_ = n.sendSingleMessage(ctx, sb.String())
	}
}

// extractTelegramErrorCode 텔레그램 API 에러에서 에러 코드와 Retry-After 값을 추출합니다.
func extractTelegramErrorCode(err error) (code int, retryAfter int) {
	if apiErr, ok := err.(tgbotapi.Error); ok {
		return apiErr.Code, apiErr.ResponseParameters.RetryAfter
	}
	if apiErrPtr, ok := err.(*tgbotapi.Error); ok {
		return apiErrPtr.Code, apiErrPtr.ResponseParameters.RetryAfter
	}
	return 0, 0
}

// shouldRetryError 주어진 에러가 재시도 가능한지 판단합니다.
// 429 (Too Many Requests)는 재시도 가능, 기타 4xx는 재시도 불가능.
func shouldRetryError(errCode int) bool {
	if errCode >= 400 && errCode < 500 {
		return errCode == 429 // 429만 재시도 가능
	}
	return true // 5xx 등은 재시도 가능
}

// getRetryWaitDuration 재시도 대기 시간을 계산합니다.
// Retry-After 헤더가 있으면 그 값을 사용하고, 없으면 기본 대기 시간을 사용합니다.
func (n *telegramNotifier) getRetryWaitDuration(retryAfter int) time.Duration {
	if retryAfter > 0 {
		return time.Duration(retryAfter) * time.Second
	}
	return n.retryDelay
}

// sendSingleMessage 단일 메시지 전송
// 컨텍스트 취소(종료 시그널)를 감지하면 즉시 중단합니다.
func (n *telegramNotifier) sendSingleMessage(ctx context.Context, message string) error {
	return n.sendSingleMessageInternal(ctx, message, true)
}

func (n *telegramNotifier) sendSingleMessageInternal(ctx context.Context, message string, useHTML bool) error {
	messageConfig := tgbotapi.NewMessage(n.chatID, message)
	if useHTML {
		messageConfig.ParseMode = tgbotapi.ModeHTML
	} else {
		messageConfig.ParseMode = "" // Plain Text
	}

	// 텔레그램 API Rate Limit 준수를 위해 발송 속도를 제어합니다.
	// 지정된 속도(Limit)를 초과하면 토큰이 확보될 때까지 대기합니다.
	// 컨텍스트가 취소되면 Wait는 즉시 에러를 반환합니다.
	if n.limiter != nil {
		if err := n.limiter.Wait(ctx); err != nil {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"error":       err,
			}).Debug(constants.LogMsgTelegramRateLimitCancel)
			return err
		}
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// 전송 전 컨텍스트 확인
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
					"notifier_id": n.ID(),
					"error":       ctx.Err(),
				}).Error(constants.LogMsgTelegramSendTimeout)
			}
			return ctx.Err()
		default:
		}

		// 전송 시도
		_, err := n.botClient.Send(messageConfig)
		if err == nil {
			// 성공
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"attempt":     attempt,
				"mode":        parseModeToString(messageConfig.ParseMode),
			}).Info(constants.LogMsgTelegramSendSuccess)
			return nil
		}

		// 실패 로그
		lastErr = err
		applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"attempt":     attempt,
			"error":       err,
			"mode":        parseModeToString(messageConfig.ParseMode),
		}).Warn(constants.LogMsgTelegramSendFail)

		// 에러 분석
		errCode, retryAfter := extractTelegramErrorCode(err)

		// HTML 파싱 에러 시 Plain Text로 Fallback
		if useHTML && errCode == 400 {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"error":       err,
			}).Warn(constants.LogMsgTelegramHTMLFallback)
			return n.sendSingleMessageInternal(ctx, message, false)
		}

		// 재시도 가능 여부 판단
		if !shouldRetryError(errCode) {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"error":       err,
				"code":        errCode,
			}).Error(constants.LogMsgTelegramCriticalError)
			return err
		}

		// 마지막 시도였으면 재시도 대기 없이 종료
		if attempt >= maxRetries {
			break
		}

		// 429 에러 시 로그
		if errCode == 429 && retryAfter > 0 {
			applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
				"notifier_id": n.ID(),
				"retry_after": retryAfter,
			}).Warn(constants.LogMsgTelegramRateLimitWait)
		}

		// 재시도 대기
		waitDuration := n.getRetryWaitDuration(retryAfter)
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
					"notifier_id": n.ID(),
					"error":       ctx.Err(),
				}).Error(constants.LogMsgTelegramRetryTimeout)
			}
			return ctx.Err()
		case <-time.After(waitDuration):
			// 재시도 대기 완료
		}
	}

	// 최종 실패
	applog.WithComponentAndFields(constants.ComponentNotifierTelegram, applog.Fields{
		"notifier_id": n.ID(),
		"chat_id":     n.chatID,
		"error":       lastErr,
		"max_retries": maxRetries,
	}).Error(constants.LogMsgTelegramSendFinalFail)

	return lastErr
}

func parseModeToString(mode string) string {
	if mode == tgbotapi.ModeHTML {
		return "HTML"
	}
	return "PlainText"
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
