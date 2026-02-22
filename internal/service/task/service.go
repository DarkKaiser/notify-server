package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// component Task 서비스의 로깅용 컴포넌트 이름
const component = "task.service"

const (
	// defaultQueueSize 이벤트 채널(Submit, Cancel, Done)의 기본 버퍼 크기입니다.
	// 일시적인 요청 급증 시 이벤트 루프가 처리하기 전까지 요청을 버퍼에 보관하여 블로킹을 줄입니다.
	defaultQueueSize = 10
)

// Service Task 서비스입니다. Task의 제출, 취소, 완료, 스케줄링을 총괄합니다.
//
// 모든 상태 변경 이벤트(Submit, Cancel, Done)는 채널을 통해 단일 이벤트 루프로 직렬화됩니다.
// 이로써 복잡한 Mutex 없이 안전한 동시성을 보장합니다.
//
// 주요 책임:
//   - 중복 실행 방지 및 Fail Fast 검증
//   - Cron 스케줄에 따른 자동 실행
//   - 서비스 종료 시 실행 중인 모든 Task의 안전한 정리 (Graceful Shutdown)
type Service struct {
	appConfig *config.AppConfig

	// tasks 현재 활성화(Running) 상태인 모든 Task 인스턴스를 보관하는 인메모리 저장소입니다.
	tasks map[contract.TaskInstanceID]provider.Task

	// idGenerator 각 Task 실행 인스턴스에 부여할 전역 고유 식별자(InstanceID)를 발급합니다.
	idGenerator contract.IDGenerator

	// taskSubmitC Submit()으로 들어온 새로운 Task 실행 요청을 이벤트 루프에 전달하는 채널입니다.
	taskSubmitC chan *contract.TaskSubmitRequest

	// taskDoneC Task 고루틴이 실행을 마쳤을 때 완료 신호를 이벤트 루프에 전달하는 채널입니다.
	taskDoneC chan contract.TaskInstanceID

	// taskCancelC Cancel()로 들어온 Task 취소 요청을 이벤트 루프에 전달하는 채널입니다.
	taskCancelC chan contract.TaskInstanceID

	// taskStopWG 실행 중인 모든 Task 고루틴의 종료를 추적하며, 종료 시의 정리 작업(handleStop)에서 대기하는 데 사용합니다.
	taskStopWG sync.WaitGroup

	// notificationSender Task의 실행 결과나 에러를 외부 메신저(예: Telegram)로 전송하는 인터페이스입니다.
	notificationSender contract.NotificationSender

	// taskResultStore Task가 스크래핑한 결과물(이전 스냅샷 등)을 영속적으로 저장하고 조회하는 저장소입니다.
	taskResultStore contract.TaskResultStore

	// fetcher 모든 Task가 공유하는 HTTP 클라이언트입니다.
	fetcher fetcher.Fetcher

	running   bool
	runningMu sync.Mutex
}

// NewService Task 서비스를 생성합니다.
func NewService(appConfig *config.AppConfig, idGenerator contract.IDGenerator, taskResultStore contract.TaskResultStore) *Service {
	if idGenerator == nil {
		panic("IDGenerator는 필수입니다")
	}

	return &Service{
		appConfig: appConfig,

		tasks: make(map[contract.TaskInstanceID]provider.Task),

		idGenerator: idGenerator,

		taskSubmitC: make(chan *contract.TaskSubmitRequest, defaultQueueSize),
		taskDoneC:   make(chan contract.TaskInstanceID, defaultQueueSize),
		taskCancelC: make(chan contract.TaskInstanceID, defaultQueueSize),

		notificationSender: nil,

		taskResultStore: taskResultStore,

		// 모든 Task가 공유하는 HTTP 클라이언트(Fetcher)를 초기화합니다.
		fetcher: fetcher.New(appConfig.HTTPRetry.MaxRetries, appConfig.HTTPRetry.RetryDelay, 0),

		running:   false,
		runningMu: sync.Mutex{},
	}
}

// SetNotificationSender Task 실행 결과 및 중요 이벤트를 외부로 전달할 NotificationSender를 주입합니다.
//
// Task 서비스는 순환 의존성 문제로 인해 생성자(NewService)에서 NotificationSender를 받지 않으므로,
// Start() 호출 전에 이 메서드를 통해 별도로 주입해야 합니다.
// Start() 내부에서 초기화 여부를 검증하므로, 주입 없이 Start()를 호출하면 오류가 반환됩니다.
//
// 매개변수:
//   - notificationSender: 알림을 전송할 구현체입니다. nil을 전달하면 Start() 시 오류가 반환됩니다.
func (s *Service) SetNotificationSender(notificationSender contract.NotificationSender) {
	s.notificationSender = notificationSender
}

