package telegram

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// receiveAndDispatchCommands 텔레그램 서버로부터 봇 명령어를 수신하고 처리합니다.
//
// 역할 (Receiver):
//
// 이 함수는 Run 함수의 메인 루프로, Receiver 역할을 수행합니다.
// Long Polling 방식으로 텔레그램 서버로부터 지속적으로 업데이트를 수신하고,
// 유효성 검사를 거쳐 명령어 처리 고루틴으로 디스패치합니다.
//
// 처리 파이프라인:
//
//  1. 채널 닫힘 감지 → cleanup 시작
//  2. 유효성 검사 → 텍스트 메시지만 처리
//  3. 보안 검사 → 허용된 chatID만 처리
//  4. 세마포어 기반 디스패치 → Non-blocking 실행
//
// Concurrency Control (세마포어 패턴):
//
//   - commandSemaphore로 동시 실행 수를 제한하여 시스템 리소스 고갈을 방지합니다
//   - 세마포어가 꽉 차면 Backpressure를 활성화하여 요청을 Drop 합니다
//   - Drop 된 요청은 경고 로그로 기록되며, 사용자는 나중에 재시도할 수 있습니다
//
// 종료 처리:
//
//   - serviceStopCtx 취소 또는 updateC 채널 닫힘 시 루프를 탈출합니다
//   - 루프 탈출 후 defer cleanup()이 실행되어 Graceful Shutdown을 수행합니다
func (n *telegramNotifier) receiveAndDispatchCommands(serviceStopCtx context.Context, updateC tgbotapi.UpdatesChannel, wg *sync.WaitGroup) {
	for {
		select {
		// ═════════════════════════════════════════════════════════════════════
		// Case A: 텔레그램 서버로부터 새로운 업데이트(메시지) 도착
		// ═════════════════════════════════════════════════════════════════════
		case update, ok := <-updateC:
			// ─────────────────────────────────────────────────────────────────
			// 1. 채널 닫힘 감지
			// ─────────────────────────────────────────────────────────────────
			// updateC 채널이 닫히면 더 이상 메시지를 받을 수 없으므로 루프를 종료합니다.
			// 이는 텔레그램 클라이언트가 StopReceivingUpdates()를 호출했을 때 발생합니다.
			if !ok {
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id": n.ID(),
					"chat_id":     n.chatID,
				}).Error("수신 채널 종료: Long Polling 루프 중단 (Cleanup 수행)")

				return
			}

			// ─────────────────────────────────────────────────────────────────
			// 2. 유효성 검사: 텍스트 메시지만 처리
			// ─────────────────────────────────────────────────────────────────
			// 사진, 스티커, 파일 등 다른 타입의 업데이트는 무시합니다.
			// 봇 명령어는 텍스트 메시지로만 전송되기 때문입니다.
			if update.Message == nil {
				continue
			}

			// ─────────────────────────────────────────────────────────────────
			// 3. 보안 검사: 허용된 채팅방에서 온 메시지만 처리
			// ─────────────────────────────────────────────────────────────────
			// 설정된 chatID와 일치하는 채팅방의 메시지만 처리하여
			// 무단 접근이나 스팸 메시지를 차단합니다.
			if update.Message.Chat.ID != n.chatID {
				continue
			}

			// ─────────────────────────────────────────────────────────────────
			// 4. 명령어 처리 디스패치 (Non-blocking + Concurrency Control)
			// ─────────────────────────────────────────────────────────────────
			// 명령어 실행(DB 조회, 비즈니스 로직 등)이 오래 걸릴 수 있으므로
			// 별도 고루틴으로 실행하여 수신 루프를 차단하지 않습니다.
			//
			// 세마포어 패턴:
			//   - commandSemaphore에 값을 보낼 수 있으면 → 실행 권한 획득
			//   - 세마포어가 꽉 차있으면 → default 케이스로 이동 (요청 Drop)
			select {
			case n.commandSemaphore <- struct{}{}:
				// ─────────────────────────────────────────────────────────
				// 실행 권한 획득 성공 → 명령어 처리 고루틴 시작
				// ─────────────────────────────────────────────────────────
				wg.Add(1)
				go func(message *tgbotapi.Message) {
					defer wg.Done()                         // 처리 완료 시 WaitGroup 감소
					defer func() { <-n.commandSemaphore }() // 처리 완료 시 세마포어 슬롯 반납
					n.dispatchCommand(serviceStopCtx, message)
				}(update.Message)

			case <-serviceStopCtx.Done():
				// ─────────────────────────────────────────────────────────
				// 세마포어 대기 중 서비스 종료 신호 발생 → 즉시 종료
				// ─────────────────────────────────────────────────────────
				return

			default:
				// ─────────────────────────────────────────────────────────
				// Backpressure: 과부하 보호
				// ─────────────────────────────────────────────────────────
				// 세마포어가 꽉 찼다면 동시 실행 중인 명령어가 최대치에 도달한 상태입니다.
				// 시스템 보호를 위해 새로운 요청을 드롭(Drop)하고 경고 로그를 남깁니다.
				// 사용자는 나중에 다시 명령어를 입력할 수 있습니다.
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id":        n.ID(),
					"chat_id":            n.chatID,
					"semaphore_capacity": cap(n.commandSemaphore), // 최대 동시 명령어 처리 수
					"active_commands":    len(n.commandSemaphore), // 현재 실행 중인 명령어 수
				}).Warn("요청 처리 거부: 동시 처리 용량 초과로 인한 드롭 (Backpressure)")

				// 사용자에게 시스템 혼잡 안내 메시지 전송
				// 요청이 거부되었음을 명확히 알려주어 사용자가 상황을 이해하고 잠시 후 다시 시도하게 합니다.
				busyMessage := "현재 시스템 이용자가 많아 요청을 처리할 수 없습니다.\n\n잠시 후 다시 시도해 주세요."
				if err := n.TrySend(serviceStopCtx, contract.NewNotification(busyMessage)); err != nil {
					// 안내 메시지 전송조차 실패한 경우(예: 발송 큐 포화), 더 이상 할 수 있는 조치가 없으므로 로그만 남깁니다.
					applog.WithComponentAndFields(component, applog.Fields{
						"notifier_id": n.ID(),
						"error":       err,
					}).Error("시스템 혼잡 안내 발송 실패: 대기열 용량 초과")
				}
			}

		// ═════════════════════════════════════════════════════════════════════
		// Case B: 서비스 종료 신호 감지
		// ═════════════════════════════════════════════════════════════════════
		case <-serviceStopCtx.Done():
			// 서비스 종료 요청 → 루프 탈출 → defer cleanup() 실행
			return

		// ═════════════════════════════════════════════════════════════════════
		// Case C: Notifier 종료 감지 (Zombie Receiver 방지)
		// ═════════════════════════════════════════════════════════════════════
		// Sender 고루틴이 패닉으로 비정상 종료되어 n.Close()가 호출된 경우,
		// 이 Notifier는 더 이상 메시지를 발송할 수 없는 상태가 됩니다.
		//
		// 문제 시나리오 (Receiver를 종료하지 않는 경우):
		//   1. Sender 고루틴이 패닉으로 사망 → n.Close() 호출됨
		//   2. 하지만 Receiver는 여전히 살아있어 명령어를 계속 수신함
		//   3. 사용자가 봇 명령어를 입력하면 정상적으로 처리되는 것처럼 보임
		//   4. 하지만 응답 메시지는 Send()에서 에러가 발생하여 전송되지 않음
		//   5. 사용자는 명령어가 무시되는지, 처리 중인지 알 수 없음 (Silent Failure)
		//
		// 해결 방법:
		//   n.Done() 채널을 감지하여 Sender가 죽으면 Receiver도 즉시 종료합니다.
		//   이를 통해 더 이상 명령어를 받지 않으며, 시스템 재시작이 필요함을 명확히 합니다.
		case <-n.Done():
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id":     n.ID(),
				"chat_id":         n.chatID,
				"active_commands": len(n.commandSemaphore), // 현재 실행 중인 명령어 수 (영향도)
				"queued_updates":  len(updateC),            // 처리되지 못한 대기 메시지 수 (손실)
			}).Error("Receiver 비상 종료: Notifier 종료 신호 감지 (Sender 사망 등)")

			return
		}
	}
}
