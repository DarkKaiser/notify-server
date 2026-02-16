package lotto

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

type predictionSnapshot struct {
}

func (t *task) executePrediction(ctx context.Context) (message string, _ any, err error) {
	// 타임아웃(10분) 처리를 위해 상위 Context를 래핑합니다.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	// 외부 프로세스(Java)를 비동기로 실행합니다.
	jarPath := filepath.Join(t.appPath, predictionJarName)
	cmdProcess, err := t.executor.Start(ctx, "java", "-Dfile.encoding=UTF-8", fmt.Sprintf("-Duser.dir=%s", t.appPath), "-jar", jarPath)
	if err != nil {
		return "", nil, err
	}

	// 프로세스 실행이 끝날 때까지 기다립니다.
	err = cmdProcess.Wait()
	if err != nil {
		// 사용자가 작업을 취소하거나 타임아웃이 발생한 경우, 해당 에러를 반환하여 상위 서비스 레이어에서 인지할 수 있도록 합니다.
		if ctx.Err() != nil {
			return "", nil, ctx.Err()
		}

		// 프로세스가 비정상적으로 종료되었거나 실행 중 오류가 발생한 경우, 표준 에러(Stderr) 내용을 로그에 남겨 디버깅을 돕습니다.
		stderr := cmdProcess.Stderr()
		if len(stderr) > 0 {
			t.Log(component, applog.ErrorLevel, "예측 프로세스 실행 실패: 비정상 종료 또는 런타임 오류", err, applog.Fields{
				"stderr": strutil.Truncate(stderr, 10000),
				"dir":    t.appPath,
				"jar":    predictionJarName,
			})

			return "", nil, newErrPredictionFailed(err)
		}

		return "", nil, err
	}

	// 실행 결과(Stdout)에서 예측 결과 파일의 경로를 찾아냅니다.
	resultFilePath, err := extractResultFilePath(cmdProcess.Stdout())
	if err != nil {
		stderr := cmdProcess.Stderr()
		if len(stderr) > 0 {
			t.Log(component, applog.ErrorLevel, "예측 프로세스 출력 분석 실패: 비정상 종료 또는 런타임 오류", err, applog.Fields{
				"stderr": strutil.Truncate(stderr, 10000),
				"dir":    t.appPath,
				"jar":    predictionJarName,
			})

			return "", nil, newErrPredictionFailed(err)
		}

		return "", nil, err
	}

	// 보안 점검: 상위 디렉터리 접근(Path Traversal) 공격 및 심볼릭 링크 우회를 방지하기 위해,
	// 추출된 결과 파일의 실제 물리적 경로가 앱 실행 경로(appPath) 내부에 존재하는지 검증합니다.
	fullPath := resultFilePath
	if !filepath.IsAbs(resultFilePath) {
		fullPath = filepath.Join(t.appPath, resultFilePath)
	}

	// 심볼릭 링크를 해제하여 실제 물리적 경로를 확보합니다.
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		return "", nil, newErrResultFileAbsFailed(err)
	}

	// 절대 경로로 정규화합니다.
	absPath, err := filepath.Abs(realPath)
	if err != nil {
		return "", nil, newErrResultFileAbsFailed(err)
	}

	relPath, err := filepath.Rel(t.appPath, absPath)
	if err != nil {
		return "", nil, newErrResultFileRelFailed(err)
	}
	if strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		t.Log(component, applog.ErrorLevel, "보안 유효성 검사 실패: Path Traversal 감지", nil, applog.Fields{
			"appPath":        t.appPath,
			"absPath":        absPath,
			"resultFilePath": resultFilePath,
		})

		return "", nil, newErrPathTraversalDetected()
	}

	// 보안 검증을 통과한 즉시 삭제 로직을 등록하여, 이후 어떤 단계에서 에러가 발생하더라도
	// 처리가 끝난 임시 파일이 삭제되도록 보장합니다.
	defer func() {
		if err := os.Remove(absPath); err != nil {
			t.Log(component, applog.WarnLevel, "예측 결과 파일 삭제 실패: 시스템 오류", err, applog.Fields{
				"path": absPath,
			})
		}
	}()

	// Context가 취소되거나 타임아웃되지 않았는지 다시 한 번 확인합니다.
	if err := ctx.Err(); err != nil {
		return "", nil, err
	}

	// 예측 결과 파일을 읽습니다. (ToCToU 및 OOM 방지)
	f, err := os.Open(absPath)
	if err != nil {
		return "", nil, newErrReadResultFileFailed(err, filepath.Base(absPath))
	}
	defer f.Close()

	// 파일의 상태를 확인하여 일반 파일인지 검증합니다.
	info, err := f.Stat()
	if err != nil {
		return "", nil, newErrReadResultFileFailed(err, filepath.Base(absPath))
	}
	if !info.Mode().IsRegular() {
		return "", nil, newErrParseResultFileFailed(fmt.Errorf("일반 파일이 아닙니다 (mode: %s)", info.Mode()), filepath.Base(absPath))
	}

	// 파일 크기 검증 (메모리 고갈 방지: 1MB 제한)
	// 입력을 제한된 Reader로 감싸서 물리적으로 읽는 양을 제한합니다.
	limit := int64(1024 * 1024)
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", nil, newErrReadResultFileFailed(err, filepath.Base(absPath))
	}

	if int64(len(data)) > limit {
		return "", nil, newErrParseResultFileFailed(fmt.Errorf("결과 파일 크기가 너무 큽니다 (limit: 1MB)"), filepath.Base(absPath))
	}

	// 파일 내용 일체를 문자열로 변환하고, 사용자에게 보낼 메시지 형식으로 가공합니다.
	message, err = formatAnalysisResult(string(data))
	if err != nil {
		return "", nil, newErrParseResultFileFailed(err, filepath.Base(absPath))
	}

	return message, nil, nil
}