// Start Task 서비스를 시작하고 이벤트 루프를 준비합니다.
//
// 내부적으로 runEventLoop()를 별도의 고루틴으로 실행하여 Task의 제출, 완료, 취소 이벤트를 처리합니다.
// 서비스가 이미 실행 중인 경우에는 경고 로그만 남기고 정상 반환합니다.
//
// 매개변수:
//   - serviceStopCtx: 서비스 종료 신호를 전달받는 컨텍스트입니다.
//     이 컨텍스트가 취소되면 이벤트 루프가 Graceful Shutdown을 시작합니다.
//   - serviceStopWG: 서비스 종료 시 이벤트 루프 고루틴이 완전히 종료될 때까지
//     대기하기 위한 WaitGroup입니다.
//
// 반환값:
//   - error: NotificationSender가 주입되지 않았거나 그 외 초기화 실패 시 오류를 반환합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent(component).Info("서비스 시작 진입: Task 서비스 초기화 프로세스를 시작합니다")

	if s.notificationSender == nil {
		defer serviceStopWG.Done()
		return ErrNotificationSenderNotInitialized
	}

	if s.running {
		defer serviceStopWG.Done()
		applog.WithComponent(component).Warn("Task 서비스가 이미 실행 중입니다 (중복 호출)")
		return nil
	}

	s.running = true

	go s.runEventLoop(serviceStopCtx, serviceStopWG)

	applog.WithComponent(component).Info("서비스 시작 완료: Task 서비스가 정상적으로 초기화되었습니다")

	return nil
}

// runEventLoop 서비스의 메인 이벤트 루프입니다.
//
// 단일 고루틴 안에서 아래 이벤트를 채널로 전달받아 순차적으로 처리하며, 복잡한 뮤텍스 없이 안전한 동시성을 유지합니다:
//
//   - taskSubmitC: 새로운 Task 실행 요청 수신 → handleTaskSubmit()
//   - taskDoneC:   Task 실행 완료 신호 수신 → handleTaskDone()
//   - taskCancelC: Task 취소 요청 수신 → handleTaskCancel()
//   - serviceStopCtx.Done(): 서비스 종료 신호 수신 → handleStop()
//
// 예기치 않은 패닉으로 인해 이 루프가 종료될 경우 서비스 전체가 마비되므로,
// 패닉 발생 시 복구(recover)하여 에러 로그를 남깁니다.
//
// Note: 이 함수는 블로킹되며, Start()에서 별도의 고루틴으로 실행됩니다.
func (s *Service) runEventLoop(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	// =====================================================================
	// [패닉 보호 전략]
	// 이벤트 루프(for + select)가 예기치 않은 패닉으로 완전히 종료되면,
	// 서비스는 가동 중인 것처럼 보이지만 실제로는 새로운 이벤트를 처리할 수 없는 "좀비 상태"에 빠집니다.
	//
	// 이를 방지하기 위해, select 블록을 익명 함수로 한 단계 감싸고 그 내부에서 recover()로 패닉을 잡습니다.
	// 패닉이 발생하더라도 해당 회차의 익명 함수만 종료될 뿐, 외부 for 루프는 살아있으므로 다음 이벤트 처리를 정상적으로 재개합니다.
	//
	// 참고: 정상/비정상 종료 여부는 익명 함수의 반환값(shouldStop)으로 제어합니다.
	// =====================================================================
loop:
	for {
		// shouldStop = true  → break loop (정상 또는 채널 닫힘으로 인한 종료)
		// shouldStop = false → 다음 이벤트 처리를 위해 루프 재개
		shouldStop := func() bool {
			defer func() {
				if r := recover(); r != nil {
					applog.WithComponentAndFields(component, applog.Fields{
						"panic":              r,
						"task_running_count": len(s.tasks),       // 당시 실행 중이던 Task 수
						"task_queue_len":     len(s.taskSubmitC), // 대기 중이던 실행(Submit) 요청 수
						"done_queue_len":     len(s.taskDoneC),   // 큐에 쌓인 완료(Done) 신호 수
						"cancel_queue_len":   len(s.taskCancelC), // 큐에 쌓인 취소(Cancel) 요청 수
					}).Error("Task 서비스 이벤트 루프 치명적 오류 복구: 예기치 않은 패닉 상태에서 회복되어 이벤트 프로세싱을 인계 및 재개합니다. (즉각적인 시스템 안정성 점검이 요구됩니다)")
				}
			}()

			select {
			case req, ok := <-s.taskSubmitC:
				// handleStop()이 taskSubmitC를 닫으면 ok=false가 됩니다.
				// 이 시점에 서비스는 이미 종료 처리를 마쳤으므로 루프를 종료합니다.
				if !ok {
					return true // break loop
				}

				// 새로운 Task 실행 요청 수신 시, Task 고루틴을 생성하고 시작합니다.
				s.handleTaskSubmit(serviceStopCtx, req)

			case instanceID := <-s.taskDoneC:
				// Task 고루틴이 모든 작업을 마치면 이 채널을 통해 자신의 InstanceID를 보고합니다.
				// 현재 관리 중인 활성화된 Task 목록(s.tasks)에서 해당 작업을 제외시켜 리소스를 정리합니다.
				s.handleTaskDone(instanceID)

			case instanceID := <-s.taskCancelC:
				// 외부(Cancel 메서드)에서 취소 요청이 들어오면 해당 Task에 취소 신호를 보냅니다.
				s.handleTaskCancel(serviceStopCtx, instanceID)

			case <-serviceStopCtx.Done():
				// 시스템 종료 신호 수신 시, 실행 중인 모든 Task를 안전하게 정리하고 루프를 종료합니다.
				s.handleStop()

				return true // break loop
			}

			return false // 루프 재개
		}()

		if shouldStop {
			break loop
		}
	}
}

