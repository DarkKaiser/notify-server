package telegram

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// sendNotification 원본 알림 메시지에 메타데이터를 추가한 후, 텔레그램으로 메시지를 전송합니다.
//
// 이 함수는 알림 전송 파이프라인의 핵심 진입점으로, 다음 두 단계로 구성됩니다:
//  1. 메시지 강화(enrichment): 원본 알림 메시지에 제목, 경과 시간, 에러 상태 등의 메타데이터를 추가
//  2. 텔레그램 API를 통한 실제 메시지 전송
//
// 중요: 텔레그램 Notifier는 HTML 서식을 지원(SupportsHTML=true)하므로,
// 사용자가 제공한 메시지 내용을 이스케이프하지 않고 그대로 허용합니다.
// 따라서 사용자는 <b>Bold</b>, <i>Italic</i> 등의 HTML 태그를 사용하여 메시지를 서식화할 수 있습니다.
//
// 파라미터:
//   - ctx: 메시지 전송 작업의 생명주기를 제어하는 컨텍스트 (취소 시그널, 타임아웃 등)
//   - notification: 알림 정보를 담은 객체 (메시지, 제목, 작업 ID, 에러 상태 등)
func (n *telegramNotifier) sendNotification(ctx context.Context, notification *contract.Notification) {
	// 1단계: 메시지 강화
	// 원본 메시지에 작업 제목, 취소 명령어, 경과 시간, 에러 표시 등을 추가하여
	// 사용자에게 더 풍부한 정보를 제공합니다.
	message := n.buildEnrichMessage(notification)

	// 2단계: 최종 메시지 전송
	// 강화된 메시지를 텔레그램 API를 통해 실제로 전송합니다.
	// 메시지가 4096자를 초과하는 경우 자동으로 분할하여 전송됩니다.
	n.sendMessage(ctx, message)
}

