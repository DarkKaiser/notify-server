package telegram

import (
	"context"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// sendNotifications 내부 시스템으로부터 알림 발송 요청을 수신하여 텔레그램으로 전송하는 작업 루프입니다.
//
// 역할 (Sender):
//
// 이 메서드는 Run 함수에서 시작된 별도의 고루틴으로, Sender 역할을 수행합니다.
// NotificationC 채널로부터 알림 요청을 지속적으로 수신하여 텔레그램 API로 전송합니다.
// Run 메서드의 Receiver 루프와 독립적으로 동작하므로, 메시지 전송 지연이 발생해도
// 봇 명령어 수신에는 전혀 영향을 주지 않습니다 (Decoupling).
//
// 주요 기능:
//
//   - 알림 요청을 순차적으로 처리하여 텔레그램 API로 전송합니다
//   - 2단계 패닉 복구: 루프 레벨 + 개별 메시지 레벨에서 패닉을 복구합니다
//   - 서비스 종료 시 Drain 프로세스를 실행하여 큐에 남은 메시지를 처리합니다
//
// Graceful Shutdown (Drain):
//
//   - 종료 시그널 수신 후 drainRemainingNotifications()를 호출합니다
//   - 최대 60초간 NotificationC에 남아있는 모든 메시지를 최대한 발송합니다
//   - 타임아웃 초과 시 남은 메시지는 손실될 수 있습니다 (무한 대기 방지)
func (n *telegramNotifier) sendNotifications(serviceStopCtx context.Context) {
	// ═════════════════════════════════════════════════════════════════════════════
	// 1. 안전장치: 루프 레벨 패닉 복구
	// ═════════════════════════════════════════════════════════════════════════════
	// 이 고루틴은 텔레그램 알림 발송을 전담하는 핵심 Sender 워커입니다.
	// 로직 수행 중 예기치 않은 런타임 오류(Panic)가 발생하더라도,
	// 서비스 전체가 영향을 받지 않도록 로그를 남기고 Sender 고루틴만 안전하게 종료합니다.
	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"notifier_id": n.ID(),
				"chat_id":     n.chatID,
				"panic":       r,
			}).Error("발송 프로세스 비정상 종료: Sender 고루틴 패닉 발생 (서비스 재시작 필요)")

			// [중요] Sender 고루틴이 죽으면 Notifier 기능이 마비되므로,
			// 상태를 명시적으로 'Closed'로 변경하여 외부에서 이를 인지할 수 있게 해야 합니다.
			// 그렇지 않으면 외부(Service)에서는 정상으로 착각하여 메시지를 계속 보내고,
			// 큐가 가득 찰 때까지 메시지가 유실되는 'Silent Failure'가 발생합니다.
			n.Close()
		}
	}()

	for {
		// [테스트 전용 훅]
		// Sender 루프의 최상위 레벨에서 발생하는 패닉을 시뮬레이션하기 위한 코드입니다.
		// 실제 운영 환경에서는 절대 실행되지 않으며, 오직 `sender_worker_test.go`에서
		// 패닉 복구 및 리소스 정리(Close) 로직이 정상 동작하는지 검증하기 위해 사용됩니다.
		if n.testHookSenderPanic != nil {
			n.testHookSenderPanic()
		}

		select {
		// ═════════════════════════════════════════════════════════════════════
		// Case A: 알림 발송 요청 수신 (정상 처리 흐름)
		// ═════════════════════════════════════════════════════════════════════
		case req, ok := <-n.NotificationC():
			// ─────────────────────────────────────────────────────────────────
			// 채널 닫힘 감지
			// ─────────────────────────────────────────────────────────────────
			// 참고: notificationC는 다중 생산자(Multi-Producer) 환경에서 패닉 방지를 위해 절대 닫히지 않습니다.
			// (notifier.Base.Close() 참조)
			// 따라서 이 케이스는 발생하지 않으며, 방어 코드로만 존재합니다.
			if !ok {
				return // 채널이 닫히면 루프 종료
			}

			// ─────────────────────────────────────────────────────────────────
			// 개별 알림 처리 (패닉 격리)
			// ─────────────────────────────────────────────────────────────────
			// 익명 함수로 격리하여 실행하는 이유:
			//   1. defer를 사용하여 개별 처리 건마다 즉시 패닉 복구 수행
			//   2. 특정 알림 데이터의 문제로 인한 패닉이 워커 루프 전체를 중단시키는 것을 방지
			//   3. 한 메시지의 실패가 다른 메시지 처리에 영향을 주지 않도록 보장
			func() {
				defer func() {
					if r := recover(); r != nil {
						fields := applog.Fields{
							"notifier_id": n.ID(),
							"chat_id":     n.chatID,
							"panic":       r,
						}
						if req.Notification.TaskID != "" {
							fields["task_id"] = req.Notification.TaskID
						}
						if req.Notification.Title != "" {
							fields["task_title"] = req.Notification.Title
						}
						applog.WithComponentAndFields(component, fields).Error("메시지 처리 실패: 발송 로직 수행 중 패닉 발생 (해당 건 스킵)")
					}
				}()

				// 실제 텔레그램 API 호출을 통한 메시지 발송
				n.sendNotification(req.Ctx, &req.Notification)
			}()

		// ═════════════════════════════════════════════════════════════════════
		// Case B: 서비스 종료 시그널 감지
		// ═════════════════════════════════════════════════════════════════════
		// 애플리케이션이 종료되거나(SIGTERM), 상위 컨텍스트가 취소된 경우입니다.
		// 루프를 탈출하여 아래의 Drain(잔여 처리) 로직으로 이동합니다.
		case <-serviceStopCtx.Done():

		// ═════════════════════════════════════════════════════════════════════
		// Case C: Notifier 인스턴스 종료 감지
		// ═════════════════════════════════════════════════════════════════════
		// Close() 메서드가 호출되어 명시적으로 종료된 경우입니다.
		// Receiver(Notifier)가 닫히면 Sender(이 고루틴)도 정리되어야 합니다. (Zombie 방지)
		// 루프를 탈출하여 아래의 Drain(잔여 처리) 로직으로 이동합니다.
		case <-n.Done():
		}

		// ═════════════════════════════════════════════════════════════════════
		// 2. 종료 처리: 잔여 메시지 배출 (Drain) 및 Graceful Shutdown
		// ═════════════════════════════════════════════════════════════════════
		// 서비스가 종료되거나 Notifier가 닫힌 경우, 큐(Channel)에 남아있는 메시지들을 처리합니다.
		if serviceStopCtx.Err() != nil || n.isClosed() {
			n.drainRemainingNotifications()
			return
		}
	}
}

