package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/idgen"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/storage"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

const (
	defaultChannelBufferSize = 10

	msgTaskRunning        = "작업 진행중입니다. 잠시만 기다려 주세요."
	msgTaskAlreadyRunning = "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요."
	msgTaskCanceledByUser = "사용자 요청에 의해 작업이 취소되었습니다."
	msgTaskNotFound       = "해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)"
)

// Service 애플리케이션의 핵심 비즈니스 로직인 Task의 실행, 스케줄링, 상태 관리를 총괄하는 중앙 오케스트레이터(Central Orchestrator)입니다.
//
// 설계 철학 및 아키텍처:
// 이 서비스는 '단일 이벤트 루프(Single-threaded Event Loop)' 패턴을 차용하여 설계되었습니다.
// Task의 제출(Submit), 완료(Done), 취소(Cancel) 등 모든 상태 변경 이벤트는 채널을 통해 직렬화(Serialize)되며,
// 메인 루프에서 순차적으로 처리됩니다. 이를 통해 복잡한 뮤텍스(Mutex) 사용을 최소화하고,
// 교착 상태(Deadlock) 없는 안전한 동시성을 보장합니다.
//
// 주요 기능 및 책임:
//  1. 요청 수신 및 검증 (Request Handling): 실행 요청의 유효성을 'Fail Fast' 원칙에 따라 즉시 검증합니다.
//  2. 스케줄링 (Scheduling): Cron 표현식에 따라 정해진 시간에 Task를 자동으로 실행합니다.
//  3. 동시성 제어 (Concurrency Control): Task별 설정(AllowMultiple)에 따라 중복 실행 방지 및 실행 흐름을 제어합니다.
//  4. 안정적 종료 (Graceful Shutdown): 시스템 종료 시 실행 중인 Task들이 안전하게 작업을 마칠 수 있도록 대기하고 정리합니다.
type Service struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	// tasks 현재 활성화(Running) 상태인 모든 Task의 인스턴스를 관리하는 인메모리 저장소입니다.
	tasks map[contract.TaskInstanceID]provider.Task

	// idGenerator는 각 Task 실행 인스턴스에 대해 전역적으로 고유한 식별자(InstanceID)를 발급하는 생성기입니다.
	idGenerator idgen.InstanceIDGenerator

	// notificationSender는 작업의 실행 결과나 중요 이벤트를 외부 시스템(예: Telegram, Slack 등)으로 전파하는
	// 책임을 가진 추상화된 인터페이스(Interface)입니다.
	notificationSender contract.NotificationSender

	// taskSubmitC는 새로운 Task 실행 요청을 전달받는 채널입니다.
	taskSubmitC chan *contract.TaskSubmitRequest

	// taskDoneC는 Task 실행이 완료되었음을 알리는 신호 채널입니다.
	taskDoneC chan contract.TaskInstanceID

	// taskCancelC는 실행 중인 Task의 취소를 요청하는 제어 채널입니다.
	taskCancelC chan contract.TaskInstanceID

	// taskStopWG는 실행 중인 모든 Task의 종료를 추적하고 대기하는 동기화 객체입니다.
	taskStopWG sync.WaitGroup

	storage storage.TaskResultStorage
}

// NewService 새로운 Service 인스턴스를 생성합니다.
func NewService(appConfig *config.AppConfig) *Service {
	return &Service{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		tasks: make(map[contract.TaskInstanceID]provider.Task),

		idGenerator: idgen.InstanceIDGenerator{},

		notificationSender: nil,

		taskSubmitC: make(chan *contract.TaskSubmitRequest, defaultChannelBufferSize),
		taskDoneC:   make(chan contract.TaskInstanceID, defaultChannelBufferSize),
		taskCancelC: make(chan contract.TaskInstanceID, defaultChannelBufferSize),

		storage: storage.NewFileTaskResultStorage(config.AppName),
	}
}

func (s *Service) SetNotificationSender(notificationSender contract.NotificationSender) {
	s.notificationSender = notificationSender
}

