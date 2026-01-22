package telegram

import (
	"context"
	"sync"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Run 텔레그램 봇의 메인 실행 루프를 시작합니다.
//
// 아키텍처 개요:
//
// 이 메서드는 Sender/Receiver 패턴을 사용하여 두 가지 핵심 작업을 병렬로 수행합니다:
//
//  1. Receiver (메인 고루틴):
//     - 텔레그램 서버로부터 봇 명령어를 Long Polling 방식으로 수신합니다
//     - 수신한 명령어를 별도 고루틴으로 디스패치하여 Non-blocking 처리합니다
//     - 세마포어로 동시 실행 수를 제한하여 과부하를 방지합니다 (Backpressure)
//
//  2. Sender (별도 고루틴):
//     - 내부 시스템으로부터 알림 발송 요청을 채널로 수신합니다
//     - 텔레그램 API를 호출하여 실제 메시지를 전송합니다
//     - Rate Limit, 재시도, HTML 파싱 오류 등을 처리합니다
//
// 설계 의도 (Decoupling):
//
//	Receiver와 Sender를 분리한 이유는 상호 간섭을 방지하기 위함입니다.
//	만약 알림 발송이 느려지거나 Rate Limit에 걸려도, 봇 명령어 수신에는
//	전혀 영향을 주지 않습니다. 각 작업의 생명주기를 독립적으로 관리할 수 있어
//	유지보수성과 안정성이 향상됩니다.
//
// 종료 처리 (Graceful Shutdown):
//
//   - serviceStopCtx 취소 또는 updateC 채널 닫힘 시 정상 종료됩니다
//   - defer cleanup()을 통해 모든 고루틴이 안전하게 종료될 때까지 대기합니다
//   - Sender는 종료 시 큐에 남은 메시지를 최대 60초간 처리합니다 (Drain)
//   - 타임아웃 안전장치로 좀비 고루틴 발생을 방지합니다
func (n *telegramNotifier) Run(serviceStopCtx context.Context) {
	// ─────────────────────────────────────────────────────────────────────────────
	// 1단계: 텔레그램 Long Polling 설정
	// ─────────────────────────────────────────────────────────────────────────────
	// Short Polling(짧은 주기로 반복 요청)과 달리, Long Polling은 서버에 연결을 열어두고
	// 새로운 메시지가 도착할 때까지 대기하는 방식입니다.
	//
	// 장점:
	//   - 네트워크 대역폭 절약: 불필요한 반복 요청을 하지 않습니다
	//   - 빠른 반응 속도: 메시지 도착 즉시 수신할 수 있습니다
	//   - 서버 부하 감소: API 호출 횟수가 크게 줄어듭니다
	config := tgbotapi.NewUpdate(0)
	config.Timeout = 60 // 60초 동안 대기 (Telegram API 권장값)

	// 업데이트 수신 채널 획득
	// 주의: GetUpdatesChan()은 내부적으로 별도 고루틴을 생성하여 지속적으로 업데이트를 가져옵니다.
	updateC := n.client.GetUpdatesChan(config)

	applog.WithComponentAndFields(component, applog.Fields{
		"notifier_id":  n.ID(),
		"bot_username": n.client.GetSelf().UserName,
		"chat_id":      n.chatID,
	}).Debug("텔레그램 봇 서비스 시작됨: Long Polling 활성화, Sender/Receiver 고루틴 실행 중")

	// ─────────────────────────────────────────────────────────────────────────────
	// 2단계: 고루틴 생명주기 관리 준비
	// ─────────────────────────────────────────────────────────────────────────────
	// WaitGroup을 사용하여 다음 고루틴들의 종료를 추적합니다:
	//   - Sender 고루틴 (알림 발송)
	//   - 명령어 처리 고루틴들 (봇 명령어 실행)
	//     → Receiver(receiveAndDispatchCommands) 내부에서 메시지 수신 시마다 동적 생성
	//
	// 이를 통해 cleanup() 함수에서 모든 고루틴이 안전하게 종료될 때까지 대기하여
	// Graceful Shutdown을 보장합니다 (리소스 누수 방지).
	var wg sync.WaitGroup

	// ─────────────────────────────────────────────────────────────────────────────
	// 3단계: Sender 고루틴 시작 (알림 발송 전담 워커)
	// ─────────────────────────────────────────────────────────────────────────────
	wg.Add(1)
	go func() {
		defer wg.Done()
		n.sendNotifications(serviceStopCtx)
	}()

	// ─────────────────────────────────────────────────────────────────────────────
	// 4단계: 종료 시 리소스 정리 예약 (defer)
	// ─────────────────────────────────────────────────────────────────────────────
	defer n.cleanup(&wg)

	// ─────────────────────────────────────────────────────────────────────────────
	// 5단계: 메인 루프 시작 (Receiver - 봇 명령어 수신 및 처리)
	// ─────────────────────────────────────────────────────────────────────────────
	n.receiveAndDispatchCommands(serviceStopCtx, updateC, &wg)
}

// cleanup Run 메서드 종료 시 모든 리소스를 안전하게 정리합니다.
//
// 이 함수는 defer로 호출되어 Run 함수가 종료될 때(정상/비정상 모두) 반드시 실행됩니다.
// Graceful Shutdown을 보장하기 위해 다음 순서를 엄격히 준수해야 합니다:
//
// 정리 순서 (중요: 순서 변경 시 리소스 누수나 좀비 고루틴 발생 가능):
//
//  1. 신규 메시지 수신 중단 (StopReceivingUpdates)
//     → 새로운 봇 명령어가 들어오지 않도록 Long Polling을 먼저 중단합니다
//
//  2. Notifier 내부 상태 종료 (Close)
//     → Sender 고루틴에게 종료 신호를 보내 Drain 프로세스를 시작합니다
//
//  3. 활성 고루틴 종료 대기 (waitForGoroutines)
//     → 모든 고루틴이 작업을 완료하고 종료될 때까지 대기합니다 (타임아웃 적용)
//
//  4. 리소스 해제 (client = nil)
//     → 모든 고루틴이 종료된 후 안전하게 클라이언트 참조를 제거합니다
func (n *telegramNotifier) cleanup(wg *sync.WaitGroup) {
	// ─────────────────────────────────────────────────────────────────────────────
	// A. 신규 메시지 수신 중단
	// ─────────────────────────────────────────────────────────────────────────────
	// 텔레그램 서버로부터 더 이상 새로운 업데이트를 받지 않도록 Long Polling을 중단합니다.
	// 이를 통해 새로운 봇 명령어가 들어오는 것을 차단하여 종료 프로세스를 시작합니다.
	if n.client != nil {
		n.client.StopReceivingUpdates()
	}

	// ─────────────────────────────────────────────────────────────────────────────
	// B. Notifier 내부 상태 종료
	// ─────────────────────────────────────────────────────────────────────────────
	// n.Done 채널을 닫아 Sender 고루틴에게 "종료해라"라는 신호를 보냅니다.
	// Sender는 이 신호를 받으면 큐에 남은 메시지를 처리(Drain)한 후 종료합니다.
	n.Close()

	// ─────────────────────────────────────────────────────────────────────────────
	// C. 활성 고루틴 종료 대기 (Graceful Shutdown)
	// ─────────────────────────────────────────────────────────────────────────────
	// Sender 고루틴과 모든 명령어 처리 고루틴이 안전하게 종료될 때까지 대기합니다.
	// 타임아웃(65초)을 적용하여 무한 대기를 방지하고 좀비 고루틴 발생을 막습니다.
	n.waitForGoroutines(wg)

	// ─────────────────────────────────────────────────────────────────────────────
	// D. 리소스 해제
	// ─────────────────────────────────────────────────────────────────────────────
	// 모든 고루틴이 종료된 후 텔레그램 클라이언트 참조를 제거하여 메모리 누수를 방지합니다.
	n.client = nil

	applog.WithComponentAndFields(component, applog.Fields{
		"notifier_id": n.ID(),
		"chat_id":     n.chatID,
	}).Debug("텔레그램 봇 서비스 종료됨: 모든 고루틴 정리 완료")
}

// waitForGoroutines 모든 활성 고루틴이 종료될 때까지 대기합니다.
//
// 이 함수는 cleanup 프로세스의 핵심으로, Sender 고루틴과 모든 명령어 처리 고루틴이
// 안전하게 작업을 완료하고 종료될 때까지 기다립니다.
//
// 타임아웃 전략:
//
//   - Drain 타임아웃(60초) + 여유분(5초) = 총 65초를 허용합니다
//   - Sender는 종료 시그널을 받으면 큐에 남은 메시지를 최대 60초간 처리합니다
//   - 5초 여유분은 명령어 처리 고루틴들이 정리될 시간을 제공합니다
//
// 타임아웃이 필요한 이유:
//
//   - 네트워크 문제나 버그로 인해 고루틴이 무한 대기할 수 있습니다
//   - 타임아웃 없이는 서비스 종료가 영원히 블로킹될 수 있습니다
//   - 운영 환경에서 빠른 재시작이 필요할 때 무한 대기는 치명적입니다
func (n *telegramNotifier) waitForGoroutines(wg *sync.WaitGroup) {
	// Sender 고루틴과 실행 중인 모든 명령어 처리 고루틴이 작업을 마칠 때까지 기다립니다.
	goroutinesDone := make(chan struct{})
	go func() {
		wg.Wait()             // 모든 고루틴이 wg.Done()을 호출할 때까지 대기
		close(goroutinesDone) // 완료 시그널 전송
	}()

	// Drain 타임아웃(60초) + 여유분(5초) = 총 65초
	// Sender의 Drain 프로세스가 최대 60초 소요될 수 있으므로, 충분한 시간을 제공합니다.
	shutdownWaitTimeout := shutdownTimeout + (5 * time.Second)
	select {
	case <-goroutinesDone:
		// ─────────────────────────────────────────────────────────────────────
		// Case A: 정상 종료 (모든 고루틴이 제한 시간 내에 종료됨)
		// ─────────────────────────────────────────────────────────────────────
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
		}).Debug("Graceful Shutdown 완료: 모든 고루틴 정상 종료됨")
	case <-time.After(shutdownWaitTimeout):
		// ─────────────────────────────────────────────────────────────────────
		// Case B: 타임아웃 발생 (좀비 고루틴 가능성)
		// ─────────────────────────────────────────────────────────────────────
		// 일부 고루틴이 아직 실행 중일 수 있지만, 무한 대기를 방지하기 위해
		// 서비스 종료를 강제로 진행합니다. 좀비 고루틴이 남을 수 있으나,
		// 프로세스 종료 시 OS가 정리합니다.
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"timeout":     shutdownWaitTimeout,
		}).Error("Graceful Shutdown 타임아웃: 일부 고루틴 강제 종료됨, 좀비 고루틴 발생 가능")
	}
}

// isClosed 텔레그램 Notifier가 현재 종료된 상태인지 확인합니다.
//
// 이 메서드는 notifier.Base의 Done() 채널을 Non-blocking Select 패턴으로 검사하여,
// 채널이 닫혔는지(종료 시그널 발생) 여부를 즉시 반환합니다.
func (n *telegramNotifier) isClosed() bool {
	select {
	case <-n.Done():
		return true
	default:
		return false
	}
}
