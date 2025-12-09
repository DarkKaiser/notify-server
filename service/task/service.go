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

// TaskService
type TaskService struct {
	appConfig *config.AppConfig

	running   bool
	runningMu sync.Mutex

	scheduler scheduler

	taskHandlers map[TaskInstanceID]TaskHandler

	instanceIDGenerator instanceIDGenerator

	taskNotificationSender TaskNotificationSender

	taskRunC    chan *TaskRunData
	taskDoneC   chan TaskInstanceID
	taskCancelC chan TaskInstanceID

	taskStopWaiter *sync.WaitGroup
}

func NewService(appConfig *config.AppConfig) *TaskService {
	return &TaskService{
		appConfig: appConfig,

		running:   false,
		runningMu: sync.Mutex{},

		scheduler: scheduler{},

		taskHandlers: make(map[TaskInstanceID]TaskHandler),

		instanceIDGenerator: instanceIDGenerator{},

		taskNotificationSender: nil,

		taskRunC:    make(chan *TaskRunData, 10),
		taskDoneC:   make(chan TaskInstanceID, 10),
		taskCancelC: make(chan TaskInstanceID, 10),

		taskStopWaiter: &sync.WaitGroup{},
	}
}

func (s *TaskService) Run(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) error {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	applog.WithComponent("task.service").Info("Task 서비스 시작중...")

	if s.taskNotificationSender == nil {
		defer serviceStopWaiter.Done()

		return apperrors.New(apperrors.ErrInternal, "TaskNotificationSender 객체가 초기화되지 않았습니다")
	}

	if s.running == true {
		defer serviceStopWaiter.Done()

		applog.WithComponent("task.service").Warn("Task 서비스가 이미 시작됨!!!")

		return nil
	}

	// Task 스케쥴러를 시작한다.
	s.scheduler.Start(s.appConfig, s, s.taskNotificationSender)

	go s.run0(serviceStopCtx, serviceStopWaiter)

	s.running = true

	applog.WithComponent("task.service").Info("Task 서비스 시작됨")

	return nil
}