// handleTaskSubmit 새로운 Task 실행 요청을 처리합니다.
//
// 요청 처리는 아래 순서로 진행됩니다:
//  1. Task 설정 조회: 전달받은 TaskID/CommandID에 대한 설정을 찾아 유효성을 검증합니다.
//     설정이 없으면 즉시 사용자에게 오류 알림을 전송하고 종료합니다.
//  2. 동시성 제한 확인: AllowMultiple=false인 경우, 동일한 Task가 이미 실행 중이면 요청을 거부합니다.
//  3. Task 생성 및 시작: 검증을 통과하면 새로운 Task 인스턴스를 생성하고 고루틴으로 실행합니다.
func (s *Service) handleTaskSubmit(serviceStopCtx context.Context, req *contract.TaskSubmitRequest) {
	applog.WithComponentAndFields(component, applog.Fields{
		"task_id":     req.TaskID,
		"command_id":  req.CommandID,
		"run_by":      req.RunBy,
		"notifier_id": req.NotifierID,
	}).Debug("Task 요청 수신: 설정 조회 및 유효성 검증 시작")

	cfg, err := provider.FindConfig(req.TaskID, req.CommandID)
	if err != nil {
		applog.WithComponentAndFields(component, applog.Fields{
			"task_id":     req.TaskID,
			"command_id":  req.CommandID,
			"run_by":      req.RunBy,
			"notifier_id": req.NotifierID,
			"error":       err,
		}).Error(provider.ErrTaskNotSupported.Error())

		// 해당 Task에 대한 설정을 찾을 수 없으므로, 지원하지 않는 작업임을 사용자에게 비동기로 알립니다.
		go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
			NotifierID:    req.NotifierID,
			TaskID:        req.TaskID,
			CommandID:     req.CommandID,
			InstanceID:    "",
			Message:       provider.ErrTaskNotSupported.Error(),
			Elapsed:       0,
			ErrorOccurred: true,
			Cancelable:    false,
		})

		return
	}

	// AllowMultiple이 false인 경우, 동일한 Task(Command)가 이미 실행 중이면 요청을 거부합니다.
	if !cfg.Command.AllowMultiple {
		if s.rejectIfAlreadyRunning(serviceStopCtx, req) {
			return
		}
	}

	// 새로운 Task 인스턴스를 생성하여 활성화된 Task 목록에 등록하고 실행합니다.
	s.registerAndRunTask(serviceStopCtx, req, cfg)
}

