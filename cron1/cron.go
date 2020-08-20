package cron1

import (
	"github.com/darkkaiser/notify-server/global"
	"github.com/darkkaiser/notify-server/task"
	_ "github.com/darkkaiser/notify-server/task"
	"github.com/robfig/cron"
	log "github.com/sirupsen/logrus"
)

// 반복적으로 실행할 task의 목록을 관리하고 이를 taskmanager에게 요청하기만 한다.
type CronServer struct {
	c   *cron.Cron
	Tmm *task.TaskManager
}

func (c *CronServer) Start(appConfig *global.AppConfig) {
	log.Debug("cron1 start")

	c.c = cron.New()

	for _, t := range appConfig.Tasks {
		for _, command := range t.Commands {
			// command.Time
			//log.Print("add func")
			c.c.AddFunc(command.Time, func() {
				log.Errorf("%s... 마다", command.Command)
				c.Tmm.Run(task.TaskId(t.Id), task.CommandId(command.Command))
			})
		}
	}
	c.c.Start()

	// ※
	// cron솨 task를 분리
	// cron은 단순 지정된 시간에 실행만을 담당하며 해당 함수에서 taskmanager에 실행해달라고 요청
	// 사용자가 태스크 실행 요청시 taskmanager에게 바로 요청하고 결과를 전송받는다.
}