// Start Task 서비스를 시작하여 요청을 처리할 준비를 합니다.
func (s *Service) Start(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	if s.notificationSender == nil {
		serviceStopWG.Done()
		return apperrors.New(apperrors.Internal, "NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running {
		serviceStopWG.Done()
		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")
		return nil
	}

	go s.run0(serviceStopCtx, serviceStopWG)

	s.running = true

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

// run0 서비스의 메인 이벤트 루프입니다.
// 단일 고루틴에서 채널을 통해 들어오는 모든 이벤트를 순차적으로 처리합니다(Single-Threaded Event Loop).
func (s *Service) run0(serviceStopCtx context.Context, serviceStopWG *sync.WaitGroup) {
	defer serviceStopWG.Done()

	// 메인 루프가 예기치 않게 종료(Panic)되지 않도록 보호합니다.
	defer func() {
		if r := recover(); r != nil {
			applog.WithComponentAndFields("task.service", applog.Fields{
				"panic": r,
			}).Error("Critical: Task Service 메인 루프 Panic 발생")
		}
	}()

	for {
		select {
		case req, ok := <-s.taskSubmitC:
			// 채널이 닫혔다면 서비스가 종료 중이라는 의미이므로 루프를 탈출합니다.
			if !ok {
				return
			}
			s.handleSubmitRequest(serviceStopCtx, req)

		case instanceID := <-s.taskDoneC:
			s.handleTaskDone(instanceID)

		case instanceID := <-s.taskCancelC:
			s.handleTaskCancel(serviceStopCtx, instanceID)

		case <-serviceStopCtx.Done():
			s.handleStop()
			return
		}
	}
}

// handleSubmitRequest 새로운 Task 실행 요청을 처리합니다.
func (s *Service) handleSubmitRequest(serviceStopCtx context.Context, req *contract.TaskSubmitRequest) {
	applog.WithComponentAndFields("task.service", applog.Fields{
		"task_id":    req.TaskID,
		"command_id": req.CommandID,
		"run_by":     req.RunBy,
	}).Debug("새로운 Task 실행 요청 수신")

	cfg, err := provider.FindConfig(req.TaskID, req.CommandID)
	if err != nil {
		applog.WithComponentAndFields("task.service", applog.Fields{
			"task_id":    req.TaskID,
			"command_id": req.CommandID,
			"error":      err,
		}).Error(provider.ErrTaskNotSupported.Error())

		go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
			NotifierID:    req.NotifierID,
			TaskID:        req.TaskID,
			CommandID:     req.CommandID,
			InstanceID:    "", // InstanceID not yet generated
			Message:       provider.ErrTaskNotSupported.Error(),
			ElapsedTime:   0,
			ErrorOccurred: true,
			Cancelable:    false,
		})

		return
	}

	// 인스턴스 중복 실행 확인
	// AllowMultiple이 false인 경우, 동일한 Task(Command)가 이미 실행 중이면 요청을 거부합니다.
	if !cfg.Command.AllowMultiple {
		if s.checkConcurrencyLimit(serviceStopCtx, req) {
			return
		}
	}

	s.createAndStartTask(serviceStopCtx, req, cfg)
}

// checkConcurrencyLimit 현재 실행 중인 Task 목록을 순회하여 중복 실행 여부를 확인합니다.
func (s *Service) checkConcurrencyLimit(serviceStopCtx context.Context, req *contract.TaskSubmitRequest) bool {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	for _, task := range s.tasks {
		if task.GetID() == req.TaskID && task.GetCommandID() == req.CommandID && !task.IsCanceled() {
			// req.TaskContext = req.TaskContext.WithTaskInstanceID... -> Removed
			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    task.GetInstanceID(),
				Message:       msgTaskAlreadyRunning,
				ElapsedTime:   time.Duration(task.ElapsedTimeAfterRun()) * time.Second,
				ErrorOccurred: false,
				Cancelable:    false,
			})
			return true
		}
	}

	return false
}