// handleTaskDone Task 고루틴이 실행을 마쳤을 때 호출되어 사후 정리를 처리합니다.
//
// 활성화된 Task 목록(s.tasks)에서 해당 instanceID를 조회하여:
//   - Task가 존재하는 경우: 완료 로그를 기록하고 목록에서 인스턴스를 제거합니다.
//   - Task가 존재하지 않는 경우: 비정상적인 완료 신호로 판단하여 경고 로그를 기록합니다.
func (s *Service) handleTaskDone(instanceID contract.TaskInstanceID) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if task, exists := s.tasks[instanceID]; exists {
		applog.WithComponentAndFields(component, applog.Fields{
			"task_id":     task.ID(),
			"command_id":  task.CommandID(),
			"instance_id": instanceID,
			"notifier_id": task.NotifierID(),
			"elapsed":     task.Elapsed(),
		}).Debug("Task 완료 성공: 작업 정상 종료")

		delete(s.tasks, instanceID)
	} else {
		applog.WithComponentAndFields(component, applog.Fields{
			"instance_id":        instanceID,
			"task_running_count": len(s.tasks),
			"reason":             "not_found",
		}).Warn("Task 완료 처리 무시: 등록되지 않은 Instance ID 수신")
	}
}

// handleTaskCancel 특정 Task 인스턴스의 실행을 취소하고 사용자에게 결과를 알립니다.
//
// 활성화된 Task 목록(s.tasks)에서 해당 instanceID를 조회하여:
//   - Task가 존재하는 경우: 실행 중인 Task를 즉시 취소하고, 알림을 발송합니다.
//   - Task가 존재하지 않는 경우: 등록되지 않은 ID에 대한 취소 요청이므로 실패 알림을 발송합니다.
func (s *Service) handleTaskCancel(serviceStopCtx context.Context, instanceID contract.TaskInstanceID) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if task, exists := s.tasks[instanceID]; exists {
		// 해당 Task에 취소 신호를 보내 작업을 취소합니다.
		task.Cancel()

		applog.WithComponentAndFields(component, applog.Fields{
			"task_id":     task.ID(),
			"command_id":  task.CommandID(),
			"instance_id": instanceID,
			"notifier_id": task.NotifierID(),
			"elapsed":     task.Elapsed(),
		}).Debug("Task 취소 성공: 사용자 요청")

		// 취소가 완료되었음을 사용자에게 비동기로 알립니다.
		// 알림 발송 자체가 이벤트 루프를 블로킹하지 않도록 고루틴으로 처리합니다.
		go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
			NotifierID:    task.NotifierID(),
			TaskID:        task.ID(),
			CommandID:     task.CommandID(),
			InstanceID:    instanceID,
			Message:       "사용자 요청에 의해 작업이 취소되었습니다.",
			Elapsed:       task.Elapsed(),
			ErrorOccurred: false,
			Cancelable:    false,
		})
	} else {
		// 해당 Instance ID에 대응하는 Task를 찾지 못했습니다.
		// 이미 작업이 완료된 후 취소 요청이 들어왔거나, 잘못된 ID가 전달된 경우입니다.
		applog.WithComponentAndFields(component, applog.Fields{
			"instance_id":        instanceID,
			"task_running_count": len(s.tasks),
			"reason":             "not_found",
		}).Warn("Task 취소 실패: 등록되지 않은 Instance ID 수신")

		// 사용자에게 전달할 취소 실패 안내 메시지를 생성하고, 비동기로 알림을 전송합니다.
		message := fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)", instanceID)

		go s.notificationSender.Notify(serviceStopCtx, contract.NewErrorNotification(message))
	}
}