// sendMessage 긴 메시지를 텔레그램 API 제한에 맞춰 "지능적으로 분할"하여 전송합니다.
//
// 배경:
// 텔레그램 Bot API는 단일 메시지의 최대 길이를 4096 바이트로 엄격하게 제한합니다.
// 이를 초과하는 메시지를 그대로 전송하면 400 Bad Request 에러가 발생합니다.
// 따라서 긴 로그, 에러 메시지, 또는 대량의 데이터를 전송할 때는 반드시 분할이 필요합니다.
//
// 분할 전략:
// 단순히 4096 바이트마다 자르면 다음과 같은 문제가 발생합니다:
//   - 문장 중간에서 잘려 가독성이 크게 떨어짐
//   - 멀티바이트 문자(한글, 이모지 등)가 바이트 경계에서 깨질 수 있음
//   - HTML 태그가 중간에 잘려 파싱 에러 발생 가능
//
// 이를 해결하기 위해 다음과 같은 3단계 지능형 분할 전략을 사용합니다:
//
//  1. 논리적 분할 (Line-based Chunking):
//     - 가능한 한 줄바꿈(\n) 단위로 메시지를 나눕니다.
//     - 이를 통해 문장이나 로그 항목이 중간에 잘리지 않도록 보장합니다.
//
//  2. 강제 분할 (Safe Split):
//     - 한 줄 자체가 4096 바이트를 초과하는 경우에만 강제로 자릅니다.
//     - 이때 UTF-8 문자 경계를 존중하여 멀티바이트 문자가 깨지지 않도록 합니다.
//
//  3. 순차 전송 및 조기 중단:
//     - 분할된 청크들은 원래 순서대로 하나씩 전송됩니다.
//     - 중간에 전송 실패가 발생하면 즉시 중단하여 불필요한 API 호출을 방지합니다.
//     - 컨텍스트 취소 시그널도 루프 중간에 확인하여 즉시 반응합니다.
//
// 파라미터:
//   - ctx: 메시지 전송 작업의 생명주기를 제어하는 컨텍스트 (취소 시그널, 타임아웃 등)
//   - message: 전송할 원본 메시지 (길이 제한 없음, 빈 문자열도 허용)
func (n *telegramNotifier) sendMessage(ctx context.Context, message string) {
	// ========================================
	// 1단계: 짧은 메시지는 즉시 전송
	// ========================================
	if len(message) <= messageMaxLength {
		_ = n.sendChunk(ctx, message)
		return
	}

	// ========================================
	// 2단계: 긴 메시지 분할 준비
	// ========================================
	var sb strings.Builder

	// 예상 크기만큼 미리 메모리를 할당하여 재할당 횟수를 줄입니다.
	sb.Grow(messageMaxLength)

	// ========================================
	// 3단계: 줄 단위로 메시지 순회
	// ========================================
	lines := strings.SplitSeq(message, "\n")
	for line := range lines {
		// 컨텍스트 취소 확인: 사용자가 작업을 취소했거나 타임아웃이 발생하면 즉시 중단
		// 긴 메시지를 처리하는 중에도 빠르게 반응하기 위해 매 루프마다 확인합니다.
		if ctx.Err() != nil {
			return
		}

		// ========================================
		// 4단계: 현재 라인을 추가할 공간 계산
		// ========================================
		neededSpace := len(line)
		if sb.Len() > 0 {
			// 청크에 이미 내용이 있다면 줄바꿈 문자(\n) 1바이트가 추가로 필요합니다.
			neededSpace += 1
		}

		// ========================================
		// 5단계: 청크 크기 초과 여부 판단
		// ========================================
		// 현재 청크 + 줄바꿈 + 새 라인을 합치면 최대 길이를 초과하는지 확인합니다.
		if sb.Len()+neededSpace > messageMaxLength {
			// ----------------------------------------
			// 5-1. 현재 청크 전송
			// ----------------------------------------
			// 지금까지 모은 청크가 있다면 먼저 전송하고 비웁니다.
			if sb.Len() > 0 {
				if err := n.sendChunk(ctx, sb.String()); err != nil {
					// 전송 실패 시 즉시 중단하여 불필요한 API 호출을 방지합니다.
					return
				}

				// 청크를 비워서 다음 메시지를 준비합니다.
				sb.Reset()
			}

			// ----------------------------------------
			// 5-2. 초장문 라인 처리 (강제 분할)
			// ----------------------------------------
			// 현재 라인 자체가 최대 길이를 초과하는 경우 (예: 4096자 이상의 한 줄짜리 로그)
			// 이런 경우 줄바꿈으로는 분할할 수 없으므로 강제로 잘라야 합니다.
			if len(line) > messageMaxLength {
				currentLine := line

				// 라인이 충분히 짧아질 때까지 반복해서 자릅니다.
				for len(currentLine) > messageMaxLength {
					// 긴 루프 중에도 컨텍스트 취소에 즉시 반응합니다.
					if ctx.Err() != nil {
						return
					}

					chunk, remainder := safeSplit(currentLine, messageMaxLength)
					if err := n.sendChunk(ctx, chunk); err != nil {
						// 전송 실패 시 즉시 중단하여 불필요한 API 호출을 방지합니다.
						return
					}
					currentLine = remainder // 남은 부분으로 계속 진행
				}

				// 자르고 남은 마지막 조각을 새로운 청크의 시작으로 설정합니다.
				// 이 조각은 다음 라인들과 합쳐질 수 있습니다.
				sb.WriteString(currentLine)
			} else {
				// ----------------------------------------
				// 5-3. 일반 라인 처리
				// ----------------------------------------
				// 현재 라인은 최대 길이 이내이므로 새로운 청크의 시작으로 설정합니다.
				sb.WriteString(line)
			}
		} else {
			// ========================================
			// 6단계: 청크에 라인 추가
			// ========================================
			// 현재 청크에 라인을 추가해도 최대 길이를 넘지 않으므로 안전하게 추가합니다.
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(line)
		}
	}

	// ========================================
	// 7단계: 마지막 청크 전송
	// ========================================
	// 루프가 끝났지만 아직 전송하지 않은 마지막 청크가 남아있을 수 있습니다.
	// 이를 전송하여 모든 내용이 전달되도록 보장합니다.
	if sb.Len() > 0 {
		_ = n.sendChunk(ctx, sb.String())
	}
}

