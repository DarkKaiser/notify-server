package task

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/config"
	"github.com/darkkaiser/notify-server/utils"
	log "github.com/sirupsen/logrus"
)

const (
	// TaskID
	TidLotto TaskID = "LOTTO"

	// TaskCommandID
	TcidLottoPrediction TaskCommandID = "Prediction" // 로또 번호 예측
)

// CommandProcess 실행 중인 프로세스를 추상화하는 인터페이스
type CommandProcess interface {
	Wait() error
	Kill() error
	Output() string
}

// CommandExecutor 외부 명령 실행을 추상화하는 인터페이스
type CommandExecutor interface {
	StartCommand(name string, args ...string) (CommandProcess, error)
}

// defaultCommandProcess exec.Cmd를 래핑한 기본 프로세스 구현
type defaultCommandProcess struct {
	cmd       *exec.Cmd
	outBuffer *bytes.Buffer
}

func (p *defaultCommandProcess) Wait() error {
	return p.cmd.Wait()
}

func (p *defaultCommandProcess) Kill() error {
	return p.cmd.Process.Signal(os.Kill)
}

func (p *defaultCommandProcess) Output() string {
	return p.outBuffer.String()
}

// DefaultCommandExecutor 기본 명령 실행기 (os/exec 사용)
type DefaultCommandExecutor struct{}

func (e *DefaultCommandExecutor) StartCommand(name string, args ...string) (CommandProcess, error) {
	cmd := exec.Command(name, args...)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	return &defaultCommandProcess{
		cmd:       cmd,
		outBuffer: &outBuffer,
	}, nil
}

type lottoTaskData struct {
	AppPath string `json:"app_path"`
}

type lottoPredictionResultData struct{}

func init() {
	supportedTasks[TidLotto] = &supportedTaskConfig{
		commandConfigs: []*supportedTaskCommandConfig{{
			taskCommandID: TcidLottoPrediction,

			allowMultipleInstances: false,

			newTaskResultDataFn: func() interface{} { return &lottoPredictionResultData{} },
		}},

		newTaskFn: func(instanceID TaskInstanceID, taskRunData *taskRunData, appConfig *config.AppConfig) (taskHandler, error) {
			if taskRunData.taskID != TidLotto {
				return nil, errors.New("등록되지 않은 작업입니다.😱")
			}

			var appPath string
			for _, t := range appConfig.Tasks {
				if taskRunData.taskID == TaskID(t.ID) {
					taskData := &lottoTaskData{}
					if err := fillTaskDataFromMap(taskData, t.Data); err != nil {
						return nil, fmt.Errorf("작업 데이터가 유효하지 않습니다.(error:%s)", err)
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

				executor: &DefaultCommandExecutor{},
			}

			task.runFn = func(taskResultData interface{}, _ bool) (string, interface{}, error) {
				switch task.CommandID() {
				case TcidLottoPrediction:
					return task.runPrediction()
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

	executor CommandExecutor
}

func (t *lottoTask) runPrediction() (message string, changedTaskResultData interface{}, err error) {
	// 비동기적으로 작업을 시작한다.
	process, err := t.executor.StartCommand("java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", fmt.Sprintf("%s%slottoprediction-1.0.0.jar", t.appPath, string(os.PathSeparator)))
	if err != nil {
		return "", nil, err
	}

	// 일정 시간마다 사용자가 작업을 취소하였는지의 여부를 확인한다.
	ticker := time.NewTicker(time.Millisecond * 500)
	tickerStopC := make(chan bool, 1)

	go func(ticker *time.Ticker, process CommandProcess) {
		for {
			select {
			case <-ticker.C:
				if t.IsCanceled() {
					ticker.Stop()
					err0 := process.Kill()
					if err0 != nil {
						log.WithFields(log.Fields{
							"task_id":    t.ID(),
							"command_id": t.CommandID(),
							"error":      err0,
						}).Error("작업 취소 중 외부 프로그램 종료 실패")
					}
					return
				}

			case <-tickerStopC:
				ticker.Stop()
				return
			}
		}
	}(ticker, process)

	// 작업이 완료될 때까지 대기한다.
	err = process.Wait()
	tickerStopC <- true

	if err != nil {
		return "", nil, err
	}

	cmdOutString := process.Output()

	// 당첨번호 예측 결과가 저장되어 있는 파일의 경로를 추출한다.
	analysisFilePath := regexp.MustCompile(`로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\((.*)\)`).FindString(cmdOutString)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New("당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}
	analysisFilePath = regexp.MustCompile(`경로:(.*)\.log`).FindString(analysisFilePath)
	if len(analysisFilePath) == 0 {
		return "", nil, errors.New("당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}
	analysisFilePath = string([]rune(analysisFilePath)[3:]) // '경로:' 문자열을 제거한다.

	// 당첨번호 예측 결과 파일을 읽어들인다.
	data, err := os.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, err
	}

	// 당첨번호 예측 결과를 추출한다.
	analysisResultData := string(data)
	index := strings.Index(analysisResultData, "- 분석결과")
	if index == -1 {
		return "", nil, fmt.Errorf("당첨번호 예측 결과 파일의 내용이 유효하지 않습니다. 자세한 내용은 로그를 확인하여 주세요.\r\n(%s)", analysisFilePath)
	}
	analysisResultData = analysisResultData[index:]

	message = regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`).FindString(analysisResultData)
	message += "\r\n\r\n"
	message += "• " + utils.Trim(regexp.MustCompile("당첨번호1(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.Trim(regexp.MustCompile("당첨번호2(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.Trim(regexp.MustCompile("당첨번호3(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.Trim(regexp.MustCompile("당첨번호4(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + utils.Trim(regexp.MustCompile("당첨번호5(.*)").FindString(analysisResultData))

	return message, nil, nil
}
