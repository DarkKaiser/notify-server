package task

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/darkkaiser/notify-server/g"
	"github.com/darkkaiser/notify-server/utils"
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

type lottoTaskData struct {
	AppPath string `json:"app_path"`
}

type lottoPredictionResultData struct{}

func init() {
	supportedTasks[TidLotto] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidLottoPrediction,

			allowMultipleIntances: false,

			newTaskResultDataFn: func() interface{} { return &lottoPredictionResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, config *g.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidLotto {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			var appPath string
			for _, t := range config.Tasks {
				if taskRunData.taskID == TaskID(t.ID) {
					taskData := lottoTaskData{}
					if err := convertMapTypeToStructType(t.Data, &taskData); err != nil {
						return nil, errors.New(fmt.Sprintf("작업 데이터가 유효하지 않습니다.(error:%s)", err))
					}

					appPath = strings.Trim(taskData.AppPath, " ")

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

			task.runFn = func(taskResultData interface{}) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidLottoPrediction:
					return task.runPrediction(taskResultData)
				}

				return "", nil, ErrNoImplementationForTaskCommand
			}

			return task, nil
		},
	}
}

type lottoTask struct {
	task

	appPath string
}

//noinspection GoUnusedParameter
func (t *lottoTask) runPrediction(taskResultData interface{}) (message string, changedTaskResultData interface{}, err error) {
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
	tickerStopC := make(chan bool, 1)

	go func(ticker *time.Ticker, cmd *exec.Cmd) {
		for {
			select {
			case <-ticker.C:
				if t.IsCanceled() == true {
					ticker.Stop()
					err0 := cmd.Process.Signal(os.Kill)
					if err0 != nil {
						log.Errorf("사용자 요청으로 작업을 취소하는 중에 실행중인 외부 프로그램의 종료가 실패하였습니다.(error:%s)", err0)
					}
					return
				}

			case <-tickerStopC:
				ticker.Stop()
				return
			}
		}
	}(ticker, cmd)

	// 작업이 완료될 때까지 대기한다.
	err = cmd.Wait()
	if err != nil {
		tickerStopC <- true

		// 작업 진행중에 사용자가 작업을 취소한 경우...
		if t.IsCanceled() == true {
			return "", nil, nil
		}

		return "", nil, err
	} else {
		tickerStopC <- true
	}

	cmdOutString := cmdOutBuffer.String()

	// 당첨번호 예측 결과가 저장되어 있는 파일의 경로를 추출한다.
	analysisFilePath := regexp.MustCompile("로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\\((.*)\\)").FindString(cmdOutString)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New(fmt.Sprint("당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요."))
	}
	analysisFilePath = regexp.MustCompile("경로:(.*)\\.log").FindString(analysisFilePath)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New(fmt.Sprint("당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요."))
	}
	analysisFilePath = string([]rune(analysisFilePath)[3:]) // '경로:' 문자열을 제거한다.

	// 당첨번호 예측 결과 파일을 읽어들인다.
	data, err := ioutil.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, err
	}

	// 당첨번호 예측 결과를 추출한다.
	analysisResultData := string(data)
	index := strings.Index(analysisResultData, "- 분석결과")
	if index == -1 {
		return "", nil, errors.New(fmt.Sprintf("당첨번호 예측 결과 파일의 내용이 유효하지 않습니다. 자세한 내용은 로그를 확인하여 주세요.\r\n(%s)", analysisFilePath))
	}
	analysisResultData = analysisResultData[index:]

	message = regexp.MustCompile("당첨 확률이 높은 당첨번호 목록\\([0-9]+개\\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.").FindString(analysisResultData)
	message += "\r\n\r\n"
	message += "• " + utils.CleanString(regexp.MustCompile("당첨번호1(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.CleanString(regexp.MustCompile("당첨번호2(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.CleanString(regexp.MustCompile("당첨번호3(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.CleanString(regexp.MustCompile("당첨번호4(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.CleanString(regexp.MustCompile("당첨번호5(.*)").FindString(analysisResultData))

	if t.IsCanceled() == true {
		return "", nil, nil
	}

	return message, nil, nil
}
