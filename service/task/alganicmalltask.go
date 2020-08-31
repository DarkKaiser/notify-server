package task

import (
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type alganicMallTask struct {
	task
}

func newAlganicMallTask(instanceId TaskInstanceId, taskRunData *taskRunData) taskHandler {
	task := &alganicMallTask{
		task: task{
			id:         taskRunData.id,
			commandId:  taskRunData.commandId,
			instanceId: instanceId,

			cancel: false,

			ctx: taskRunData.ctx,
		},
	}

	return task
}

func (t *alganicMallTask) Run(taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceId) {
	defer taskStopWaiter.Done()

	switch t.CommandId() {
	case TcidAlganicMallWatchNewEvents:
		t.runWatchNewEvents()

	default:
		// @@@@@ 로그 메시지 출력+notify
	}

	// @@@@@ cancel된것도 보내나?

	taskDoneC <- t.instanceId
}

func (t *alganicMallTask) runWatchNewEvents() {
	for i := 0; i < 10; i++ {
		log.Info("&&&&&&&&&&&&&&&&&&& alganicMallTask running.. ")
		time.Sleep(1 * time.Second)

		if t.cancel == true {
			// 종료처리필요
			break
		}
	}

	if t.cancel == false {
		// notify??
	}

	// 웹 크롤링해서 이벤트를 로드하고 Noti로 알린다.
	// 각각의 데이터는 data.xxx.json 파일로 관리한다.
	// 데이터파일에서 어떤 노티에 보내야하는지를 설정한다.(없으면 모두, 있으면 해당 노티로 보낸다, 지금은 1개)
	// 만약 사용자가 직접 요청해서 실행된 결과라면 요청한 노티로 보내야 한다.

	// 로또번호구하기 : 타 프로그램 실행 후 결과 받기
	// - 이미 실행된 프로그램? 아니면 새로 시작할것인가?
	// > 이미 실행된 프로그램 XXX
	//	 프로그램을 찾아서 메시지를 넘겨서 결과를 전송받아야 한다.
	// > 새로 시작
	//	 프로그램 시작시 메시지를 같이 넘기고 그 결과를 전송받아야 한다.
	//	 결과는 프로그램 콘솔에 찍힌걸 읽어올수 있으면 이걸 사용
	//	 안되면 결과파일을 지정해서 넘겨주고 종료시 이 결과파일을 분석

}
