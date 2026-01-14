package telegram

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTelegramNotifier_RetryAfter_Compliance
// 429 Too Many Requests 에러와 Retry-After 헤더가 수신되었을 때,
// 지정된 시간만큼 대기하고 재시도하는지 검증합니다.
func TestTelegramNotifier_RetryAfter_Compliance(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{}
	// Notifier 수동 생성 (테스트 대상 메서드가 private이므로, public interface나 struct method를 통해 호출해야 함.
	// 그러나 sendSingleMessage는 private이므로 handleNotifyRequest나 notify 등을 통해 간접 호출해야 함.
	// 여기서는 handleNotifyRequest를 통해 테스트합니다.
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 10 * time.Millisecond, // 기본 재시도 대기 시간 (짧게 설정)
		// RateLimiter는 nil로 두어 로직 간소화
	}

	// 1. 첫 번째 호출: 429 에러 + Retry-After 1초 반환
	retryAfterSeconds := 1
	apiErr := &tgbotapi.Error{
		Code:    429,
		Message: "Too Many Requests: retry after 1",
		ResponseParameters: tgbotapi.ResponseParameters{
			RetryAfter: retryAfterSeconds,
		},
	}

	// 2. 두 번째 호출: 성공
	// Mock 설정
	// 첫 번째 호출은 에러 반환
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, apiErr).Once()
	// 두 번째 호출은 성공
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, nil).Once()

	// 시간 측정 시작
	start := time.Now()

	// 실행
	ctx := context.Background()
	// handleNotifyRequest -> sendMessage -> sendSingleMessage
	n.sendSingleMessage(ctx, "Test Message")

	// 시간 측정 종료
	elapsed := time.Since(start)

	// 검증
	// 1. Mock 호출 검증 (총 2회 호출 확인)
	mockBot.AssertExpectations(t)

	// 2. 소요 시간 검증
	// Retry-After가 1초이므로, 최소 1초 이상 걸려야 함.
	// (기본 retryDelay는 10ms이므로, Retry-After를 무시했다면 훨씬 빨리 끝났을 것임)
	require.GreaterOrEqual(t, elapsed.Seconds(), float64(retryAfterSeconds), "Retry-After 시간만큼 대기하지 않았습니다.")
}

// TestTelegramNotifier_400_FallbackRetry
// 400 Bad Request 에러 발생 시, HTML 파싱 에러로 간주하고 Plain Text로 1회 재시도(Fallback)하는지 검증
// Fallback도 실패하면 즉시 중단되어야 함 (무한 루프 방지)
func TestTelegramNotifier_400_FallbackRetry(t *testing.T) {
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 100 * time.Millisecond,
	}

	// 400 에러
	apiErr := &tgbotapi.Error{
		Code:    400,
		Message: "Bad Request",
	}

	// 1. 첫 번째 호출 (HTML 모드) -> 400 에러
	mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ParseMode == tgbotapi.ModeHTML
	})).Return(tgbotapi.Message{}, apiErr).Once()

	// 2. 두 번째 호출 (Fallback: Plain Text 모드) -> 여전히 400 에러
	mockBot.On("Send", mock.MatchedBy(func(msg tgbotapi.MessageConfig) bool {
		return msg.ParseMode == ""
	})).Return(tgbotapi.Message{}, apiErr).Once()

	start := time.Now()
	// Default uses HTML=true
	n.sendSingleMessage(ctx, "Test Message")
	elapsed := time.Since(start)

	mockBot.AssertExpectations(t)

	// 재시도 대기(100ms) 없이 즉시 리턴해야 함 (Fallback은 즉시 실행되므로)
	require.Less(t, elapsed.Milliseconds(), int64(50), "Fallback 로직은 대기 시간 없이 즉시 실행되어야 합니다.")
}

var ctx = context.Background()