// sendChunk 단일 메시지 청크를 텔레그램 API로 전송합니다.
//
// 이 함수는 이미 분할된(chunked) 메시지 조각을 실제로 텔레그램 API로 전송하는 역할을 합니다.
// HTML 파싱 모드를 활성화하여 전송하며, 실패 시 자동으로 재시도 로직이 적용됩니다.
//
// 파라미터:
//   - ctx: 메시지 전송 작업의 생명주기를 제어하는 컨텍스트 (취소 시그널, 타임아웃 등)
//   - message: 전송할 메시지 내용 (이미 길이 제한 내로 분할된 상태)
//
// 반환값:
//   - error: 메시지 전송 실패 시 에러, 성공 시 nil
func (n *telegramNotifier) sendChunk(ctx context.Context, message string) error {
	return n.attemptSendWithRetry(ctx, message, true)
}

// attemptSendWithRetry 텔레그램 메시지 전송을 시도하며, 실패 시 자동으로 재시도합니다.
//
// 이 함수는 텔레그램 메시지 전송의 "복원력(resilience)"을 담당하는 핵심 엔진입니다.
// 네트워크 불안정, 서버 과부하, API 제한 등 다양한 실패 상황에서도 메시지가
// 안정적으로 전달될 수 있도록 다음과 같은 고급 기능을 제공합니다:
//
// 핵심 기능:
//
//  1. Rate Limiting (속도 제한 준수):
//     - 텔레그램 API의 분당 전송 횟수 제한을 자동으로 준수합니다.
//
//  2. 지능형 재시도 (Smart Retry):
//     - 일시적 오류 발생 시 최대 3회까지 자동으로 재시도합니다.
//     - 재시도 가능한 에러(5xx, 429)와 불가능한 에러(4xx)를 구분하여 처리합니다.
//     - 불필요한 재시도를 방지하여 리소스를 절약합니다.
//
//  3. 적응형 대기 (Adaptive Backoff):
//     - 429 Rate Limit 에러 시 서버가 요청한 시간(Retry-After)만큼 정확히 대기합니다.
//     - 일반 에러는 기본 대기 시간을 사용하여 서버 부하를 분산시킵니다.
//
//  4. HTML Fallback (자동 모드 전환):
//     - HTML 파싱 실패(400 에러) 시 자동으로 PlainText 모드로 전환하여 재시도합니다.
//     - 메시지 내용은 그대로 유지하되, 태그만 문자 그대로 표시하여 전송을 보장합니다.
//     - 재귀 호출로 구현되어 있어 Fallback 후에도 모든 재시도 로직이 동일하게 적용됩니다.
//
//  5. 컨텍스트 인식 (Context-Aware):
//     - 사용자의 취소 시그널이나 타임아웃을 즉시 감지하여 반응합니다.
//     - 재시도 대기 중에도 컨텍스트를 확인하여 불필요한 대기를 방지합니다.
//     - 타임아웃 에러는 특별히 로그를 남겨 운영 모니터링을 지원합니다.
//
// 동작 흐름:
//  1. Rate Limiter 통과 대기
//  2. 텔레그램 API 호출 (최대 3회 재시도)
//  3. 성공 시 즉시 반환
//  4. 실패 시 에러 분석:
//     - HTML 파싱 에러(400) → PlainText로 Fallback
//     - 재시도 불가 에러(4xx) → 즉시 실패 반환
//     - 재시도 가능 에러(5xx, 429) → 대기 후 재시도
//  5. 모든 재시도 실패 시 최종 에러 반환
//
// 파라미터:
//   - ctx: 메시지 전송 작업의 생명주기를 제어하는 컨텍스트 (취소 시그널, 타임아웃 등)
//   - message: 전송할 메시지 내용 (이미 길이 제한 내로 분할된 상태)
//   - useHTML: true면 HTML 파싱 모드, false면 PlainText 모드
//
// 반환값:
//   - error: 최종 전송 실패 시 에러, 성공 시 nil
func (n *telegramNotifier) attemptSendWithRetry(ctx context.Context, message string, useHTML bool) error {
	// ========================================
	// 1단계: 메시지 설정 준비
	// ========================================
	messageConfig := tgbotapi.NewMessage(n.chatID, message)
	if useHTML {
		messageConfig.ParseMode = tgbotapi.ModeHTML
	} else {
		messageConfig.ParseMode = ""
	}

	// ========================================
	// 2단계: Rate Limiting 적용
	// ========================================
	// 텔레그램 API는 분당 전송 횟수를 제한합니다 (예: 30메시지/초).
	// Rate Limiter는 토큰 버킷(Token Bucket) 알고리즘을 사용하여 이 제한을 준수합니다:
	//   - 토큰이 있으면 즉시 통과
	//   - 토큰이 없으면 토큰이 재충전될 때까지 대기
	//   - 컨텍스트 취소 시 즉시 에러 반환
	if n.rateLimiter != nil {
		if err := n.rateLimiter.Wait(ctx); err != nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"error":       err,
				"limit":       n.rateLimiter.Limit(),
				"burst":       n.rateLimiter.Burst(),
			}).Debug("작업 중단: RateLimiter 대기 중 컨텍스트가 취소되었습니다")

			return err
		}
	}

	// ========================================
	// 3단계: 재시도 루프 초기화
	// ========================================
	// 최대 3회까지 재시도합니다.
	// 일시적 네트워크 오류나 서버 과부하 상황에서 복원력을 제공합니다.
	const maxRetries = 3
	var lastErr error

	// ========================================
	// 4단계: 재시도 루프 시작
	// ========================================
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// ----------------------------------------
		// 4-1. 전송 전 컨텍스트 확인
		// ----------------------------------------
		// non-blocking select로 컨텍스트 취소를 확인합니다.
		// 취소되었다면 즉시 에러를 반환하여 불필요한 API 호출을 방지합니다.
		select {
		case <-ctx.Done():
			// 타임아웃 에러는 특별히 로그를 남깁니다 (운영 모니터링에 중요).
			if ctx.Err() == context.DeadlineExceeded {
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id": n.ID(),
					"error":       ctx.Err(),
					"attempt":     attempt,
				}).Error("작업 중단: 발송 제한 시간(Timeout)을 초과하였습니다")
			}
			return ctx.Err()

		default:
			// 컨텍스트가 정상이므로 계속 진행
		}

		// ----------------------------------------
		// 4-2. 텔레그램 API 호출
		// ----------------------------------------
		_, err := n.client.Send(messageConfig)
		if err == nil {
			// ----------------------------------------
			// 4-3. 성공 처리
			// ----------------------------------------
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id":    n.ID(),
				"chat_id":        n.chatID,
				"attempt":        attempt,
				"mode":           formatParseMode(messageConfig.ParseMode),
				"message_length": len(message),
			}).Info("발송 성공: 텔레그램 API로 메시지가 정상 전송되었습니다")

			return nil
		}

		// ----------------------------------------
		// 4-4. 실패 처리 및 에러 분석
		// ----------------------------------------
		lastErr = err
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id":    n.ID(),
			"chat_id":        n.chatID,
			"attempt":        attempt,
			"error":          err,
			"mode":           formatParseMode(messageConfig.ParseMode),
			"message_length": len(message),
		}).Warn("발송 실패: 텔레그램 API 호출에서 오류가 발생했습니다 (재시도 예정)")

		// 텔레그램 API 에러에서 HTTP 상태 코드와 Retry-After 값을 추출합니다.
		// 이 정보는 재시도 전략을 결정하는 데 사용됩니다.
		errCode, retryAfter := parseTelegramError(err)

		// ----------------------------------------
		// 4-5. HTML Fallback 메커니즘
		// ----------------------------------------
		// 400 Bad Request 에러는 대부분 HTML 파싱 실패를 의미합니다.
		// 예: 닫히지 않은 태그, 잘못된 HTML 문법 등
		//
		// 이 경우 HTML 모드를 끄고 PlainText 모드로 재귀 호출합니다.
		// 중요: 메시지 내용은 그대로 유지하고, 파싱 모드만 변경합니다.
		if useHTML && errCode == 400 {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id":    n.ID(),
				"error":          err,
				"attempt":        attempt,
				"message_length": len(message),
			}).Warn("HTML 파싱 오류(400): PlainText 모드로 자동 전환하여 재시도합니다 (Fallback)")

			return n.attemptSendWithRetry(ctx, message, false)
		}

		// ----------------------------------------
		// 4-6. 재시도 가능 여부 판단
		// ----------------------------------------
		if !shouldRetry(errCode) {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"error":       err,
				"code":        errCode,
				"attempt":     attempt,
			}).Error("작업 중단: 재시도 불가능한 API 오류가 발생했습니다 (4xx Fatal Error)")

			return err
		}

		// 마지막 시도였다면 루프를 빠져나가 최종 실패 처리로 이동합니다.
		if attempt >= maxRetries {
			break
		}

		// ----------------------------------------
		// 4-7. 재시도 대기
		// ----------------------------------------
		// 429 Rate Limit 에러 시 특별 처리:
		// 텔레그램 서버가 Retry-After 헤더로 대기 시간을 명시할 수 있습니다.
		// 이 경우 서버가 요청한 시간만큼 정확히 대기해야 합니다.
		if errCode == 429 && retryAfter > 0 {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"retry_after": retryAfter,
				"attempt":     attempt,
				"limit":       n.rateLimiter.Limit(),
				"burst":       n.rateLimiter.Burst(),
			}).Warn("재시도 대기: 429 Rate Limit 감지 (Retry-After 준수)")
		}

		backoff := n.delayForRetry(retryAfter)
		select {
		case <-ctx.Done():
			// 대기 중 컨텍스트가 취소되면 즉시 반환합니다.
			if ctx.Err() == context.DeadlineExceeded {
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id": n.ID(),
					"error":       ctx.Err(),
					"backoff":     backoff,
					"attempt":     attempt,
				}).Error("재시도 중단: 대기 중 작업 제한 시간(Timeout)을 초과하였습니다")
			}
			return ctx.Err()

		case <-time.After(backoff):
			// 대기 시간이 경과하면 다음 재시도로 진행합니다.
		}
	}

	// ========================================
	// 5단계: 최종 실패 처리
	// ========================================
	// 모든 재시도가 실패한 경우 여기에 도달합니다.
	// 운영 모니터링을 위해 상세한 에러 로그를 남깁니다.
	applog.WithComponentAndFields(component, applog.Fields{
		"notifier_id":    n.ID(),
		"chat_id":        n.chatID,
		"error":          lastErr,
		"max_retries":    maxRetries,
		"message_length": len(message),
		"use_html":       useHTML,
	}).Error("전송 최종 실패: 최대 재시도 횟수를 초과하였습니다")

	return lastErr
}

