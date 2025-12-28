package lotto

import (
	"regexp"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

var (
	reLogFilePath    = regexp.MustCompile(`경로:(.*\.log)`)
	reAnalysisEnd    = regexp.MustCompile(`로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\((.*)\)`)
	reAnalysisResult = regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`)
	reLottoNum1      = regexp.MustCompile("당첨번호1(.*)")
	reLottoNum2      = regexp.MustCompile("당첨번호2(.*)")
	reLottoNum3      = regexp.MustCompile("당첨번호3(.*)")
	reLottoNum4      = regexp.MustCompile("당첨번호4(.*)")
	reLottoNum5      = regexp.MustCompile("당첨번호5(.*)")
)

// extractLogFilePath 표준 출력(Standard Output)에서 분석 결과 로그 파일의 경로를 추출합니다.
func extractLogFilePath(cmdOutString string) (string, error) {
	// 당첨번호 예측 결과가 저장되어 있는 파일의 경로를 추출한다.
	analysisFilePath := reAnalysisEnd.FindString(cmdOutString)
	if len(analysisFilePath) == 0 {
		return "", apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 작업이 정상적으로 완료되었는지 확인할 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}

	// 정규식 캡처 그룹을 사용하여 경로를 안전하게 추출합니다.
	matches := reLogFilePath.FindStringSubmatch(analysisFilePath)
	if len(matches) < 2 {
		return "", apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과가 저장되어 있는 파일의 경로를 찾을 수 없습니다. 자세한 내용은 로그를 확인하여 주세요")
	}

	return matches[1], nil
}

// parseAnalysisResult 로그 파일 내용에서 예측된 당첨 번호를 추출하여 포맷팅된 문자열로 반환합니다.
func parseAnalysisResult(analysisResultData string) (string, error) {
	// 당첨번호 예측 결과를 추출한다.
	index := strings.Index(analysisResultData, "- 분석결과")
	if index == -1 {
		return "", apperrors.New(apperrors.ExecutionFailed, "당첨번호 예측 결과 파일의 내용이 유효하지 않습니다 (- 분석결과 없음). 자세한 내용은 로그를 확인하여 주세요")
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

	return sb.String(), nil
}
