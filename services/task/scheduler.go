package task

import (
	"github.com/darkkaiser/notify-server/g"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"sync"
)

type scheduler struct {
	cron *cron.Cron

	running   bool
	runningMu sync.Mutex
}

func (s *scheduler) Start(config *g.AppConfig, taskRunner TaskRunner, taskNotificationSender TaskNotificationSender) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running == true {
		return
	}

	s.cron = cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.StandardLogger())))

	for _, t := range config.Tasks {
		for _, c := range t.Commands {
			if c.Scheduler.Runnable == false {
				continue
			}

			// ※ 아래 구문을 func() 내부로 옮기게 되면 for 루프 마지막 항목의 값으로만 전달된다.
			taskID := TaskID(t.ID)
			taskCommandID := TaskCommandID(c.ID)
			defaultNotifierID := c.DefaultNotifierID

			_, err := s.cron.AddFunc(c.Scheduler.TimeSpec, func() {
				if taskRunner.TaskRun(taskID, taskCommandID, defaultNotifierID, false, TaskRunByScheduler) == false {
					m := "작업 스케쥴러에서의 작업 실행 요청이 실패하였습니다.😱"

					log.Error(m)

					taskNotificationSender.NotifyWithTaskContext(defaultNotifierID, m, NewContext().WithTask(taskID, taskCommandID).WithError())
				}
			})

			if err != nil {
				log.Panic(err)
			}
		}
	}

	s.cron.Start()

	s.running = true

	log.Debug("Task 스케쥴러 시작됨")
}

func (s *scheduler) Stop() {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running == false {
		return
	}

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.running = false

	log.Debug("Task 스케쥴러 중지됨")
}