// shouldRetry 주어진 HTTP 상태 코드를 기반으로 메시지 전송 재시도 가능 여부를 판단합니다.
//
// HTTP 상태 코드 분류:
//   - 4xx (Client Error): 클라이언트 측 문제 → 재시도 불가능
//   - 429 (Too Many Requests): Rate Limit → 재시도 가능 (예외)
//   - 5xx (Server Error): 서버 측 일시적 문제 → 재시도 가능
//
// 파라미터:
//   - statusCode: HTTP 상태 코드
//
// 반환값: 메시지 전송 재시도 가능 여부 (true: 가능, false: 불가)
func shouldRetry(statusCode int) bool {
	if statusCode >= 400 && statusCode < 500 {
		// 예외: 429 Too Many Requests는 Rate Limit이므로 재시도 가능
		return statusCode == 429
	}

	// 5xx 서버 에러 및 기타 모든 에러는 재시도 가능 (네트워크 에러 등 errCode=0인 경우도 포함)
	return true
}

// delayForRetry 메시지 전송 실패 시, 다음 재시도까지의 대기 시간을 계산합니다.
//
// 텔레그램 API는 429 에러 발생 시 Retry-After 헤더로 대기 시간을 지정할 수 있습니다.
// 이 값을 우선 사용하고, 없으면 기본 대기 시간(retryDelay)을 사용합니다.
//
// 파라미터:
//   - retryAfter: 서버가 요청한 대기 시간(초)
//
// 반환값:
//   - time.Duration: 다음 재시도까지의 대기 시간
func (n *telegramNotifier) delayForRetry(retryAfter int) time.Duration {
	// 서버가 명시적으로 대기 시간을 지정한 경우 (429 에러 시)
	if retryAfter > 0 {
		return time.Duration(retryAfter) * time.Second
	}

	// 기본 대기 시간 사용 (일반적인 재시도)
	return n.retryDelay
}

