package lotto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

func (t *task) executePrediction() (message string, _ interface{}, err error) {
	// 별도의 Context 생성을 통해 타임아웃(10분)과 취소 처리를 통합 관리합니다.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Task 취소 플래그(t.canceled)를 주기적으로 확인하여 Context를 취소합니다.
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

	jarPath := filepath.Join(t.appPath, lottoJarFileName)

	// 비동기적으로 작업을 시작한다
	process, err := t.executor.StartCommand(ctx, "java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", jarPath)
	if err != nil {
		return "", nil, err
	}

	// 작업이 완료(종료)될 때까지 대기한다.
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

	stdout := process.Stdout()

	// 당첨번호 예측 결과가 저장되어 있는 파일의 경로를 추출한다.
	analysisFilePath, err := extractLogFilePath(stdout)
	if err != nil {
		stderr := process.Stderr()
		if len(stderr) > 0 {
			return "", nil, fmt.Errorf("%w\n(Stderr: %s)", err, stderr)
		}
		return "", nil, err
	}

	// 추출된 파일 경로가 반드시 appPath 내부에 있어야 합니다 (Path Traversal 방지)
	// analysisFilePath는 절대 경로일 수도 있고 상대 경로일 수도 있습니다.
	absAnalysisPath, err := filepath.Abs(analysisFilePath)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "분석 결과 파일의 절대 경로를 확인(Resolve)하는 도중 시스템 오류가 발생했습니다")
	}

	relPath, err := filepath.Rel(t.appPath, absAnalysisPath)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, "파일 경로의 유효성을 검증하는 도중 오류가 발생했습니다")
	}
	if strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return "", nil, apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("보안 정책 위반: 허용된 경로 범위를 벗어난 파일 접근이 감지되었습니다 (Path Traversal 시도 의심: %s)", analysisFilePath))
	}

	analysisFilePath = absAnalysisPath
	defer func() {
		// 분석이 끝난 로그 파일은 삭제한다.
		if err := os.Remove(analysisFilePath); err != nil {
			applog.WithComponentAndFields("task.lotto", log.Fields{
				"task_id":    t.GetID(),
				"command_id": t.GetCommandID(),
				"path":       analysisFilePath,
				"error":      err,
			}).Warn("분석 결과 임시 파일 삭제 실패")
		}
	}()

	// 당첨번호 예측 결과 파일을 읽어들인다.
	data, err := os.ReadFile(analysisFilePath)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("예측 결과 파일(%s)을 읽는 도중 I/O 오류가 발생했습니다", filepath.Base(analysisFilePath)))
	}

	// 당첨번호 예측 결과를 추출한다.
	analysisResultData := string(data)
	message, err = parseAnalysisResult(analysisResultData)
	if err != nil {
		return "", nil, apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("예측 결과 파일(%s)의 내용을 파싱하는 도중 오류가 발생했습니다", filepath.Base(analysisFilePath)))
	}

	return message, nil, nil
}
