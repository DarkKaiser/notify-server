package task

import (
	"context"
	"fmt"
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

			_, err := s.cron.AddFunc(c.Scheduler.TimeSpec, func() {
				if taskRunner.TaskRun(TaskID(t.ID), TaskCommandID(c.ID), c.DefaultNotifierID, false) == false {
					taskCtx := context.Background()
					taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, t.ID)
					taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, c.ID)
					taskCtx = context.WithValue(taskCtx, TaskCtxKeyErrorOccurred, true)

					m := fmt.Sprintf("Task 스케쥴러에서 요청한 '%s::%s' Task의 실행 요청이 실패하였습니다.", t.ID, c.ID)

					log.Error(m)
					taskNotificationSender.Notify(c.DefaultNotifierID, m, taskCtx)
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
