package task

import (
	"github.com/darkkaiser/notify-server/global"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
	"sync"
)

type scheduler struct {
	cron *cron.Cron

	running   bool
	runningMu sync.Mutex
}

func (s *scheduler) Start(config *global.AppConfig, r TaskRunRequester) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	if s.running == true {
		return
	}

	s.cron = cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.StandardLogger())))

	for _, task := range config.Tasks {
		for _, command := range task.Commands {
			_, err := s.cron.AddFunc(command.TimeSpec, func() {
				if r.TaskRun(TaskId(task.Id), TaskCommandId(command.Id)) == false {
					// @@@@@ 로그 남기고 notify 하기
					//log.Warnf()
				}
			})

			if err != nil {
				log.Panic(err)
			}
		}
	}

	s.cron.Start()

	s.running = true

	log.Debug("Task 스케쥴러가 시작되었습니다.")
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

	log.Debug("Task 스케쥴러가 중지되었습니다.")
}
