package lotto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	log "github.com/sirupsen/logrus"
)

var (
	reLottoAnalysisEnd = regexp.MustCompile(`로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\((.*)\)`)
	reLogFilePath      = regexp.MustCompile(`경로:(.*\.log)`)
	reAnalysisResult   = regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`)
	reLottoNum1        = regexp.MustCompile("당첨번호1(.*)")
	reLottoNum2        = regexp.MustCompile("당첨번호2(.*)")
	reLottoNum3        = regexp.MustCompile("당첨번호3(.*)")
	reLottoNum4        = regexp.MustCompile("당첨번호4(.*)")
	reLottoNum5        = regexp.MustCompile("당첨번호5(.*)")
)

func (t *task) executePrediction() (message string, changedTaskResultData interface{}, err error) {
	// 별도의 Context 생성을 통해 타임아웃(10분)과 취소 처리를 통합 관리합니다.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// 백그라운드 고루틴: Task 취소 플래그(t.canceled)를 주기적으로 확인하여 Context를 취소합니다.
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if t.IsCanceled() {
					cancel()
					return
				}
			}
		}
	}()

	// 안전한 경로 생성 (filepath.Join)
	jarPath := filepath.Join(t.appPath, "lottoprediction-1.0.0.jar")

	// 비동기적으로 작업을 시작한다 (Context 전달).
	process, err := t.executor.StartCommand(ctx, "java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", jarPath)
	if err != nil {
		return "", nil, err
	}

	// 작업이 완료될 때까지 대기한다.
	err = process.Wait()
	if err != nil {
		// 취소된 경우 에러가 아님 (조용한 종료)
		if ctx.Err() == context.Canceled {
			return "", nil, nil
		}

		// 에러 발생 시 Stderr 내용을 포함하여 로깅합니다.
		stderr := process.Stderr()
		if len(stderr) > 0 {
			applog.WithComponentAndFields("task.lotto", log.Fields{
				"task_id":    t.GetID(),
				"command_id": t.GetCommandID(),
				"stderr":     stderr,
			}).Error("외부 프로세스 실행 중 에러 발생")
		}
		return "", nil, err
	}

	cmdOutString := process.Output()

	// 당첨번호 예측 결과가 저장되어 있는 파일의 경로를 추출한다.
	analysisFilePath := reLottoAnalysisEnd.FindString(cmdOutString)
	if len(analysisFilePath) == 0 {
		return "", nil, apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}

	// 정규식 캡처 그룹을 사용하여 경로를 안전하게 추출합니다.
	matches := reLogFilePath.FindStringSubmatch(analysisFilePath)
	if len(matches) < 2 {
		return "", nil, apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}
	analysisFilePath = matches[1]

	// 당첨번호 예측 결과 파일을 읽어들인다.
	data, err := os.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, err
	}

	// 당첨번호 예측 결과를 추출한다.
	analysisResultData := string(data)
	index := strings.Index(analysisResultData, "- 분석결과")
	if index == -1 {
		return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("당첨번호 예측 결과 파일의 내용이 유효하지 않습니다. 자세한 내용은 로그를 확인하여 주세요.\r\n(%s)", analysisFilePath))
	}
	analysisResultData = analysisResultData[index:]

	var sb strings.Builder
	sb.WriteString(reAnalysisResult.FindString(analysisResultData))
	sb.WriteString("\r\n\r\n")
	sb.WriteString("• " + strutil.NormalizeSpaces(reLottoNum1.FindString(analysisResultData)) + "\r\n")
	sb.WriteString("• " + strutil.NormalizeSpaces(reLottoNum2.FindString(analysisResultData)) + "\r\n")
	sb.WriteString("• " + strutil.NormalizeSpaces(reLottoNum3.FindString(analysisResultData)) + "\r\n")
	sb.WriteString("• " + strutil.NormalizeSpaces(reLottoNum4.FindString(analysisResultData)) + "\r\n")
	sb.WriteString("• " + strutil.NormalizeSpaces(reLottoNum5.FindString(analysisResultData)))

	return sb.String(), nil, nil
}