// formatParseMode 텔레그램 메시지 파싱 모드를 로깅용 문자열로 변환합니다.
//
// 텔레그램 API의 ParseMode 값을 사람이 읽기 쉬운 형태로 변환하여
// 로그 메시지에 포함시킬 때 사용합니다.
//
// 파라미터:
//   - mode: 텔레그램 메시지 파싱 모드 (예: tgbotapi.ModeHTML, "" 등)
//
// 반환값:
//   - "HTML": HTML 모드인 경우
//   - "PlainText": 그 외 모든 경우 (빈 문자열 포함)
func formatParseMode(mode string) string {
	if mode == tgbotapi.ModeHTML {
		return "HTML"
	}
	return "PlainText"
}

// parseTelegramError 텔레그램 API 에러에서 에러 코드와 Retry-After 값을 추출합니다.
//
// 이 함수는 일반 error 인터페이스에서 텔레그램 특화 에러 정보를 추출하여
// 재시도 로직과 에러 처리에 필요한 정보를 제공합니다.
//
// 파라미터:
//   - err: 텔레그램 API 호출에서 발생한 에러
//
// 반환값:
//   - code: HTTP 상태 코드 (예: 400, 401, 429, 500 등)
//   - retryAfter: 429 에러 시 서버가 요청한 대기 시간(초), 없으면 0
func parseTelegramError(err error) (code int, retryAfter int) {
	// 값 타입으로 어설션 시도
	if apiErr, ok := err.(tgbotapi.Error); ok {
		return apiErr.Code, apiErr.ResponseParameters.RetryAfter
	}

	// 포인터 타입으로 어설션 시도
	if apiErrPtr, ok := err.(*tgbotapi.Error); ok {
		return apiErrPtr.Code, apiErrPtr.ResponseParameters.RetryAfter
	}

	// 텔레그램 에러가 아닌 경우 (일반 네트워크 에러 등)
	return 0, 0
}

