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

	msgTaskNotFound           = "등록되지 않은 작업입니다.😱"
	msgTaskAlreadyRunning     = "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요."
	msgTaskRunning            = "작업 진행중입니다. 잠시만 기다려 주세요."
	msgTaskCanceledByUser     = "사용자 요청에 의해 작업이 취소되었습니다."
	msgTaskCancelInfoNotFound = "해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)"
)

// Service
type Service struct {
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

func NewService(appConfig *config.AppConfig) *Service {
	return &Service{
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

func (s *Service) Start(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
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

func (s *Service) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case req := <-s.taskRunC:
			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    req.TaskID,
				"command_id": req.CommandID,
				"run_by":     req.RunBy,
			}).Debug("새로운 Task 실행 요청 수신")

			if req.TaskContext == nil {
				req.TaskContext = NewTaskContext()
			}
			req.TaskContext = req.TaskContext.WithTask(req.TaskID, req.CommandID)

			searchResult, err := findConfig(req.TaskID, req.CommandID)
			if err != nil {
				m := msgTaskNotFound

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    req.TaskID,
					"command_id": req.CommandID,
					"error":      err,
				}).Error(m)

				go s.notificationSender.Notify(req.TaskContext.WithError(), req.NotifierID, m)

				continue
			}

			// 인스턴스 중복 실행 확인 (Concurrency Control)
			// AllowMultiple=false인 경우, 이미 실행 중인 동일 CommandID의 태스크가 있다면 실행을 거부합니다.
			var alreadyRunTaskHandler TaskHandler
			if !searchResult.Command.AllowMultiple {
				s.runningMu.Lock()
				for _, handler := range s.taskHandlers {
					// 작업 중복 확인 로직
					if handler.GetID() == req.TaskID && handler.GetCommandID() == req.CommandID && handler.IsCanceled() == false {
						alreadyRunTaskHandler = handler
						break
					}
				}
				s.runningMu.Unlock()

				if alreadyRunTaskHandler != nil {
					req.TaskContext = req.TaskContext.WithInstanceID(alreadyRunTaskHandler.GetInstanceID(), alreadyRunTaskHandler.ElapsedTimeAfterRun())
					go s.notificationSender.Notify(req.TaskContext, req.NotifierID, msgTaskAlreadyRunning)
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

			h, err := searchResult.Task.NewTaskFn(instanceID, req, s.appConfig)
			if h == nil {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    req.TaskID,
					"command_id": req.CommandID,
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
			req.TaskContext = req.TaskContext.WithInstanceID(instanceID, 0)
			go h.Run(req.TaskContext, s.notificationSender, s.taskStopWaiter, s.taskDoneC)

			if req.NotifyOnStart == true {
				go s.notificationSender.Notify(req.TaskContext.WithInstanceID(instanceID, 0), req.NotifierID, msgTaskRunning)
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

				go s.notificationSender.Notify(NewTaskContext().WithTask(taskHandler.GetID(), taskHandler.GetCommandID()), taskHandler.GetNotifierID(), msgTaskCanceledByUser)
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

				go s.notificationSender.NotifyDefault(fmt.Sprintf(msgTaskCancelInfoNotFound, instanceID))
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

func (s *Service) Run(req *RunRequest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 실행 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    req.TaskID,
				"command_id": req.CommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	s.taskRunC <- req

	return nil
}

func (s *Service) Cancel(instanceID InstanceID) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = apperrors.New(apperrors.ErrInternal, fmt.Sprintf("Task 취소 요청중에 panic 발생: %v", r))

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": instanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.taskCancelC <- instanceID

	return nil
}

func (s *Service) SetNotificationSender(notificationSender NotificationSender) {
	s.notificationSender = notificationSender
}
