package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskCommandConfig_EqualsTaskCommandID(t *testing.T) {
	cases := []struct {
		name             string
		configCommandID  CommandID
		compareCommandID CommandID
		expectedResult   bool
		description      string
	}{
		{
			name:             "정확히 일치하는 경우",
			configCommandID:  "WatchPrice",
			compareCommandID: "WatchPrice",
			expectedResult:   true,
			description:      "동일한 CommandID는 true를 반환해야 합니다",
		},
		{
			name:             "일치하지 않는 경우",
			configCommandID:  "WatchPrice",
			compareCommandID: "WatchStock",
			expectedResult:   false,
			description:      "다른 CommandID는 false를 반환해야 합니다",
		},
		{
			name:             "와일드카드 매칭 - 일치",
			configCommandID:  CommandID("WatchPrice_*"),
			compareCommandID: "WatchPrice_Product1",
			expectedResult:   true,
			description:      "와일드카드 패턴과 일치하면 true를 반환해야 합니다",
		},
		{
			name:             "와일드카드 매칭 - 불일치",
			configCommandID:  CommandID("WatchPrice_*"),
			compareCommandID: "WatchStock_Product1",
			expectedResult:   false,
			description:      "와일드카드 패턴과 일치하지 않으면 false를 반환해야 합니다",
		},
		{
			name:             "와일드카드 매칭 - 짧은 입력",
			configCommandID:  CommandID("WatchPrice_*"),
			compareCommandID: "Watch",
			expectedResult:   false,
			description:      "입력이 패턴보다 짧으면 false를 반환해야 합니다",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config := &TaskCommandConfig{
				TaskCommandID: c.configCommandID,
			}

			result := config.equalsTaskCommandID(c.compareCommandID)
			assert.Equal(t, c.expectedResult, result, c.description)
		})
	}
}

func TestFindConfigFromSupportedTask(t *testing.T) {
	// 테스트용 임시 Task 등록
	testTaskID := ID("TEST_TASK_FIND_CONFIG")
	testCommandID := CommandID("TEST_COMMAND")

	originalTasks := supportedTasks
	defer func() {
		// 테스트 후 원래 상태로 복원
		supportedTasks = originalTasks
	}()

	supportedTasks = make(map[ID]*TaskConfig)
	supportedTasks[testTaskID] = &TaskConfig{
		CommandConfigs: []*TaskCommandConfig{
			{
				TaskCommandID:          testCommandID,
				AllowMultipleInstances: true,
				NewTaskResultDataFn:    func() interface{} { return nil },
			},
		},
		NewTaskFn: nil,
	}

	t.Run("존재하는 Task와 Command를 찾는 경우", func(t *testing.T) {
		taskConfig, commandConfig, err := findConfigFromSupportedTask(testTaskID, testCommandID)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.NotNil(t, taskConfig, "Task 설정을 찾아야 합니다")
		assert.NotNil(t, commandConfig, "Command 설정을 찾아야 합니다")
		assert.Equal(t, testCommandID, commandConfig.TaskCommandID, "올바른 Command 설정을 반환해야 합니다")
	})

	t.Run("존재하지 않는 Task를 찾는 경우", func(t *testing.T) {
		taskConfig, commandConfig, err := findConfigFromSupportedTask(ID("NON_EXISTENT"), testCommandID)

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrTaskNotSupported, err, "ErrTaskNotSupported 에러를 반환해야 합니다")
		assert.Nil(t, taskConfig, "Task 설정이 nil이어야 합니다")
		assert.Nil(t, commandConfig, "Command 설정이 nil이어야 합니다")
	})

	t.Run("존재하지 않는 Command를 찾는 경우", func(t *testing.T) {
		taskConfig, commandConfig, err := findConfigFromSupportedTask(testTaskID, CommandID("NON_EXISTENT"))

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrCommandNotSupported, err, "ErrCommandNotSupported 에러를 반환해야 합니다")
		assert.Nil(t, taskConfig, "Task 설정이 nil이어야 합니다")
		assert.Nil(t, commandConfig, "Command 설정이 nil이어야 합니다")
	})
}
