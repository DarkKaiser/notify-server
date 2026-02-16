package lotto

import (
	"regexp"
	"slices"
	"strings"

	"github.com/darkkaiser/notify-server/pkg/strutil"
)

var (
	reResultFilePath        = regexp.MustCompile(`경로:(.*\.log)`)
	rePredictionCompleteMsg = regexp.MustCompile(`로또 당첨번호 예측작업이 종료되었습니다. [0-9]+개의 대상 당첨번호가 추출되었습니다.\((.*)\)`)
	reResultHeader          = regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`)
	reWinningNum1           = regexp.MustCompile("당첨번호1(.*)")
	reWinningNum2           = regexp.MustCompile("당첨번호2(.*)")
	reWinningNum3           = regexp.MustCompile("당첨번호3(.*)")
	reWinningNum4           = regexp.MustCompile("당첨번호4(.*)")
	reWinningNum5           = regexp.MustCompile("당첨번호5(.*)")
)

// extractResultFilePath 외부 명령어 실행 결과(표준 출력)에서 '예측 결과가 저장된 파일의 절대 경로'를 파싱하여 반환합니다.
//
// 파라미터:
//   - output: 외부 명령어의 표준 출력(Stdout) 문자열
//
// 반환값:
//   - string: 예측 결과가 저장된 파일의 절대 경로
//   - error: 오류 발생 시 발생한 에러
func extractResultFilePath(output string) (string, error) {
	// 표준 출력에서 예측 작업 완료 메시지 라인을 찾는다.
	resultLine := rePredictionCompleteMsg.FindString(output)
	if len(resultLine) == 0 {
		return "", ErrPredictionCompleteMsgNotFound
	}

	// 완료 메시지 내에서 정규식 그룹으로 예측 결과 파일 경로만 추출한다.
	matches := reResultFilePath.FindStringSubmatch(resultLine)
	if len(matches) < 2 {
		return "", ErrResultFilePathNotFound
	}

	return strings.TrimSpace(matches[1]), nil
}

// formatAnalysisResult 예측 결과 파일의 전체 내용에서 당첨 번호 목록을 추출하고, 사용자가 읽기 좋은 형태로 포맷팅하여 반환합니다.
//
// 파라미터:
//   - content: 예측 결과 파일에서 읽어온 원본 문자열
//
// 반환값:
//   - string: 사용자가 읽기 좋은 형태로 포맷팅된 당첨 번호 목록
//   - error: 오류 발생 시 발생한 에러
func formatAnalysisResult(content string) (string, error) {
	// 당첨번호 예측 결과 섹션을 추출한다.
	index := strings.Index(content, "- 분석결과")
	if index == -1 {
		return "", ErrAnalysisResultInvalid
	}
	content = content[index:]
	if len(content) <= len("- 분석결과") {
		return "", ErrAnalysisResultInvalid
	}

	// 필수 정보 파싱 및 검증
	header := reResultHeader.FindString(content)
	if header == "" {
		return "", ErrAnalysisResultInvalid
	}

	winningNums := []string{
		strutil.NormalizeSpace(reWinningNum1.FindString(content)),
		strutil.NormalizeSpace(reWinningNum2.FindString(content)),
		strutil.NormalizeSpace(reWinningNum3.FindString(content)),
		strutil.NormalizeSpace(reWinningNum4.FindString(content)),
		strutil.NormalizeSpace(reWinningNum5.FindString(content)),
	}

	// 모든 당첨번호가 정상적으로 추출되었는지 확인
	if slices.Contains(winningNums, "") {
		return "", ErrAnalysisResultInvalid
	}

	// 당첨번호 목록을 포맷팅한다.
	var sb strings.Builder
	sb.WriteString(header)
	sb.WriteString("\r\n\r\n")

	for i, num := range winningNums {
		sb.WriteString("• " + num)

		if i < len(winningNums)-1 {
			sb.WriteString("\r\n")
		}
	}

	return sb.String(), nil
}
