package task

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

// scheduler 앱 설정(AppConfig)에 정의된 작업을 Cron 스케줄에 맞춰 실행 관리하는 구조체입니다.
type scheduler struct {
	cron *cron.Cron

	running   bool
	runningMu sync.Mutex
}

// Start 스케줄러를 시작하고 정의된 작업들을 Cron에 등록합니다.
func (s *scheduler) Start(appConfig *config.AppConfig, taskExecutor TaskExecutor, taskNotificationSender TaskNotificationSender) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return
	}

	// Cron 인스턴스 초기화: 초 단위 스케줄링 지원 및 로거 설정
	s.cron = cron.New(
		cron.WithLogger(cron.VerbosePrintfLogger(log.StandardLogger())),
		cron.WithSeconds(),
	)

	// 설정 파일의 모든 작업을 순회하며 스케줄링 등록
	for _, t := range appConfig.Tasks {
		for _, c := range t.Commands {
			if !c.Scheduler.Runnable {
				continue
			}

			// 클로저 캡처 문제 방지를 위해 로컬 변수에 재할당 (중요!)
			taskID := TaskID(t.ID)
			taskCommandID := TaskCommandID(c.ID)
			defaultNotifierID := c.DefaultNotifierID
			timeSpec := c.Scheduler.TimeSpec

			// Cron 스케줄 등록
			_, err := s.cron.AddFunc(timeSpec, func() {
				// 작업 실행 요청. 실패 시(false 반환) 에러 처리 및 알림 발송
				if !taskExecutor.TaskRun(taskID, taskCommandID, defaultNotifierID, false, TaskRunByScheduler) {
					msg := "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다.😱"
					s.handleError(taskNotificationSender, defaultNotifierID, taskID, taskCommandID, msg, nil)
				}
			})

			if err != nil {
				msg := fmt.Sprintf("Cron 스케줄 파싱 실패 (TimeSpec: %s)", timeSpec)
				s.handleError(taskNotificationSender, defaultNotifierID, taskID, taskCommandID, msg, err)
				continue
			}
		}
	}

	s.cron.Start()

	s.running = true

	// 등록된 스케줄 개수 로깅
	registeredCount := len(s.cron.Entries())
	applog.WithComponentAndFields("task.scheduler", log.Fields{
		"registered_schedules": registeredCount,
	}).Info("Task 스케쥴러 시작됨")
}

// Stop 실행 중인 스케줄러를 중지합니다.
func (s *scheduler) Stop() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if !s.running {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.running = false

	applog.WithComponent("task.scheduler").Info("Task 스케쥴러 중지됨")
}

// handleError 에러 로깅 및 알림 전송을 처리하는 헬퍼 메서드
// 에러 발생 시 로그를 남기고, 설정된 Notifier를 통해 담당자에게 알림을 보냅니다.
func (s *scheduler) handleError(taskNotificationSender TaskNotificationSender, notifierID string, taskID TaskID, taskCommandID TaskCommandID, msg string, err error) {
	fields := log.Fields{
		"task_id":    taskID,
		"command_id": taskCommandID,
		"run_by":     TaskRunByScheduler,
	}
	if err != nil {
		fields["error"] = err
		// 에러 객체가 있으면 메시지에 상세 내용 추가
		msg = fmt.Sprintf("%s: %v", msg, err)
	}

	applog.WithComponentAndFields("task.scheduler", fields).Error(msg)

	// 관리자 알림 발송 (에러 컨텍스트 포함)
	taskNotificationSender.NotifyWithTaskContext(
		notifierID,
		msg,
		NewContext().WithTask(taskID, taskCommandID).WithError(),
	)
}