func (s *Service) createAndStartTask(serviceStopCtx context.Context, req *contract.TaskSubmitRequest, cfg *provider.ConfigLookup) {
	// 무한 루프 방지를 위한 최대 재시도 횟수
	const maxRetries = 3

	for i := 0; i < maxRetries; i++ {
		// ID 생성을 락 밖에서 수행하여 Lock Holding Time을 최소화한다.
		var instanceID = s.idGenerator.New()

		// Lock을 잡고 ID 중복 여부를 1차로 빠르게 확인합니다.
		// 만약 충돌한다면 락 내부에서 재시도하지 않고(Deadlock 위험 방지),
		// 즉시 락을 해제한 후 루프의 처음으로 돌아가서 새로운 ID를 발급받습니다.
		s.runningMu.Lock()
		if _, exists := s.tasks[instanceID]; exists {
			s.runningMu.Unlock()

			// 로그는 디버그 레벨로 낮춰서 과도한 로깅을 방지합니다 (어차피 재시도하므로)
			applog.WithComponentAndFields("task.service", applog.Fields{
				"instance_id": instanceID,
			}).Debug("Task ID 충돌 (1차 확인) - 재시도")

			continue
		}
		s.runningMu.Unlock()

		// Task 인스턴스 생성
		h, err := cfg.Task.NewTask(instanceID, req, s.appConfig)
		if h == nil {
			applog.WithComponentAndFields("task.service", applog.Fields{
				"task_id":    req.TaskID,
				"command_id": req.CommandID,
				"error":      err,
			}).Error(err)

			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    "", // InstanceID not generated
				Message:       err.Error(),
				ElapsedTime:   0,
				ErrorOccurred: true,
				Cancelable:    false,
			})

			return // Task 생성 실패는 치명적 오류이므로 재시도하지 않고 종료합니다.
		}

		h.SetStorage(s.storage)

		// 최종 등록 및 충돌 확인
		s.runningMu.Lock()
		if _, exists := s.tasks[instanceID]; exists {
			s.runningMu.Unlock()

			applog.WithComponentAndFields("task.service", applog.Fields{
				"task_id":     req.TaskID,
				"command_id":  req.CommandID,
				"instance_id": instanceID,
				"retry_count": i + 1,
			}).Warn("Task 등록 시점 ID 충돌 발생 (재시도 중...)")

			continue // 충돌 발생 시, 루프의 처음으로 돌아가 새로운 ID로 다시 시작합니다.
		}
		s.tasks[instanceID] = h
		s.runningMu.Unlock()

		// Task 실행
		s.taskStopWG.Add(1)
		// Task 내부의 알림 전송이 서비스 종료 시그널(serviceStopCtx)에 영향받지 않도록
		// context.Background()를 전달합니다. Task 취소는 task.Cancel()을 통해 처리됩니다.
		go h.Run(context.Background(), s.notificationSender, &s.taskStopWG, s.taskDoneC)

		if req.NotifyOnStart {
			go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
				NotifierID:    req.NotifierID,
				TaskID:        req.TaskID,
				CommandID:     req.CommandID,
				InstanceID:    instanceID,
				Message:       msgTaskRunning,
				ElapsedTime:   0,
				ErrorOccurred: false,
				Cancelable:    req.RunBy == contract.TaskRunByUser,
			})
		}

		// 성공적으로 실행했으므로 함수를 종료합니다.
		return
	}

	// 모든 재시도 실패 시
	applog.WithComponentAndFields("task.service", applog.Fields{
		"task_id":    req.TaskID,
		"command_id": req.CommandID,
	}).Error("Task ID 생성 충돌이 반복되어 실행에 실패했습니다.")

	go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
		NotifierID:    req.NotifierID,
		TaskID:        req.TaskID,
		CommandID:     req.CommandID,
		InstanceID:    "",
		Message:       "시스템 오류로 작업 실행에 실패했습니다 (ID 충돌).",
		ElapsedTime:   0,
		ErrorOccurred: true,
		Cancelable:    false,
	})
}

func (s *Service) handleTaskDone(instanceID contract.TaskInstanceID) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if task, exists := s.tasks[instanceID]; exists {
		applog.WithComponentAndFields("task.service", applog.Fields{
			"task_id":     task.GetID(),
			"command_id":  task.GetCommandID(),
			"instance_id": instanceID,
		}).Debug("Task 작업 완료")

		delete(s.tasks, instanceID)
	} else {
		applog.WithComponentAndFields("task.service", applog.Fields{
			"instance_id": instanceID,
		}).Warn("등록되지 않은 Task에 대한 작업완료 메시지 수신")
	}
}

func (s *Service) handleTaskCancel(serviceStopCtx context.Context, instanceID contract.TaskInstanceID) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if task, exists := s.tasks[instanceID]; exists {
		task.Cancel()

		applog.WithComponentAndFields("task.service", applog.Fields{
			"task_id":     task.GetID(),
			"command_id":  task.GetCommandID(),
			"instance_id": instanceID,
		}).Debug("Task 작업 취소")

		go s.notificationSender.Notify(serviceStopCtx, contract.Notification{
			NotifierID:    task.GetNotifierID(),
			TaskID:        task.GetID(),
			CommandID:     task.GetCommandID(),
			InstanceID:    instanceID,
			Message:       msgTaskCanceledByUser,
			ElapsedTime:   time.Duration(task.ElapsedTimeAfterRun()) * time.Second,
			ErrorOccurred: false,
			Cancelable:    false,
		})
	} else {
		applog.WithComponentAndFields("task.service", applog.Fields{
			"instance_id": instanceID,
		}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

		go s.notificationSender.Notify(serviceStopCtx, contract.NewNotification(fmt.Sprintf(msgTaskNotFound, instanceID)))
	}
}

