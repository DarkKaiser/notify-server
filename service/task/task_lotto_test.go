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

// Mock implementations for testing

// MockCommandProcess 테스트용 프로세스 mock
type MockCommandProcess struct {
	waitErr    error
	killErr    error
	output     string
	killCalled bool
}

func (m *MockCommandProcess) Wait() error {
	return m.waitErr
}

func (m *MockCommandProcess) Kill() error {
	m.killCalled = true
	return m.killErr
}

func (m *MockCommandProcess) Output() string {
	return m.output
}

// MockCommandExecutor 테스트용 executor mock
type MockCommandExecutor struct {
	process *MockCommandProcess
	err     error
}

func (m *MockCommandExecutor) StartCommand(name string, args ...string) (CommandProcess, error) {
	return m.process, m.err
}

func TestLottoTask_WithMockExecutor_Success(t *testing.T) {
	t.Run("Mock Executor로 정상 실행 테스트", func(t *testing.T) {
		// 테스트 결과 파일 생성
		tempDir := CreateTestTempDir(t)
		resultPath := filepath.Join(tempDir, "result.log")
		testContent := LoadTestData(t, "lotto/prediction_result.log")
		err := os.WriteFile(resultPath, testContent, 0644)
		assert.NoError(t, err)

		// Mock 출력 생성
		mockOutput := "로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:" + resultPath + ")"

		mockProcess := &MockCommandProcess{
			waitErr: nil,
			output:  mockOutput,
		}

		mockExecutor := &MockCommandExecutor{
			process: mockProcess,
			err:     nil,
		}

		// lottoTask 생성
		task := &lottoTask{
			task: task{
				id:        TidLotto,
				commandID: TcidLottoPrediction,
				canceled:  false,
			},
			appPath:  "/test/path",
			executor: mockExecutor,
		}

		// runPrediction 실행
		message, changedData, err := task.runPrediction()

		assert.NoError(t, err, "정상 실행 시 에러가 없어야 합니다")
		assert.Nil(t, changedData, "changedData는 nil이어야 합니다")
		assert.Contains(t, message, "당첨번호1", "메시지에 당첨번호가 포함되어야 합니다")
		assert.False(t, mockProcess.killCalled, "정상 실행 시 Kill이 호출되지 않아야 합니다")
	})
}

func TestLottoTask_WithMockExecutor_StartCommandError(t *testing.T) {
	t.Run("StartCommand 실패 테스트", func(t *testing.T) {
		mockExecutor := &MockCommandExecutor{
			process: nil,
			err:     assert.AnError,
		}

		task := &lottoTask{
			task: task{
				id:        TidLotto,
				commandID: TcidLottoPrediction,
				canceled:  false,
			},
			appPath:  "/test/path",
			executor: mockExecutor,
		}

		_, _, err := task.runPrediction()

		assert.Error(t, err, "StartCommand 실패 시 에러가 발생해야 합니다")
	})
}

func TestLottoTask_WithMockExecutor_WaitError(t *testing.T) {
	t.Run("Wait 실패 테스트", func(t *testing.T) {
		mockProcess := &MockCommandProcess{
			waitErr: assert.AnError,
			output:  "",
		}

		mockExecutor := &MockCommandExecutor{
			process: mockProcess,
			err:     nil,
		}

		task := &lottoTask{
			task: task{
				id:        TidLotto,
				commandID: TcidLottoPrediction,
				canceled:  false,
			},
			appPath:  "/test/path",
			executor: mockExecutor,
		}

		_, _, err := task.runPrediction()

		assert.Error(t, err, "Wait 실패 시 에러가 발생해야 합니다")
	})
}

func TestLottoTask_WithMockExecutor_InvalidOutput(t *testing.T) {
	t.Run("잘못된 출력 형식 테스트", func(t *testing.T) {
		mockProcess := &MockCommandProcess{
			waitErr: nil,
			output:  "Invalid output without completion message",
		}

		mockExecutor := &MockCommandExecutor{
			process: mockProcess,
			err:     nil,
		}

		task := &lottoTask{
			task: task{
				id:        TidLotto,
				commandID: TcidLottoPrediction,
				canceled:  false,
			},
			appPath:  "/test/path",
			executor: mockExecutor,
		}

		_, _, err := task.runPrediction()

		assert.Error(t, err, "잘못된 출력 형식 시 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "정상적으로 완료되었는지 확인할 수 없습니다", "적절한 에러 메시지가 반환되어야 합니다")
	})
}

func TestDefaultCommandExecutor_RealExecution(t *testing.T) {
	t.Run("DefaultCommandExecutor 실제 실행 테스트", func(t *testing.T) {
		executor := &DefaultCommandExecutor{}

		// Windows에서는 cmd /c echo를 사용
		process, err := executor.StartCommand("cmd", "/c", "echo", "test")

		assert.NoError(t, err, "echo 명령 실행이 성공해야 합니다")
		assert.NotNil(t, process, "프로세스가 생성되어야 합니다")

		err = process.Wait()
		assert.NoError(t, err, "Wait이 성공해야 합니다")

		output := process.Output()
		assert.Contains(t, output, "test", "출력에 'test'가 포함되어야 합니다")
	})
}
