package service

import (
	"github.com/darkkaiser/notify-server/global"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"sync"
)

type taskScheduler struct {
	cron *cron.Cron

	running   bool
	runningMu sync.Mutex
}

func (s *taskScheduler) Start(config *global.AppConfig, runner TaskRunner) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running == true {
		return
	}

	s.cron = cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.StandardLogger())))

	for _, t := range config.Tasks {
		for _, c := range t.Commands {
			_, err := s.cron.AddFunc(c.TimeSpec, func() {
				if runner.TaskRun(TaskId(t.Id), TaskCommandId(c.Id), NotifierId(c.NotifierId)) == false {
					// @@@@@ 로그 남기고 notify 하기
					// log.Warnf()
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

func (s *taskScheduler) Stop() {
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