// handleStop 실행 중인 모든 Task를 안전하게 종료하고 서비스 리소스를 정리합니다.
//
// 종료는 아래 순서로 진행됩니다:
//  1. running = false 설정 및 모든 활성화된 Task에 취소 신호 전송
//  2. 입력 채널(taskSubmitC, taskCancelC) 닫기
//  3. 모든 Task 고루틴 종료 대기 (최대 30초)
//  4. taskDoneC 닫기 및 내부 상태 초기화
//
// 채널을 닫는 순서가 매우 중요합니다. 순서를 바꾸면 패닉이 발생할 수 있습니다.
// 자세한 이유는 각 단계의 주석을 참고하세요.
func (s *Service) handleStop() {
	applog.WithComponent(component).Info("종료 절차 진입: Task 서비스 중지 시그널을 수신했습니다")

	// =====================================================================
	// [단계 1] 외부 요청 수신 차단 및 활성화된 Task 취소
	// =====================================================================
	s.runningMu.Lock()

	// running = false를 먼저 설정하는 이유:
	// Submit()/Cancel() 메서드는 running 플래그를 확인한 후 채널에 전송합니다.
	// 만약 running = false 설정 없이 채널을 먼저 닫으면, 다른 고루틴이 닫힌 채널에
	// 전송을 시도해 패닉이 발생할 수 있습니다. 뮤텍스를 통해 이 순서를 보장합니다.
	s.running = false

	// 현재 실행 중인 모든 Task에 취소 신호를 보냅니다.
	// 각 Task는 신호를 받은 후 자신의 작업을 스스로 정리하고 종료합니다.
	for _, task := range s.tasks {
		task.Cancel()
	}

	s.runningMu.Unlock()

	// =====================================================================
	// [단계 2] 입력 채널 닫기
	// =====================================================================
	// 채널을 닫으면 이벤트 루프가 더 이상 외부 요청(Submit, Cancel)을 받지 않습니다.
	// 단계 1에서 running = false를 먼저 설정했으므로, 이 시점 이후에 Submit()/Cancel()이
	// 호출되더라도 채널 전송 전에 early return하여 패닉이 발생하지 않습니다.
	close(s.taskSubmitC)
	close(s.taskCancelC)

	// =====================================================================
	// [단계 3] 모든 Task 고루틴 종료 대기
	// =====================================================================

	// 각 Task 고루틴은 종료 시 taskDoneC에 InstanceID를 전송합니다.
	// taskDoneC의 버퍼가 가득 차면 Task 고루틴이 블로킹되므로,
	// 별도의 고루틴에서 taskDoneC를 지속적으로 비워 고루틴들이 막히지 않도록 합니다.
	// (taskDoneC는 아래 단계 4에서 taskStopWG.Wait() 완료 후에 닫힙니다)
	go func() {
		for range s.taskDoneC {
			// 종료 중이므로 완료 메시지는 별도 처리 없이 폐기합니다.
		}
	}()

	// 모든 Task가 종료될 때까지 대기합니다.
	// 별도의 고루틴에서 Wait()를 수행하고 done 채널로 알리는 방식을 사용하여,
	// 아래 select에서 타임아웃과 함께 대기할 수 있도록 합니다.
	done := make(chan struct{})
	go func() {
		s.taskStopWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 모든 Task가 정상적으로 종료되었습니다.

	case <-time.After(30 * time.Second):
		// 일부 Task가 종료되지 않아 타임아웃에 도달했습니다.
		// 더 이상 대기하지 않고 강제로 다음 단계(리소스 정리)를 진행합니다.
		applog.WithComponent(component).Warn("Task 서비스 강제 종료: 고루틴 종료 대기 시간 초과 (30s)")
	}

	// =====================================================================
	// [단계 4] taskDoneC 닫기 및 리소스 정리
	// =====================================================================

	// taskDoneC는 반드시 taskStopWG.Wait()가 완료된 이후에 닫아야 합니다.
	// 이유: Wait() 완료 전까지는 Task 고루틴들이 아직 살아있을 수 있으며,
	// 이들이 종료되며 taskDoneC에 전송을 시도할 수 있습니다.
	// 미리 닫아버리면 "send on closed channel" 패닉이 발생합니다.
	close(s.taskDoneC)

	// 서비스 내부 상태를 초기화하여 GC가 관련 리소스를 회수할 수 있도록 합니다.
	s.runningMu.Lock()
	s.tasks = nil
	s.notificationSender = nil
	s.runningMu.Unlock()

	applog.WithComponent(component).Info("Task 서비스 종료 완료: 모든 리소스가 정리되었습니다")
}

// rejectIfAlreadyRunning 동일한 Task가 이미 실행 중인지 확인하고, 중복 실행을 방지합니다.
//
// 현재 실행 중인 Task 목록을 순회하여, 동일한 TaskID와 CommandID를 가진 Task가
// 이미 활성 상태(취소되지 않은 상태)로 존재하는 경우 중복 실행으로 판단합니다.
//
// 중복으로 판단되면, 요청자에게 "이미 진행 중"임을 알리는 알림을 비동기로 전송한 뒤
// true를 반환하여 호출자가 새로운 Task 시작을 즉시 중단할 수 있도록 합니다.
// 중복이 없다면 false를 반환합니다.
func (s *Service) rejectIfAlreadyRunning(serviceStopCtx context.Context, req *contract.TaskSubmitRequest) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, task := range s.tasks {
		if task.ID() == req.TaskID && task.CommandID() == req.CommandID && !task.IsCanceled() {
			// 동일한 작업이 이미 실행 중임을 사용자에게 알립니다.
			// 사용자가 직접 요청한 경우(TaskRunByUser)에만 이전 작업 취소 버튼을 노출합니다.
			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    task.InstanceID(),
				Message:       "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요.",
				Elapsed:       task.Elapsed(),
				ErrorOccurred: false,
				Cancelable:    req.RunBy == contract.TaskRunByUser,
			})

			return true
		}
	}

	return false
}

