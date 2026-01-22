package telegram

import (
	"context"
	"sync"

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
				}).Error("Long Polling 채널 종료됨: 명령어 수신 루프 종료 및 cleanup 시작")

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
				}).Warn("명령어 처리 용량 초과로 요청 드롭됨: 빈번 발생 시 세마포어 용량 증가 검토 필요")
			}

		// ═════════════════════════════════════════════════════════════════════
		// Case B: 서비스 종료 신호 감지
		// ═════════════════════════════════════════════════════════════════════
		case <-serviceStopCtx.Done():
			// 서비스 종료 요청 → 루프 탈출 → defer cleanup() 실행
			return
		}
	}
}