func (s *TaskService) run0(serviceStopCtx context.Context, serviceStopWaiter *sync.WaitGroup) {
	defer serviceStopWaiter.Done()

	for {
		select {
		case taskRunData := <-s.taskRunC:
			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    taskRunData.TaskID,
				"command_id": taskRunData.TaskCommandID,
				"run_by":     taskRunData.TaskRunBy,
			}).Debug("새로운 Task 실행 요청 수신")

			if taskRunData.TaskCtx == nil {
				taskRunData.TaskCtx = NewContext()
			}
			taskRunData.TaskCtx.WithTask(taskRunData.TaskID, taskRunData.TaskCommandID)

			taskConfig, commandConfig, err := findConfigFromSupportedTask(taskRunData.TaskID, taskRunData.TaskCommandID)
			if err != nil {
				m := "등록되지 않은 작업입니다.😱"

				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    taskRunData.TaskID,
					"command_id": taskRunData.TaskCommandID,
					"error":      err,
				}).Error(m)

				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, m, taskRunData.TaskCtx.WithError())

				continue
			}

			// 다중 인스턴스의 생성이 허용되지 않는 Task인 경우, 이미 실행중인 동일한 Task가 있는지 확인한다.
			if commandConfig.AllowMultipleInstances == false {
				var alreadyRunTaskHandler TaskHandler

				s.runningMu.Lock()
				for _, handler := range s.taskHandlers {
					if handler.GetID() == taskRunData.TaskID && handler.GetCommandID() == taskRunData.TaskCommandID && handler.IsCanceled() == false {
						alreadyRunTaskHandler = handler
						break
					}
				}
				s.runningMu.Unlock()

				if alreadyRunTaskHandler != nil {
					taskRunData.TaskCtx.WithInstanceID(alreadyRunTaskHandler.GetInstanceID(), alreadyRunTaskHandler.ElapsedTimeAfterRun())
					s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, "요청하신 작업은 이미 진행중입니다.\n이전 작업을 취소하시려면 아래 명령어를 클릭하여 주세요.", taskRunData.TaskCtx)
					continue
				}
			}

			var instanceID TaskInstanceID

			s.runningMu.Lock()
			for {
				instanceID = s.instanceIDGenerator.New()
				if _, exists := s.taskHandlers[instanceID]; exists == false {
					break
				}
			}
			s.runningMu.Unlock()

			h, err := taskConfig.NewTaskFn(instanceID, taskRunData, s.appConfig)
			if h == nil {
				applog.WithComponentAndFields("task.service", log.Fields{
					"task_id":    taskRunData.TaskID,
					"command_id": taskRunData.TaskCommandID,
					"error":      err,
				}).Error(err)

				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, err.Error(), taskRunData.TaskCtx.WithError())

				continue
			}

			s.runningMu.Lock()
			s.taskHandlers[instanceID] = h
			s.runningMu.Unlock()

			s.taskStopWaiter.Add(1)
			go h.Run(s.taskNotificationSender, s.taskStopWaiter, s.taskDoneC)

			if taskRunData.NotifyResultOfTaskRunRequest == true {
				s.taskNotificationSender.NotifyWithTaskContext(taskRunData.NotifierID, "작업 진행중입니다. 잠시만 기다려 주세요.", taskRunData.TaskCtx.WithInstanceID(instanceID, 0))
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

				s.taskNotificationSender.NotifyWithTaskContext(taskHandler.GetNotifierID(), "사용자 요청에 의해 작업이 취소되었습니다.", NewContext().WithTask(taskHandler.GetID(), taskHandler.GetCommandID()))
			} else {
				applog.WithComponentAndFields("task.service", log.Fields{
					"instance_id": instanceID,
				}).Warn("등록되지 않은 Task에 대한 작업취소 요청 메시지 수신")

				s.taskNotificationSender.NotifyToDefault(fmt.Sprintf("해당 작업에 대한 정보를 찾을 수 없습니다.😱\n취소 요청이 실패하였습니다.(ID:%s)", instanceID))
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
			s.taskNotificationSender = nil
			s.runningMu.Unlock()

			applog.WithComponent("task.service").Info("Task 서비스 중지됨")

			return
		}
	}
}

func (s *TaskService) TaskRun(taskID TaskID, taskCommandID TaskCommandID, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool) {
	return s.TaskRunWithContext(taskID, taskCommandID, nil, notifierID, notifyResultOfTaskRunRequest, taskRunBy)
}

func (s *TaskService) TaskRunWithContext(taskID TaskID, taskCommandID TaskCommandID, taskCtx TaskContext, notifierID string, notifyResultOfTaskRunRequest bool, taskRunBy TaskRunBy) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("task.service", log.Fields{
				"task_id":    taskID,
				"command_id": taskCommandID,
				"panic":      r,
			}).Error("Task 실행 요청중에 panic 발생")
		}
	}()

	s.taskRunC <- &TaskRunData{
		TaskID:        taskID,
		TaskCommandID: taskCommandID,

		TaskCtx: taskCtx,

		NotifierID: notifierID,

		NotifyResultOfTaskRunRequest: notifyResultOfTaskRunRequest,

		TaskRunBy: taskRunBy,
	}

	return true
}

func (s *TaskService) TaskCancel(taskInstanceID TaskInstanceID) (succeeded bool) {
	defer func() {
		if r := recover(); r != nil {
			succeeded = false

			applog.WithComponentAndFields("task.service", log.Fields{
				"instance_id": taskInstanceID,
				"panic":       r,
			}).Error("Task 취소 요청중에 panic 발생")
		}
	}()

	s.taskCancelC <- taskInstanceID

	return true
}

func (s *TaskService) SetTaskNotificationSender(taskNotificiationSender TaskNotificationSender) {
	s.taskNotificationSender = taskNotificiationSender
}
