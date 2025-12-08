package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTask_ReadWriteTaskResultDataFromFile(t *testing.T) {
	t.Run("TaskResultData 파일 읽기/쓰기 테스트", func(t *testing.T) {
		// 테스트용 임시 Task 생성
		testTask := &Task{
			ID:         TaskID("TEST_TASK"),
			CommandID:  TaskCommandID("TEST_COMMAND"),
			InstanceID: TaskInstanceID("test_instance_rw"),
		}

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

		err := testTask.writeTaskResultDataToFile(writeData)
		assert.NoError(t, err, "파일 쓰기가 성공해야 합니다")

		// 읽기 테스트
		readData := &TestResultData{}
		err = testTask.readTaskResultDataFromFile(readData)
		assert.NoError(t, err, "파일 읽기가 성공해야 합니다")

		// 데이터 검증
		assert.Equal(t, writeData.Value, readData.Value, "읽은 데이터의 Value가 일치해야 합니다")
		assert.Equal(t, writeData.Count, readData.Count, "읽은 데이터의 Count가 일치해야 합니다")
	})

	t.Run("존재하지 않는 파일 읽기", func(t *testing.T) {
		testTask := &Task{
			ID:         TaskID("NONEXISTENT_TASK"),
			CommandID:  TaskCommandID("NONEXISTENT_COMMAND"),
			InstanceID: TaskInstanceID("nonexistent_instance"),
		}

		type TestResultData struct {
			Value string `json:"value"`
		}

		readData := &TestResultData{}
		err := testTask.readTaskResultDataFromFile(readData)

		// 파일이 없으면 nil을 반환함 (의도된 동작)
		assert.NoError(t, err, "존재하지 않는 파일 읽기는 nil을 반환해야 합니다")
		// 데이터는 초기값 그대로여야 함
		assert.Equal(t, "", readData.Value, "파일이 없으면 데이터가 변경되지 않아야 합니다")
	})
}

func TestTaskRunBy_Values(t *testing.T) {
	t.Run("TaskRunBy 상수 값 테스트", func(t *testing.T) {
		assert.Equal(t, TaskRunBy(0), TaskRunByUser, "TaskRunByUser는 0이어야 합니다")
		assert.Equal(t, TaskRunBy(1), TaskRunByScheduler, "TaskRunByScheduler는 1이어야 합니다")
	})

	t.Run("TaskRunBy 비교 테스트", func(t *testing.T) {
		testTask := &Task{
			RunBy: TaskRunByUser,
		}

		assert.Equal(t, TaskRunByUser, testTask.RunBy, "Task의 runBy가 TaskRunByUser여야 합니다")
		assert.NotEqual(t, TaskRunByScheduler, testTask.RunBy, "Task의 runBy가 TaskRunByScheduler가 아니어야 합니다")
	})
}
