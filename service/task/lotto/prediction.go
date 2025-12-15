package lotto

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	log "github.com/sirupsen/logrus"
)

func (t *task) executePrediction() (message string, changedTaskResultData interface{}, err error) {
	// 비동기적으로 작업을 시작한다.
	process, err := t.executor.StartCommand("java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", fmt.Sprintf("%s%slottoprediction-1.0.0.jar", t.appPath, string(os.PathSeparator)))
	if err != nil {
		return "", nil, err
	}

	// 일정 시간마다 사용자가 작업을 취소하였는지의 여부를 확인한다.
	ticker := time.NewTicker(time.Millisecond * 500)
	tickerStopC := make(chan bool, 1)

	go func(ticker *time.Ticker, process commandProcess) {
		for {
			select {
			case <-ticker.C:
				if t.IsCanceled() {
					ticker.Stop()
					err0 := process.Kill()
					if err0 != nil {
						applog.WithComponentAndFields("task.lotto", log.Fields{
							"task_id":    t.GetID(),
							"command_id": t.GetCommandID(),
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
		return "", nil, apperrors.New(tasksvc.ErrTaskExecutionFailed, "당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}
	analysisFilePath = regexp.MustCompile(`경로:(.*)\.log`).FindString(analysisFilePath)
	if len(analysisFilePath) == 0 {
		return "", nil, apperrors.New(tasksvc.ErrTaskExecutionFailed, "당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
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
		return "", nil, apperrors.New(tasksvc.ErrTaskExecutionFailed, fmt.Sprintf("당첨번호 예측 결과 파일의 내용이 유효하지 않습니다. 자세한 내용은 로그를 확인하여 주세요.\r\n(%s)", analysisFilePath))
	}
	analysisResultData = analysisResultData[index:]

	message = regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`).FindString(analysisResultData)
	message += "\r\n\r\n"
	message += "• " + strutil.NormalizeSpaces(regexp.MustCompile("당첨번호1(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + strutil.NormalizeSpaces(regexp.MustCompile("당첨번호2(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + strutil.NormalizeSpaces(regexp.MustCompile("당첨번호3(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + strutil.NormalizeSpaces(regexp.MustCompile("당첨번호4(.*)").FindString(analysisResultData)) + "\r\n"
	message += "• " + strutil.NormalizeSpaces(regexp.MustCompile("당첨번호5(.*)").FindString(analysisResultData))

	return message, nil, nil
}
