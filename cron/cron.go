package cron

import (
	"fmt"
	"github.com/robfig/cron"
)

type Cron struct {
	// task를 모두 등록하여 관리한다.
}

func (c *Cron) Start() {
	cc := cron.New()
	cc.AddFunc("30 * * * *", func() { fmt.Println("30 분마다") })

	// @@@@@ 이미 등록된 테스크중에서 바로 실행되어야 하는경우는 어떻게 할 것인가?
	// AddFunc() 함수 2번째 인자로 넘어가는 함수를 저장해뒀다가 다시 Add하고 작업 끝나면 Remove????
	//
	// 함수목록을 관리??(또는 그때그때 함수를 생성해서 고루틴으로 돌려도 됨, 함수생성시 어떤 노티에 보내야 하는지 설정필요)
	// 함수목록을 cron에 등록하여 지정된 시간에 실행
	// 사용자로부터 요청이 들어오면 해당 함수를 찾아서 고루틴으로 실행, waitgroup을 둬서 stop시 실행이 종료될때까지 대기하도록 한다.
	// 스레드 안전하게 테스크를 실행가능하도록 만들어야 한다.
}
