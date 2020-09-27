package task

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
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
	cmd := exec.Command("java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", fmt.Sprintf("%s%slottoprediction-1.0.0.jar", t.appPath, string(os.PathSeparator)))

	var cmdOutBuffer bytes.Buffer
	cmd.Stdout = &cmdOutBuffer

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

	// @@@@@
	///////////////////////////////////
	// 작업 결과를 받아온다.
	// 로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(최대 당첨가능 확률값:0.1, 경로:F:\Solutions-Private\lotto-prediction\dist\prediction-logs\931_20200927_233542591_predict.log)
	cmdOutString := cmdOutBuffer.String()
	analysisFilePath := regexp.MustCompile("로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\\((.*)\\)").FindString(cmdOutString)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New("")
	}
	analysisFilePath = regexp.MustCompile("경로:(.*)\\.log").FindString(analysisFilePath)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New("")
	}
	analysisFilePath = analysisFilePath[len("경로:"):]

	// 추출된 당첨번호 정보를 읽어들인다.
	data, err := ioutil.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, err
	}

	analysisResultData := string(data)
	index := strings.Index(analysisResultData, "- 분석결과")
	if index == -1 {
		return "", nil, errors.New(fmt.Sprintf("%s", analysisFilePath)) //@@@@@
	}
	analysisResultData = analysisResultData[index:]

	message = regexp.MustCompile("당첨 확률이 높은 당첨번호 목록\\([0-9]+개\\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.").FindString(analysisResultData)
	message += "\r\n\r\n"
	message += "- " + regexp.MustCompile("당첨번호1(.*)").FindString(analysisResultData) + "\r\n"
	message += "- " + regexp.MustCompile("당첨번호2(.*)").FindString(analysisResultData) + "\r\n"
	message += "- " + regexp.MustCompile("당첨번호3(.*)").FindString(analysisResultData) + "\r\n"
	message += "- " + regexp.MustCompile("당첨번호4(.*)").FindString(analysisResultData) + "\r\n"
	message += "- " + regexp.MustCompile("당첨번호5(.*)").FindString(analysisResultData)
	//////////////////////////////////

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, nil, nil
}