// handleStop 서비스를 안전하게 중지합니다.
func (s *Service) handleStop() {
	applog.WithComponent("task.service").Info("Task 서비스 중지중...")

	s.runningMu.Lock()
	// SubmitTask가 running 상태를 확인하고 채널에 전송하기(send) 전에,
	// 여기서 먼저 running을 false로 설정하여 "닫힌 채널에 전송(Panic)"을 원천 차단합니다.
	// (SubmitTask는 runningMu를 획득해야만 진행 가능하므로, 여기서 running=false 설정 시 안전이 보장됨)
	s.running = false
	// 현재 작업중인 Task의 작업을 모두 취소한다.
	for _, task := range s.tasks {
		task.Cancel()
	}
	s.runningMu.Unlock()

	// 입력 채널을 닫아 더 이상의 외부 요청(Submit, Cancel)을 받지 않음을 명시합니다.
	// 이후 이 채널들에 send를 시도하면 panic이 발생하므로, 앞선 단계(running=false)가 중요합니다.
	close(s.taskSubmitC)
	close(s.taskCancelC)

	// Task의 작업이 모두 취소될 때까지 대기한다.
	// 이 때, taskDoneC 채널이 가득 차서 Task 고루틴들이 블락되는 것을 방지하기 위해 별도 고루틴에서 채널을 비워줍니다.
	// (taskStopWG.Wait()가 완료되면 taskDoneC를 닫을 것이며, 이때 range 루프도 종료됩니다)
	go func() {
		for range s.taskDoneC {
			// Discard: 종료 중이므로 완료 메시지는 무시합니다.
		}
	}()

	done := make(chan struct{})
	go func() {
		s.taskStopWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		// 정상적으로 모든 태스크가 종료됨

	case <-time.After(30 * time.Second):
		applog.WithComponent("task.service").Warn("Service 종료 대기 시간 초과! (30s) 강제 종료합니다.")
	}

	// taskDoneC는 가장 마지막에 닫아야 합니다.
	// 이유: s.taskStopWG.Wait()가 완료되기 전까지는 실행 중인 Task들이 여전히 살아있으며,
	// 이들이 종료되면서 s.taskDoneC <- instanceID를 보낼 수 있습니다.
	// 만약 여기서 미리 닫아버리면 "send on closed channel" 패닉이 발생합니다.
	close(s.taskDoneC)

	s.runningMu.Lock()
	s.tasks = nil
	s.notificationSender = nil
	s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 중지됨")
}

// Submit 작업을 실행 큐에 등록합니다.
func (s *Service) Submit(ctx context.Context, req *contract.TaskSubmitRequest) (err error) {
	if req == nil {
		return apperrors.New(apperrors.Internal, "Invalid task submit request type")
	}

	if err := req.Validate(); err != nil {
		return err
	}
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.Internal, fmt.Sprintf("Task 실행 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", applog.Fields{
				"task_id":    req.TaskID,
				"command_id": req.CommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	// 1. 요청 검증: 요청된 TaskID와 CommandID가 유효한지 먼저 검증합니다.
	if _, err := provider.FindConfig(req.TaskID, req.CommandID); err != nil {
		return err
	}

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	// 2. 상태 검증: 서비스가 실행 중인지 확인합니다.
	if !s.running {
		return apperrors.New(apperrors.Internal, "Task 서비스가 실행중이 아닙니다.")
	}

	// 3. 큐잉: 버퍼드 채널에 요청을 넣습니다.
	select {
	case s.taskSubmitC <- req:
		return nil

	default:
		return apperrors.New(apperrors.Internal, "Task 실행 요청 큐가 가득 찼습니다.")
	}
}

// Cancel 특정 작업 인스턴스의 실행을 취소합니다.
func (s *Service) Cancel(instanceID contract.TaskInstanceID) (err error) {

	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.Internal, fmt.Sprintf("Task 취소 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", applog.Fields{
				"instance_id": instanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return apperrors.New(apperrors.Internal, "Task 서비스가 실행중이 아닙니다.")
	}

	select {
	case s.taskCancelC <- instanceID:
		return nil

	default:
		return apperrors.New(apperrors.Internal, "Task 취소 요청 큐가 가득 찼습니다.")
	}
}
