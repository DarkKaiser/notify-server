package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const testAppName = "test-app"

func TestFileTaskResultStorage_LoadSave(t *testing.T) {
	t.Run("TaskResultData 파일 읽기/쓰기 테스트", func(t *testing.T) {
		storage := NewFileTaskResultStorage(testAppName)

		// 테스트 격리를 위해 임시 디렉토리 사용
		tempDir := t.TempDir()
		storage.SetBaseDir(tempDir)

		taskID := ID("TEST_TASK")
		commandID := CommandID("TEST_COMMAND")

		// 테스트용 데이터 구조
		type TestResultData struct {
			Value string `json:"value"`
			Count int    `json:"count"`
		}

		// 쓰기 테스트
		writeData := &TestResultData{
			Value: "test_value",
			Count: 42,
		}

		err := storage.Save(taskID, commandID, writeData)
		assert.NoError(t, err, "파일 쓰기가 성공해야 합니다")

		// 읽기 테스트
		readData := &TestResultData{}
		err = storage.Load(taskID, commandID, readData)
		assert.NoError(t, err, "파일 읽기가 성공해야 합니다")

		// 데이터 검증
		assert.Equal(t, writeData.Value, readData.Value, "읽은 데이터의 Value가 일치해야 합니다")
		assert.Equal(t, writeData.Count, readData.Count, "읽은 데이터의 Count가 일치해야 합니다")
	})

	t.Run("존재하지 않는 파일 읽기", func(t *testing.T) {
		storage := NewFileTaskResultStorage(testAppName)

		// 테스트 격리를 위해 임시 디렉토리 사용
		tempDir := t.TempDir()
		storage.SetBaseDir(tempDir)

		taskID := ID("NONEXISTENT_TASK")
		commandID := CommandID("NONEXISTENT_COMMAND")

		type TestResultData struct {
			Value string `json:"value"`
		}

		readData := &TestResultData{}
		err := storage.Load(taskID, commandID, readData)

		// 파일이 없으면 nil을 반환함 (의도된 동작)
		assert.NoError(t, err, "존재하지 않는 파일 읽기는 nil을 반환해야 합니다")
		// 데이터는 초기값 그대로여야 함
		assert.Equal(t, "", readData.Value, "파일이 없으면 데이터가 변경되지 않아야 합니다")
	})
}
