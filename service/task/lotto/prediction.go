package lotto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
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
	analysisFilePath, err := extractLogFilePath(cmdOutString)
	if err != nil {
		return "", nil, err
	}

	// 당첨번호 예측 결과 파일을 읽어들인다.
	data, err := os.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, err
	}

	// 당첨번호 예측 결과를 추출한다.
	analysisResultData := string(data)
	message, err = parseAnalysisResult(analysisResultData)
	if err != nil {
		return "", nil, fmt.Errorf("%w\r\n(%s)", err, analysisFilePath)
	}

	return message, nil, nil
}