// registerAndRunTask 새로운 Task 인스턴스를 생성하고, 활성 목록에 안전하게 등록한 뒤 고루틴으로 실행합니다.
//
// # 재시도 전략
//
// InstanceID 생성과 등록 사이의 극히 짧은 시간 간격(TOCTOU) 동안 동일 ID가 충돌할 가능성에 대비하여,
// 최대 3회까지 새로운 ID를 발급받아 등록을 재시도합니다.
//
// # 2단계 ID 충돌 감지 설계
//
// 충돌 감지는 의도적으로 두 번에 걸쳐 수행됩니다:
//
//  1. [1차 확인 - 빠른 사전 검증]: Task 인스턴스 생성(NewTask) 이전에 ID 충돌을 미리 감지합니다.
//     생성 비용이 발생하기 전에 낭비를 조기에 차단하는 것이 목적입니다.
//
//  2. [2차 확인 - 최종 등록 전 원자적 확인]: NewTask 실행 중에 드물게 동일 ID가 등록될 수 있으므로,
//     s.tasks 맵에 쓰기 직전 락을 잡은 상태에서 다시 한번 충돌 여부를 확인합니다.
//     이 과정이 실질적인 레이스 컨디션(Race Condition)을 원천 차단합니다.
//
// # Task 생성 실패 처리
//
// NewTask가 nil을 반환하는 경우는 설정 오류 등 복구 불가능한 상황이므로,
// 재시도 없이 즉시 사용자에게 오류 알림을 보내고 종료합니다.
func (s *Service) registerAndRunTask(serviceStopCtx context.Context, req *contract.TaskSubmitRequest, cfg *provider.ResolvedConfig) {
	// 무한 루프 방지를 위한 최대 재시도 횟수입니다.
	// ID 충돌은 매우 드문 이벤트이므로 3회면 충분합니다.
	const maxRetries = 3

	for i := range maxRetries {
		// =====================================================================
		// [단계 1] InstanceID 생성
		// =====================================================================
		// ID 생성은 락 바깥에서 수행하여 Lock Holding Time을 최소화합니다.
		var instanceID = s.idGenerator.New()

		// =====================================================================
		// [단계 2] 1차 충돌 확인 (빠른 사전 검증)
		// =====================================================================
		// Task 생성(NewTask) 전에 ID 충돌을 빠르게 감지해 불필요한 생성 비용을 예방합니다.
		// 충돌 시, 락 내에서 재시도하지 않고(Deadlock 위험 방지) 즉시 락을 해제한 뒤
		// 루프 처음으로 돌아가 새로운 ID를 발급받습니다.
		s.runningMu.Lock()
		if _, exists := s.tasks[instanceID]; exists {
			s.runningMu.Unlock()

			applog.WithComponentAndFields(component, applog.Fields{
				"task_id":     req.TaskID,
				"command_id":  req.CommandID,
				"instance_id": instanceID,
				"attempt":     i + 1,
				"max_retries": maxRetries,
			}).Debug("Task 1차 등록 실패: ID 충돌 (재시도 예정)")

			continue
		}
		s.runningMu.Unlock()

		// =====================================================================
		// [단계 3] Task 인스턴스 생성
		// =====================================================================
		// 락 바깥에서 Task를 생성하여 락 보유 시간을 최소화합니다.
		task, err := cfg.Task.NewTask(provider.NewTaskParams{
			AppConfig:   s.appConfig,
			Request:     req,
			InstanceID:  instanceID,
			Storage:     s.taskResultStore,
			Fetcher:     s.fetcher,
			NewSnapshot: cfg.Command.NewSnapshot,
		})
		if task == nil {
			applog.WithComponentAndFields(component, applog.Fields{
				"task_id":     req.TaskID,
				"command_id":  req.CommandID,
				"notifier_id": req.NotifierID,
				"instance_id": instanceID,
				"error":       err,
			}).Error(err)

			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    "",
				Message:       err.Error(),
				Elapsed:       0,
				ErrorOccurred: true,
				Cancelable:    false,
			})

			return // Task 생성 실패는 설정 오류 등 복구 불가능한 상황이므로, 재시도 없이 즉시 종료합니다.
		}

		// =====================================================================
		// [단계 4] 2차 충돌 확인 및 최종 등록 (원자적 처리)
		// =====================================================================
		// NewTask 실행 중 극히 드물게 동일 ID가 등록될 수 있으므로,
		// s.tasks 맵에 쓰기 직전 락을 잡고 다시 한번 충돌 여부를 확인합니다.
		s.runningMu.Lock()
		if _, exists := s.tasks[instanceID]; exists {
			s.runningMu.Unlock()

			applog.WithComponentAndFields(component, applog.Fields{
				"task_id":     req.TaskID,
				"command_id":  req.CommandID,
				"instance_id": instanceID,
				"attempt":     i + 1,
				"max_retries": maxRetries,
			}).Warn("Task 2차 등록 실패: 레이스 컨디션 감지 (재시도 예정)")

			continue
		}

		// 충돌이 없다면, 락을 잡은 상태에서 원자적으로 등록합니다.
		s.tasks[instanceID] = task

		s.runningMu.Unlock()

		// =====================================================================
		// [단계 5] Task 실행
		// =====================================================================
		s.taskStopWG.Add(1)
		go func(t provider.Task) {
			defer s.taskStopWG.Done()
			defer func() {
				s.taskDoneC <- t.InstanceID()
			}()

			// context.Background()를 전달하는 이유:
			// serviceStopCtx가 취소되더라도 Task 내부의 알림 전송이 중단되지 않도록 하기 위함입니다.
			// Task의 중단은 context가 아닌 task.Cancel()을 통해 명시적으로 처리합니다.
			t.Run(context.Background(), s.notificationSender)
		}(task)

		// =====================================================================
		// [단계 6] 시작 알림 전송 (선택적)
		// =====================================================================
		if req.NotifyOnStart {
			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    instanceID,
				Message:       "작업 진행중입니다. 잠시만 기다려 주세요.",
				Elapsed:       0,
				ErrorOccurred: false,
				Cancelable:    req.RunBy == contract.TaskRunByUser,
			})
		}

		return // 모든 단계가 성공적으로 완료되었습니다.
	}

	// =====================================================================
	// 모든 재시도 소진
	// =====================================================================
	// maxRetries 횟수를 모두 사용했음에도 ID 충돌이 해소되지 않은 경우입니다.
	// 이는 정상적인 운영 환경에서는 발생해서는 안 되는 비정상 상황입니다.

	applog.WithComponentAndFields(component, applog.Fields{
		"task_id":      req.TaskID,
		"command_id":   req.CommandID,
		"notifier_id":  req.NotifierID,
		"max_retries":  maxRetries,
		"active_tasks": len(s.tasks),
	}).Error("Task 실행 실패: ID 생성 충돌 한도 초과")

	go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
		NotifierID:    req.NotifierID,
		TaskID:        req.TaskID,
		CommandID:     req.CommandID,
		InstanceID:    "",
		Message:       "시스템 오류로 작업 실행에 실패했습니다. (ID 충돌)",
		Elapsed:       0,
		ErrorOccurred: true,
		Cancelable:    false,
	})
}

