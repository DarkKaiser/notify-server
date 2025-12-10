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
func (s *scheduler) Start(appConfig *config.AppConfig, runner Runner, notificationSender NotificationSender) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running {
		return
	}

	// Cron 인스턴스 초기화: 초 단위 스케줄링 지원 및 로거, 미들웨어 설정
	s.cron = cron.New(
		cron.WithSeconds(),
		cron.WithLogger(cron.VerbosePrintfLogger(log.StandardLogger())), // 기본 로거 추가
		cron.WithChain(
			cron.Recover(cron.VerbosePrintfLogger(log.StandardLogger())),            // Panic 복구
			cron.SkipIfStillRunning(cron.VerbosePrintfLogger(log.StandardLogger())), // 이전 작업이끝나지 않았으면 스킵
		),
	)

	// 설정 파일의 모든 작업을 순회하며 스케줄링 등록
	for _, t := range appConfig.Tasks {
		for _, c := range t.Commands {
			if !c.Scheduler.Runnable {
				continue
			}

			// 클로저 캡처 문제 방지를 위해 로컬 변수에 재할당 (중요!)
			taskID := ID(t.ID)
			taskCommandID := CommandID(c.ID)
			defaultNotifierID := c.DefaultNotifierID
			timeSpec := c.Scheduler.TimeSpec

			// Cron 스케줄 등록
			_, err := s.cron.AddFunc(timeSpec, func() {
				// 작업 실행 요청. 실패 시 에러 처리 및 알림 발송
				if err := runner.Run(&RunRequest{
					TaskID:        taskID,
					TaskCommandID: taskCommandID,
					NotifierID:    defaultNotifierID,
					NotifyOnStart: false,
					RunBy:         RunByScheduler,
				}); err != nil {
					msg := "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다.😱"
					s.handleError(notificationSender, defaultNotifierID, taskID, taskCommandID, msg, err)
				}
			})

			if err != nil {
				msg := fmt.Sprintf("Cron 스케줄 파싱 실패 (TimeSpec: %s)", timeSpec)
				s.handleError(notificationSender, defaultNotifierID, taskID, taskCommandID, msg, err)
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

	if s.cron != nil {
		ctx := s.cron.Stop()
		<-ctx.Done()
	}

	s.running = false
	s.cron = nil

	applog.WithComponent("task.scheduler").Info("Task 스케쥴러 중지됨")
}

// handleError 에러 로깅 및 알림 전송을 처리하는 헬퍼 메서드
// 에러 발생 시 로그를 남기고, 설정된 Notifier를 통해 담당자에게 알림을 보냅니다.
func (s *scheduler) handleError(notificationSender NotificationSender, notifierID string, taskID ID, taskCommandID CommandID, msg string, err error) {
	fields := log.Fields{
		"task_id":    taskID,
		"command_id": taskCommandID,
		"run_by":     RunByScheduler,
	}
	if err != nil {
		fields["error"] = err
		// 에러 객체가 있으면 메시지에 상세 내용 추가
		msg = fmt.Sprintf("%s: %v", msg, err)
	}

	applog.WithComponentAndFields("task.scheduler", fields).Error(msg)

	notificationSender.Notify(
		NewTaskContext().WithTask(taskID, taskCommandID).WithError(),
		notifierID,
		msg,
	)
}
