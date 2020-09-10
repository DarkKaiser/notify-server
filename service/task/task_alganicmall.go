package task

import (
	"context"
	"fmt"
	log "github.com/sirupsen/logrus"
	"time"
)

const (
	// TaskID
	TidAlganicMall TaskID = "ALGANICMALL" // 엘가닉몰(http://www.alganicmall.com/)

	// TaskCommandID
	TcidAlganicMallWatchNewEvents TaskCommandID = "WatchNewEvents" // 엘가닉몰 신규 이벤트 확인
)

func init() {
	supportedTasks[TidAlganicMall] = &supportedTaskData{
		supportedCommands: []TaskCommandID{TcidAlganicMallWatchNewEvents},

		newTaskFunc: func(instanceID TaskInstanceID, taskRunData *taskRunData) taskHandler {
			if taskRunData.taskID != TidAlganicMall {
				return nil
			}

			task := &alganicMallTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					cancel: false,
				},
			}

			task.runFunc = func(taskNotificationSender TaskNotificationSender) {
				switch task.CommandID() {
				case TcidAlganicMallWatchNewEvents:
					task.runWatchNewEvents(taskNotificationSender)

				default:
					taskCtx := context.Background()
					taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskID, task.ID())
					taskCtx = context.WithValue(taskCtx, TaskCtxKeyTaskCommandID, task.CommandID())

					m := fmt.Sprintf("'%s' Task의 '%s' 명령은 등록되지 않았습니다.", task.ID(), task.CommandID())

					log.Error(m)
					taskNotificationSender.Notify(task.NotifierID(), m, taskCtx)
				}
			}

			return task
		},
	}
}

type alganicMallTask struct {
	task
}

func (t *alganicMallTask) runWatchNewEvents(taskNotificationSender TaskNotificationSender) {
	// @@@@@
	for i := 0; i < 500; i++ {
		log.Info("&&&&&&&&&&&&&&&&&&& alganicMallTask running.. ")
		time.Sleep(1 * time.Second)

		if t.cancel == true {
			// 종료처리필요
			log.Info("==============================취소==========================================")
			break
		}
	}

	if t.cancel == true {
		return
	}

	taskNotificationSender.Notify(t.notifierID, "태스크가 완료되었습니다.", nil)
	// notify??
	// @@@@@ 메시지도 수신받아서 notifyserver로 보내기, 이때 유효한 task인지 체크도 함
	//				handler := s.taskHandlers[newId]
	//ctx2 := handler.Context()
	//notifyserverChan<- struct {
	//				message:
	//					ctx : ctx2
	//				}

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
