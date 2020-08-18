package cron

import (
	"fmt"
	_ "github.com/darkkaiser/notify-server/task"
	"github.com/robfig/cron"
)

type Cron struct {
	// 반복적으로 실행할 task의 목록을 관리하고 이를 taskmanager에게 요청하기만 한다.
}

func (c *Cron) Start() {
	cc := cron.New()
	cc.AddFunc("30 * * * *", func() { fmt.Println("30 분마다") })

	// ※
	// cron솨 task를 분리
	// cron은 단순 지정된 시간에 실행만을 담당하며 해당 함수에서 taskmanager에 실행해달라고 요청
	// 사용자가 태스크 실행 요청시 taskmanager에게 바로 요청하고 결과를 전송받는다.
}