// drainRemainingNotifications Graceful Shutdown의 마지막 단계로, 큐에 남아있는 알림을 처리합니다.
//
// # 호출 시점 및 목적
//
// 이 함수는 sendNotifications의 종료 처리 단계에서 호출되며,
// serviceStopCtx가 취소된 후에도 NotificationC 채널에 남아있는 메시지들을
// 최대한 발송하여 메시지 손실을 최소화합니다.
//
// # 설계 전략
//
// Context 관리:
//   - serviceStopCtx는 이미 취소됨(Cancelled) 상태이므로 사용 불가
//   - 새로운 drainCtx(60초 타임아웃)를 생성하여 텔레그램 API 호출 가능하게 함
//   - 타임아웃 계산: 버퍼 1000개 / 초당 20개 처리 ≈ 50초 + 여유분 10초
//
// Non-blocking 전략:
//   - select-default 패턴으로 채널이 비어있으면 즉시 종료
//   - 채널 닫힘을 기다리지 않음 (notificationC는 절대 닫히지 않음)
//   - 타임아웃 초과 시 남은 메시지를 버리고 강제 종료
//
// 패닉 복구:
//   - 개별 메시지 처리 중 패닉이 발생해도 Drain 프로세스는 계속됩니다
//   - 패닉 발생 시 해당 메시지는 손실되지만 다른 메시지는 처리됩니다
//
// # 운영 고려사항
//
// 이 함수는 프로세스 종료를 지연시키므로, 운영 환경에서 빠른 재시작이 필요한 경우
// shutdownTimeout 값을 적절히 조정해야 합니다. 타임아웃이 너무 길면 배포 시간이 늘어나고,
// 너무 짧으면 메시지 손실이 증가합니다.
func (n *telegramNotifier) drainRemainingNotifications() {
	// ═════════════════════════════════════════════════════════════════════════════
	// 1. Drain 전용 컨텍스트 생성 (Time-bound)
	// ═════════════════════════════════════════════════════════════════════════════
	// serviceStopCtx는 이미 취소됨(Cancelled) 상태일 가능성이 높습니다.
	// 따라서 큐에 남은 메시지를 발송하기 위해서는 "살아있는" 새로운 컨텍스트가 필요합니다.
	//
	// 타임아웃 설정 이유:
	//   - 프로세스가 무한정 종료되지 않는 것을 방지
	//   - shutdownTimeout(60초): 버퍼 1000개 / 초당 20개 처리 ≈ 50초 + 여유분
	drainCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel() // 함수 종료 시 리소스 정리

	// ═════════════════════════════════════════════════════════════════════════════
	// 2. 경쟁 상태 방지: Sender 진입 대기 (Wait for Pending Sends)
	// ═════════════════════════════════════════════════════════════════════════════
	// 종료 직전에 Send() 메서드에 진입하여 채널에 넣으려는 시도들이 완료될 때까지 기다립니다.
	// 이를 통해 "채널 확인(Empty) -> 종료 -> Send(Push)" 순서로 발생하는 데이터 유실을 방지합니다.
	// WaitGroup.Wait()는 블로킹되므로, 타임아웃(waitPendingSendsCtx)을 적용하여 무한 대기를 방지합니다.
	waitPendingSendsC := make(chan struct{})
	go func() {
		n.Base.WaitForPendingSends()
		close(waitPendingSendsC)
	}()

	// Pending Sends 대기용으로 짧은 타임아웃(예: 6초)을 별도로 설정합니다.
	waitPendingSendsCtx, waitPendingSendsCancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer waitPendingSendsCancel()

	select {
	case <-waitPendingSendsC:
		// 대기 완료: 모든 Sender가 작업을 마침 (채널에 넣었거나 포기했거나)

	case <-waitPendingSendsCtx.Done():
		// 대기 타임아웃: Pending Sends가 너무 오래 걸림 -> 포기하고 Drain으로 넘어감
		applog.WithComponentAndFields(component, applog.Fields{
			"notifier_id": n.ID(),
			"chat_id":     n.chatID,
			"timeout":     6 * time.Second,
			"queue_depth": len(n.Base.NotificationC()),
		}).Warn("Pending Sends 대기 중단: 대기 제한 시간 초과 (잔여 시간 동안 전송 시도)")
	}

	// ═════════════════════════════════════════════════════════════════════════════
	// 3. Non-blocking Drain Loop
	// ═════════════════════════════════════════════════════════════════════════════
	// select-default 패턴을 사용하여:
	//   - 채널에 메시지가 있으면 → 처리
	//   - 채널이 비어있으면 → 즉시 종료 (블로킹 없음)
	//
	// 참고: notificationC는 절대 닫히지 않으므로 채널 닫힘을 기다리지 않습니다.
Loop:
	for {
		select {
		// ─────────────────────────────────────────────────────────────────────
		// Case A: 잔여 메시지 수신
		// ─────────────────────────────────────────────────────────────────────
		case req := <-n.NotificationC():
			// ─────────────────────────────────────────────────────────────────
			// 타임아웃 체크
			// ─────────────────────────────────────────────────────────────────
			// Drain 프로세스가 너무 오래 걸리면 강제 종료합니다.
			// 운영 환경에서 빠른 재시작이 필요할 때 무한 대기는 치명적입니다.
			if drainCtx.Err() != nil {
				applog.WithComponentAndFields(component, applog.Fields{
					"notifier_id":         n.ID(),
					"chat_id":             n.chatID,
					"timeout":             shutdownTimeout,
					"remaining_in_buffer": len(n.NotificationC()),
				}).Warn("잔여 메시지 폐기: 종료 대기 시간(Drain Timeout) 초과")

				break Loop // 타임아웃 시 남은 메시지를 버리고 종료
			}

			// ─────────────────────────────────────────────────────────────────
			// 개별 알림 최대한 발송 (패닉 격리)
			// ─────────────────────────────────────────────────────────────────
			// 익명 함수로 격리하여 개별 알림의 패닉이 전체 Drain 프로세스를 중단시키지 않도록 합니다.
			func() {
				defer func() {
					if r := recover(); r != nil {
						fields := applog.Fields{
							"notifier_id": n.ID(),
							"chat_id":     n.chatID,
							"panic":       r,
						}
						if req.Notification.TaskID != "" {
							fields["task_id"] = req.Notification.TaskID
						}
						if req.Notification.Title != "" {
							fields["task_title"] = req.Notification.Title
						}
						applog.WithComponentAndFields(component, fields).Error("잔여 메시지 처리 실패: Drain 로직 수행 중 패닉 발생 (해당 건 스킵)")
					}
				}()

				// 중요: serviceStopCtx(취소됨)가 아닌 drainCtx(유효함)를 사용해야 합니다.
				// 그렇게 해야 텔레그램 API 호출이 정상적으로 수행됩니다.
				n.sendNotification(drainCtx, &req.Notification)
			}()

		// ─────────────────────────────────────────────────────────────────────
		// Case B: 채널 비어있음 (Drain 완료)
		// ─────────────────────────────────────────────────────────────────────
		default:
			// 채널에 더 이상 메시지가 없으므로 Drain 프로세스를 종료합니다.
			break Loop
		}
	}
}
