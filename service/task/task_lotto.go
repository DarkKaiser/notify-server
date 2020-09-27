package task

import (
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
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

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) taskHandler {
			if taskRunData.taskID != TidLotto {
				return nil
			}

			var appPath string
			for _, t := range config.Tasks {
				if taskRunData.taskID == TaskID(t.ID) {
					for _, c := range t.Commands {
						if taskRunData.taskCommandID == TaskCommandID(c.ID) {
							appPath = strings.Trim(c.ReservedData1, " ")
							break
						}
					}
					break
				}
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

				appPath: appPath,
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
	// @@@@@ 결과파일 경로 넘겨주기
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
	message = "종료되었습니다."

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, nil, nil
}