// safeSplit UTF-8 문자열을 지정된 바이트 길이(limit) 내에서 안전하게 분할합니다.
//
// 이 함수는 텔레그램 API의 메시지 길이 제한(바이트 단위)을 준수하면서
// 멀티바이트 문자(한글, 이모지 등)가 깨지지 않도록 보장합니다.
//
// 바이트 vs 룬(문자):
//   - 바이트: 메모리 저장 단위 (한글 1글자 = 3바이트, 이모지 = 4바이트)
//   - 룬: 사용자가 보는 문자 단위 (한글 1글자 = 1룬)
//   - 텔레그램 API는 바이트 단위로 제한하므로 바이트 기준 분할 필요
//
// 파라미터:
//   - s: 분할할 원본 문자열
//   - limit: 첫 번째 청크의 최대 바이트 길이
//
// 반환값:
//   - chunk: limit 이내의 안전하게 잘린 첫 번째 부분
//   - remainder: 나머지 부분 (빈 문자열일 수 있음)
func safeSplit(s string, limit int) (chunk, remainder string) {
	// 문자열이 제한보다 짧으면 분할 불필요
	if len(s) <= limit {
		return s, ""
	}

	// 1단계: 룬 경계 찾기
	// limit 위치가 멀티바이트 문자의 중간일 수 있으므로,
	// 뒤로 이동하며 가장 가까운 룬 시작 위치를 찾습니다.
	//
	// UTF-8 인코딩 구조:
	//   - 1바이트 문자: 0xxxxxxx (ASCII)
	//   - 멀티바이트 시작: 110xxxxx, 1110xxxx, 11110xxx
	//   - 연속 바이트: 10xxxxxx (룬의 중간 부분)
	//
	// utf8.RuneStart()는 해당 바이트가 룬의 시작인지 확인합니다.
	splitIndex := limit
	for splitIndex > 0 && !utf8.RuneStart(s[splitIndex]) {
		splitIndex--
	}

	// 2단계: 엣지 케이스 처리
	// splitIndex가 0까지 후퇴한 경우는 매우 드물지만,
	// limit 이전에 유효한 룬 시작점이 없다는 의미입니다.
	// 이 경우 강제로 limit에서 자르되, 깨진 문자는 감수합니다.
	// (실제로는 limit가 3900바이트이므로 발생 가능성 극히 낮음)
	if splitIndex == 0 {
		return s[:limit], s[limit:]
	}

	// 3단계: 안전한 위치에서 분할
	// splitIndex는 룬의 시작 위치이므로 여기서 자르면 문자가 깨지지 않습니다.
	return s[:splitIndex], s[splitIndex:]
}
