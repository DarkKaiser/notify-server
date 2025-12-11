package task

import (
	"context"
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

const (
	defaultChannelBufferSize = 10
)

// TaskService
type TaskService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[InstanceID]TaskHandler

	instanceIDGenerator instanceIDGenerator

	notificationSender NotificationSender

	taskRunC    chan *RunRequest
	taskDoneC   chan InstanceID
	taskCancelC chan InstanceID

	taskStopWaiter *sync.WaitGroup

	taskStorage TaskResultStorage
}

func NewService(appConfig *config.AppConfig) *TaskService {
	return &TaskService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[InstanceID]TaskHandler),

		instanceIDGenerator: instanceIDGenerator{},

		notificationSender: nil,

		taskRunC:    make(chan *RunRequest, defaultChannelBufferSize),
		taskDoneC:   make(chan InstanceID, defaultChannelBufferSize),
		taskCancelC: make(chan InstanceID, defaultChannelBufferSize),

		taskStopWaiter: &sync.WaitGroup{},

		taskStorage: NewFileTaskResultStorage(config.AppName),
	}
}

func (s *TaskService) Start(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	// NotificationSender 검증
	if s.notificationSender == nil {
		defer serviceStopWaiter.Done()
		return apperrors.New(apperrors.ErrInternal, "NotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()
		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")
		return nil
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.appConfig, s, s.notificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

func (s *TaskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case req := <-s.taskRunC:
			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    req.TaskID,
				"command_id": req.TaskCommandID,
				"run_by":     req.RunBy,
			}).Debug("새로운 Task 실행 요청 수신")

			if req.TaskContext == nil {
				req.TaskContext = NewTaskContext()
			}
			req.TaskContext = req.TaskContext.WithTask(req.TaskID, req.TaskCommandID)

			taskConfig, commandConfig, err := findConfigFromSupportedTask(req.TaskID, req.TaskCommandID)
			if err != nil {
				m := "등록되지 않은 작업입니다.😱"

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    req.TaskID,
					"command_id": req.TaskCommandID,
					"error":      err,
				}).Error(m)

				go s.notificationSender.Notify(req.TaskContext.WithError(), req.NotifierID, m)

				continue
			}

			// 다중 인스턴스의 생성이 허용되지 않는 Task인 경우, 이미 실행중인 동일한 Task가 있는지 확인한다.
			if commandConfig.AllowMultipleInstances == false {
				var alreadyRunTaskHandler TaskHandler

				s.runningMu.Lock()
				for _, handler := range s.taskHandlers {
					if handler.GetID() == req.TaskID && handler.GetCommandID() == req.TaskCommandID && handler.IsCanceled() == false {
						alreadyRunTaskHandler = handler
						break
					}
				}
				s.runningMu.Unlock()

				if alreadyRunTaskHandler != nil {
					req.TaskContext = req.TaskContext.WithInstanceID(alreadyRunTaskHandler.GetInstanceID(), alreadyRunTaskHandler.ElapsedTimeAfterRun())
					go s.notificationSender.Notify(req.TaskContext, req.NotifierID, "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요.")
					continue
				}
			}

			var instanceID InstanceID

			s.runningMu.Lock()
			for {
				instanceID = s.instanceIDGenerator.New()
				if _, exists := s.taskHandlers[instanceID]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			h, err := taskConfig.NewTaskFn(instanceID, req, s.appConfig)
			if h == nil {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    req.TaskID,
					"command_id": req.TaskCommandID,
					"error":      err,
				}).Error(err)

				go s.notificationSender.Notify(req.TaskContext.WithError(), req.NotifierID, err.Error())

				continue
			}

			// 생성된 Task에 Storage 주입
			// TaskHandler 인터페이스를 통해 주입하므로 구체적인 타입을 알 필요가 없음
			h.SetStorage(s.taskStorage)

			s.runningMu.Lock()
			s.taskHandlers[instanceID] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.notificationSender, s.taskStopWaiter, s.taskDoneC)

			if req.NotifyOnStart == true {
				go s.notificationSender.Notify(req.TaskContext.WithInstanceID(instanceID, 0), req.NotifierID, "작업 진행중입니다. 잠시만 기다려 주세요.")
			}

		case instanceID := <-s.taskDoneC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":     taskHandler.GetID(),
					"command_id":  taskHandler.GetCommandID(),
					"instance_id": instanceID,
				}).Debug("Task 작업 완료")

				delete(s.taskHandlers, instanceID)
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업완료 메시지 수신")
			}
			s.runningMu.Unlock()

		case instanceID := <-s.taskCancelC:
			s.runningMu.Lock()
			if taskHandler, exists := s.taskHandlers[instanceID]; exists == true {
				taskHandler.Cancel()

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":     taskHandler.GetID(),
					"command_id":  taskHandler.GetCommandID(),
					"instance_id": instanceID,
				}).Debug("Task 작업 취소")

				go s.notificationSender.Notify(NewTaskContext().WithTask(taskHandler.GetID(), taskHandler.GetCommandID()), taskHandler.GetNotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.")
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

				go s.notificationSender.NotifyDefault(fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)", instanceID))
			}
			s.runningMu.Unlock()

		case <-serviceStopCtx.Done():
			applog.WithComponent("task.service").Info("Task 서비스 중지중...")

			// Task 스케쥴러를 중지한다.
			s.scheduler.Stop()

			s.runningMu.Lock()
			// 현재 작업중인 Task의 작업을 모두 취소한다.
			for _, handler := range s.taskHandlers {
				handler.Cancel()
			}
			s.runningMu.Unlock()

			close(s.taskRunC)
			close(s.taskCancelC)

			// Task의 작업이 모두 취소될 때까지 대기한다.
			s.taskStopWaiter.Wait()

			close(s.taskDoneC)

			s.runningMu.Lock()
			s.running = false
			s.taskHandlers = nil
			s.notificationSender = nil
			s.runningMu.Unlock()

			applog.WithComponent("task.service").Info("Task 서비스 중지됨")

			return
		}
	}
}

func (s *TaskService) Run(req *RunRequest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 실행 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    req.TaskID,
				"command_id": req.TaskCommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	s.taskRunC <- req

	return nil
}

func (s *TaskService) Cancel(taskInstanceID InstanceID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 취소 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": taskInstanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.taskCancelC <- taskInstanceID

	return nil
}

func (s *TaskService) SetNotificationSender(notificationSender NotificationSender) {
	s.notificationSender = notificationSender
}
