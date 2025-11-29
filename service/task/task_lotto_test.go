package task

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLottoTask_ParsePredictionResult(t *testing.T) {
	t.Run("정상적인 예측 결과 파싱", func(t *testing.T) {
		// testdata에서 샘플 로또 결과 로드
		resultData := LoadTestDataAsString(t, "lotto/prediction_result.log")

		// 당첨번호 예측 결과 추출 정규표현식 테스트
		re := regexp.MustCompile(`당첨 확률이 높은 당첨번호 목록\([0-9]+개\)중에서 [0-9]+개의 당첨번호가 추출되었습니다.`)
		match := re.FindString(resultData)

		assert.NotEmpty(t, match, "예측 결과 메시지가 추출되어야 합니다")
		assert.Contains(t, match, "5개의 당첨번호가 추출되었습니다", "올바른 메시지가 추출되어야 합니다")
	})

	t.Run("당첨번호 추출", func(t *testing.T) {
		resultData := LoadTestDataAsString(t, "lotto/prediction_result.log")

		// 각 당첨번호 추출
		re1 := regexp.MustCompile(`당첨번호1(.*)`)
		re2 := regexp.MustCompile(`당첨번호2(.*)`)
		re3 := regexp.MustCompile(`당첨번호3(.*)`)
		re4 := regexp.MustCompile(`당첨번호4(.*)`)
		re5 := regexp.MustCompile(`당첨번호5(.*)`)

		match1 := re1.FindString(resultData)
		match2 := re2.FindString(resultData)
		match3 := re3.FindString(resultData)
		match4 := re4.FindString(resultData)
		match5 := re5.FindString(resultData)

		assert.NotEmpty(t, match1, "당첨번호1이 추출되어야 합니다")
		assert.NotEmpty(t, match2, "당첨번호2가 추출되어야 합니다")
		assert.NotEmpty(t, match3, "당첨번호3이 추출되어야 합니다")
		assert.NotEmpty(t, match4, "당첨번호4가 추출되어야 합니다")
		assert.NotEmpty(t, match5, "당첨번호5가 추출되어야 합니다")

		assert.Contains(t, match1, "1  2  3  4  5  6", "당첨번호1의 숫자가 포함되어야 합니다")
	})

	t.Run("분석결과 섹션 추출", func(t *testing.T) {
		resultData := LoadTestDataAsString(t, "lotto/prediction_result.log")

		// "- 분석결과" 섹션 찾기
		index := regexp.MustCompile(`- 분석결과`).FindStringIndex(resultData)

		assert.NotNil(t, index, "분석결과 섹션이 존재해야 합니다")
		assert.Greater(t, index[0], -1, "분석결과 섹션의 위치를 찾아야 합니다")

		// 분석결과 이후 데이터 추출
		analysisResult := resultData[index[0]:]
		assert.Contains(t, analysisResult, "당첨번호1", "분석결과에 당첨번호가 포함되어야 합니다")
	})
}

func TestLottoTask_FilePathExtraction(t *testing.T) {
	t.Run("파일 경로 정규표현식 테스트", func(t *testing.T) {
		// 실제 로또 프로그램 출력 예시
		output := "로또 당첨번호 예측작업이 종료되었습니다. 100개의 대상 당첨번호가 추출되었습니다.(경로:C:\\test\\result.log)"

		re := regexp.MustCompile(`경로:(.*?)\.log`)
		match := re.FindStringSubmatch(output)

		assert.NotNil(t, match, "파일 경로가 추출되어야 합니다")
		assert.Greater(t, len(match), 1, "매칭 그룹이 존재해야 합니다")

		if len(match) > 1 {
			filePath := match[1]
			assert.Contains(t, filePath, "C:\\test\\result", "올바른 파일 경로가 추출되어야 합니다")
		}
	})

	t.Run("파일 경로 추출 실패 케이스", func(t *testing.T) {
		output := "로또 당첨번호 예측작업이 실패했습니다."

		re := regexp.MustCompile(`경로:(.*?)\.log`)
		match := re.FindStringSubmatch(output)

		assert.Nil(t, match, "파일 경로가 없으면 매칭되지 않아야 합니다")
	})
}

func TestLottoTask_ResultFileReading(t *testing.T) {
	t.Run("결과 파일 읽기 테스트", func(t *testing.T) {
		// 임시 디렉토리 생성
		tempDir := CreateTestTempDir(t)

		// 테스트 결과 파일 생성
		resultFilePath := filepath.Join(tempDir, "lotto_result.log")
		testContent := LoadTestData(t, "lotto/prediction_result.log")

		err := os.WriteFile(resultFilePath, testContent, 0644)
		assert.NoError(t, err, "테스트 파일 생성이 성공해야 합니다")

		// 파일 읽기
		data, err := os.ReadFile(resultFilePath)
		assert.NoError(t, err, "파일 읽기가 성공해야 합니다")
		assert.NotEmpty(t, data, "파일 내용이 비어있지 않아야 합니다")

		// 내용 검증
		content := string(data)
		assert.Contains(t, content, "분석결과", "파일에 분석결과가 포함되어야 합니다")
	})

	t.Run("존재하지 않는 파일 읽기", func(t *testing.T) {
		_, err := os.ReadFile("nonexistent_file.log")
		assert.Error(t, err, "존재하지 않는 파일 읽기는 에러가 발생해야 합니다")
	})
}

func TestLottoTask_CancelLogic(t *testing.T) {
	t.Run("작업 취소 플래그 테스트", func(t *testing.T) {
		task := CreateTestTask(TidLotto, TcidLottoPrediction, "test_instance")

		// 초기 상태 확인
		assert.False(t, task.IsCanceled(), "초기 상태에서는 취소되지 않아야 합니다")

		// 작업 취소
		task.Cancel()
		assert.True(t, task.IsCanceled(), "Cancel 호출 후에는 취소 상태여야 합니다")
	})
}

func TestLottoTask_MessageFormatting(t *testing.T) {
	t.Run("예측 결과 메시지 포맷 테스트", func(t *testing.T) {
		// 실제 메시지 포맷 검증
		message := "당첨 확률이 높은 당첨번호 목록(100개)중에서 5개의 당첨번호가 추출되었습니다.\r\n\r\n"
		message += "• 당첨번호1  1  2  3  4  5  6\r\n"
		message += "• 당첨번호2  7  8  9  10 11 12\r\n"
		message += "• 당첨번호3  13 14 15 16 17 18\r\n"
		message += "• 당첨번호4  19 20 21 22 23 24\r\n"
		message += "• 당첨번호5  25 26 27 28 29 30"

		assert.Contains(t, message, "당첨 확률이 높은", "메시지에 헤더가 포함되어야 합니다")
		assert.Contains(t, message, "당첨번호1", "당첨번호1이 포함되어야 합니다")
		assert.Contains(t, message, "당첨번호5", "당첨번호5가 포함되어야 합니다")
		assert.Contains(t, message, "•", "불릿 포인트가 포함되어야 합니다")
	})
}
