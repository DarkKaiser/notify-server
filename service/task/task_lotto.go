package task

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"time"
)

const (
	// TaskID
	TidLotto TaskID = "LOTTO"

	// TaskCommandID
	TcidLottoPrediction TaskCommandID = "Prediction" // 로또 번호 예측
)

type lottoPredictionData struct{}

func init() {
	supportedTasks[TidLotto] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidLottoPrediction,

			allowMultipleIntances: false,

			newTaskDataFn: func() interface{} { return &lottoPredictionData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData) taskHandler {
			if taskRunData.taskID != TidLotto {
				return nil
			}

			task := &lottoTask{
				task: task{
					id:         taskRunData.taskID,
					commandID:  taskRunData.taskCommandID,
					instanceID: instanceID,

					notifierID: taskRunData.notifierID,

					canceled: false,

					runBy: taskRunData.taskRunBy,
				},

				appPath: "e:\\1", //@@@@@
			}

			task.runFn = func(taskData interface{}) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidLottoPrediction:
					return task.runPrediction(taskData)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task
		},
	}
}

type lottoTask struct {
	task

	appPath string
}

func (t *lottoTask) runPrediction(taskData interface{}) (message string, changedTaskData interface{}, err error) {
	cmd := exec.Command("java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", fmt.Sprintf("%s%slottoprediction-1.0.0.jar", t.appPath, string(os.PathSeparator)))

	// 비동기적으로 작업을 시작한다.
	err = cmd.Start()
	if err != nil {
		return "", nil, err
	}

	// 일정 시간마다 사용자가 작업을 취소하였는지의 여부를 확인한다.
	ticker := time.NewTicker(time.Millisecond * 500)
	defer ticker.Stop()

	go func(ticker *time.Ticker, cmd *exec.Cmd) {
		for range ticker.C {
			if t.IsCanceled() == true {
				err0 := cmd.Process.Signal(os.Kill)
				if err0 != nil {
					log.Errorf("사용자 요청으로 작업을 취소하는 중에 실행중인 외부 프로그램의 종료가 실패하였습니다.(error:%s)", err0)
				}
				break
			}
		}
	}(ticker, cmd)

	// 작업이 완료될 때까지 대기한다.
	err = cmd.Wait()
	if err != nil {
		// 작업 진행중에 사용자가 작업을 취소한 경우...
		if t.IsCanceled() == true {
			return "", nil, nil
		}

		return "", nil, err
	}

	// 작업 결과를 받아온다.
	// @@@@@
	//	s := cmdOutBuffer.String()
	//	println(s)
	message = "종료되었습니다."

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, nil, nil
}