// Submit Task 실행 요청을 검증하고 이벤트 루프의 실행 큐에 등록합니다.
//
// 요청은 아래 순서로 검증된 후 큐에 등록됩니다:
//  1. 요청 객체 유효성 검사 (nil 체크, 필드 유효성)
//  2. TaskID / CommandID 지원 여부 확인 (지원하지 않으면 즉시 오류 반환)
//  3. 서비스 실행 상태 확인
//  4. taskSubmitC 채널에 비동기로 전달
//
// 매개변수:
//   - ctx: 채널이 가득 찼을 때 호출자가 대기를 취소할 수 있는 컨텍스트입니다.
//     ctx가 취소되면 ctx.Err()를 반환합니다.
//   - req: 실행을 요청할 Task의 식별 정보(TaskID, CommandID, NotifierID 등)를 담은 요청 객체입니다.
//
// 반환값:
//   - nil: 요청이 성공적으로 큐에 등록된 경우
//   - error: 요청이 유효하지 않거나, 서비스가 중지 중이거나, ctx가 취소된 경우
func (s *Service) Submit(ctx context.Context, req *contract.TaskSubmitRequest) (err error) {
	if req == nil {
		return ErrInvalidTaskSubmitRequest
	}

	// 전달받은 작업 실행 요청 정보가 유효한지 검증합니다.
	if err := req.Validate(); err != nil {
		return err
	}

	// handleStop()이 taskSubmitC를 닫은 이후에 Submit() 메서드가 호출될 경우,
	// 닫힌 채널에 전송을 시도해 패닉이 발생할 수 있습니다.
	// defer + recover로 이를 잡아 패닉을 에러로 변환하여 호출자에게 안전하게 반환합니다.
	defer func() {
		if r := recover(); r != nil {
			err = newTaskSubmitPanicError(r)

			applog.WithComponentAndFields(component, applog.Fields{
				"task_id":          req.TaskID,
				"command_id":       req.CommandID,
				"notifier_id":      req.NotifierID,
				"submit_queue_len": len(s.taskSubmitC),
				"panic":            r,
			}).Error("Task 실행 요청 실패: 패닉 발생")
		}
	}()

	// [검증 1] 요청받은 작업을 수행할 수 있는 유효한 설정이 있는지 조회합니다.
	// Fail Fast 원칙에 따라, 이벤트 루프에 전달하기 전에 미리 걸러냅니다.
	if _, err := provider.FindConfig(req.TaskID, req.CommandID); err != nil {
		return err
	}

	// [검증 2] 서비스 실행 상태를 확인합니다.
	// running 플래그를 읽을 때는 뮤텍스로 보호하여 데이터 레이스를 방지합니다.
	s.runningMu.Lock()
	running := s.running
	s.runningMu.Unlock()

	if !running {
		return ErrServiceNotRunning
	}

	// [큐잉] 락을 해제한 상태에서 채널 전송을 시도합니다.
	// Cancel()과 달리 ctx를 통해 블로킹 대기를 지원합니다.
	// 이는 작업 제출이 일시적인 큐 포화 상태에서도 ctx 타임아웃까지 재시도를 허용하기 위함입니다.
	// (이벤트 루프가 채널을 소비하면 자연스럽게 전송이 완료됩니다)
	select {
	case s.taskSubmitC <- req:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// Cancel 전달받은 InstanceID에 해당하는 실행 중인 Task의 취소를 요청합니다.
//
// 이 메서드는 취소 요청을 taskCancelC 채널에 전달하는 역할만 담당합니다.
// 실제 취소 처리(task.Cancel() 호출 및 사용자 알림)는 이벤트 루프의 handleTaskCancel()이 수행합니다.
//
// 반환값:
//   - nil: 취소 요청이 성공적으로 큐에 등록된 경우
//   - error: 서비스가 실행 중이 아니거나, 취소 요청 큐가 가득 찬 경우
func (s *Service) Cancel(instanceID contract.TaskInstanceID) (err error) {
	// handleStop()이 taskCancelC를 닫은 이후에 Cancel() 메서드가 호출될 경우,
	// 닫힌 채널에 전송을 시도해 패닉이 발생할 수 있습니다.
	// defer + recover로 이를 잡아 패닉을 에러로 변환하여 호출자에게 안전하게 반환합니다.
	defer func() {
		if r := recover(); r != nil {
			err = newTaskCancelPanicError(r)

			applog.WithComponentAndFields(component, applog.Fields{
				"instance_id":      instanceID,
				"cancel_queue_len": len(s.taskCancelC),
				"panic":            r,
			}).Error("Task 취소 실패: 패닉 발생")
		}
	}()

	// 서비스 실행 상태를 확인합니다.
	// running 플래그를 읽을 때는 뮤텍스로 보호하여 데이터 레이스를 방지합니다.
	s.runningMu.Lock()
	running := s.running
	s.runningMu.Unlock()

	if !running {
		return ErrServiceNotRunning
	}

	// 락을 해제한 상태에서 비블로킹 방식으로 채널에 전송을 시도합니다.
	// Submit()과 달리 context를 통한 대기 없이 즉시 실패를 반환합니다.
	// 취소는 사용자가 명시적으로 요청하는 경우로, 큐가 가득 찼다면 즉시 알려주는 것이 더 적합합니다.
	select {
	case s.taskCancelC <- instanceID:
		return nil

	default:
		return ErrCancelQueueFull
	}
}
