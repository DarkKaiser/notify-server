package telegram

import (
	"context"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTelegramNotifier_SendTimeout_Log
// 네트워크 전송 타임아웃 발생 시, 함수가 정상적으로 종료되는지 검증합니다.
func TestTelegramNotifier_SendTimeout_Log(t *testing.T) {
	// Setup
	mockBot := &MockTelegramBot{}
	n := &telegramNotifier{
		botAPI:     mockBot,
		chatID:     12345,
		retryDelay: 10 * time.Millisecond,
	}

	// Mock 설정: Send 호출 시 즉시 에러 반환 (재시도 로직 진입 유도)
	mockBot.On("Send", mock.Anything).Return(tgbotapi.Message{}, &tgbotapi.Error{Code: 500, Message: "Internal Server Error"}).Maybe()

	// 짧은 타임아웃 컨텍스트 생성 (50ms)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// 재시도 대기 시간을 타임아웃보다 길게 설정 (100ms)
	n.retryDelay = 100 * time.Millisecond

	start := time.Now()

	// 실행
	// sendSingleMessage 내부에서 ctx.Done()을 체크하므로 50ms 후 종료되어야 함
	n.sendSingleMessage(ctx, "Test Timeout Message")

	elapsed := time.Since(start)

	// 검증
	// 50ms (타임아웃) + 알파 내에 종료되어야 함.
	// 너무 빨리 끝나면(지연 없이 바로 리턴) Fail, 너무 오래 걸리면(타임아웃 무시) Fail.
	require.GreaterOrEqual(t, elapsed.Milliseconds(), int64(50), "타임아웃 시간보다 너무 빨리 종료되었습니다.")
	require.Less(t, elapsed.Milliseconds(), int64(150), "타임아웃이 발생했음에도 함수가 즉시 종료되지 않았습니다.")
}
