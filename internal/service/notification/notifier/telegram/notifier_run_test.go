package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestTelegramNotifier_Run_ChannelClosed_Fix
// Telegram의 Updates 채널이 예기치 않게 닫혔을 때,
// 무한 루프(Busy Loop)에 빠지지 않고 안전하게 종료(Return)되는지 검증합니다.
func TestTelegramNotifier_Run_ChannelClosed_Fix(t *testing.T) {
	// Setup
	appConfig := &config.AppConfig{}
	mockBot := &MockTelegramBot{}
	mockExecutor := &mocks.MockExecutor{}

	// Notifier 생성
	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, 11111, appConfig, mockExecutor)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	// Mock 설정
	// 1. GetSelf: 로그 출력용
	mockBot.On("GetSelf").Return(tgbotapi.User{UserName: "TestBot"})

	// 2. GetUpdatesChan: 테스트용 채널을 반환하고, 곧바로 닫아야 함
	updatesChan := make(chan tgbotapi.Update)
	// Mock implementation ignores Return value, uses field instead. Injecting manually.
	mockBot.updatesChan = updatesChan
	mockBot.On("GetUpdatesChan", mock.Anything).Return((tgbotapi.UpdatesChannel)(updatesChan))

	// 3. StopReceivingUpdates: 루프 탈출 시 호출되어야 함
	mockBot.On("StopReceivingUpdates").Return()

	// 테스트 실행
	// 별도 고루틴에서 Run 실행
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runResult := make(chan struct{})

	go func() {
		n.Run(ctx)
		close(runResult)
	}()

	// 채널 닫음 (오류 상황 시뮬레이션)
	close(updatesChan)

	// 검증: 무한 루프에 빠지지 않고 즉시 종료되어야 함
	select {
	case <-runResult:
		// 성공: Run 메서드가 리턴됨
	case <-time.After(2 * time.Second):
		t.Fatal("Updates 채널이 닫혔음에도 Run 메서드가 종료되지 않았습니다 (Busy Loop 가능성)")
	}
}
