package service

import (
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

type alganicMallTask struct {
	task
}

func newAlganicMallTask(instanceId TaskInstanceId, taskRunData *taskRunData) taskHandler {
	return &alganicMallTask{
		task: task{
			id:         taskRunData.id,
			commandId:  taskRunData.commandId,
			instanceId: instanceId,

			notifierId:  taskRunData.notifierId,
			notifierCtx: taskRunData.notifierCtx,

			cancel: false,
		},
	}
}

func (t *alganicMallTask) Run(sender NotifySender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- TaskInstanceId) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.instanceId
	}()

	switch t.CommandId() {
	case TcidAlganicMallWatchNewEvents:
		t.runWatchNewEvents(sender)

	default:
		// @@@@@ 로그 메시지 출력+notify
		// log.Errorf("등록되지 않은 Task 실행 요청이 수신되었습니다(TaskId:%s, CommandId:%s)", taskRunData.id, taskRunData.commandId)
	}
}

func (t *alganicMallTask) runWatchNewEvents(sender NotifySender) {
	// @@@@@
	for i := 0; i < 5; i++ {
		log.Info("&&&&&&&&&&&&&&&&&&& alganicMallTask running.. ")
		time.Sleep(1 * time.Second)

		if t.cancel == true {
			// 종료처리필요
			break
		}
	}

	if t.cancel == false {
		// notify??
		// @@@@@ 메시지도 수신받아서 notifyserver로 보내기, 이때 유효한 task인지 체크도 함
		//				handler := s.taskHandlers[newId]
		//ctx2 := handler.Context()
		//notifyserverChan<- struct {
		//				message:
		//					ctx : ctx2
		//				}

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
