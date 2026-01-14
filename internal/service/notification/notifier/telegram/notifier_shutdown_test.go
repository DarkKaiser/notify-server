package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/darkkaiser/notify-server/internal/service/task/mocks"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRunSender_GracefulShutdown_InFlightMessage(t *testing.T) {
	// Setup
	appConfig := &config.AppConfig{}

	// Mock Setup
	mockBot := &MockTelegramBot{}
	mockExecutor := &mocks.MockExecutor{}

	// Notifier 생성 (직접 생성하여 Mock 기대치 제어)
	nHandler, err := newTelegramNotifierWithBot("test-notifier", mockBot, 12345, appConfig, mockExecutor)
	require.NoError(t, err)
	n := nHandler.(*telegramNotifier)

	// 테스트용 메시지
	msg := "In-Flight Message"

	// 동기화 도구
	var wgSender sync.WaitGroup
	wgSender.Add(1) // Sender 종료 대기

	// Send 호출이 발생했는지 확인할 채널
	sendCalled := make(chan struct{})

	// 1. Send Mock 설정
	// 메인 컨텍스트가 취소된 이후에도 이 함수가 호출되어야 함
	mockBot.On("Send", mock.Anything).Run(func(args mock.Arguments) {
		close(sendCalled) // 호출됨을 알림
	}).Return(tgbotapi.Message{}, nil)

	// 2. Notifier 실행 및 메시지 주입
	ctx, cancel := context.WithCancel(context.Background())

	// Sender 실행
	go func() {
		defer wgSender.Done()
		n.runSender(ctx)
	}()

	// 메시지 주입 (RequestC에 넣음)
	n.RequestC <- &notifier.NotifyRequest{Message: msg}

	// 3. 중요: Sender가 메시지를 꺼내서 처리하기 직전/도중에 Context를 취소해야 함
	// 하지만 정확한 타이밍을 잡기 어려우므로,
	// "메시지를 넣은 직후"에 "바로 취소"하여,
	// runSender의 select문에서 case msg <- RequestC가 선택된 후 case <-ctx.Done()이 체크되기 전에
	// 내부 로직이 실행되는지 확인하는 방식보다는,
	//
	// 독립 컨텍스트가 적용되었다면, 외부 ctx가 취소되어도 내부 Send는 성공해야 한다는 점을 이용.

	// 약간의 텀을 주어 runSender가 메시지를 꺼내게 함 (매우 짧게)
	time.Sleep(1 * time.Millisecond)

	// 4. Shutdown Trigger (Context Cancel)
	cancel()

	// 5. 검증: Send가 호출되었는가?
	select {
	case <-sendCalled:
		// 성공: 컨텍스트 취소에도 불구하고 Send가 호출됨
	case <-time.After(1 * time.Second):
		t.Fatal("Shutdown 시 In-Flight 메시지가 전송되지 않고 유실되었습니다.")
	}

	// Sender 종료 대기 (Drain 로직 포함)
	// Sender가 종료될 수 있도록 Drain 큐가 비어있어야 하므로 (현재 1개 처리함)
	// BaseNotifier의 Done 채널 닫힘이 필요할 수 있음 (n.Close() 호출 필요)
	// 하지만 runSender는 ctx.Done()이 되면 Drain 모드로 진입하고, RequestC가 비면 종료됨.
	// 여기서 n.Close()는 호출하지 않았으므로 Drain 모드에서 n.Done() 대기를 할 수 있음.
	// runSender 코드를 보면 `<-n.Done()` 대기가 있음.
	n.Close() // Notifier 종료 신호

	wgSender.Wait()

	mockBot.AssertExpectations(t)
}
