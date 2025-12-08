package task

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTaskInstanceIDGenerator_New(t *testing.T) {
	generator := &taskInstanceIDGenerator{}

	// 여러 번 호출하여 고유한 ID가 생성되는지 확인
	ids := make(map[TaskInstanceID]bool)
	for i := 0; i < 100; i++ {
		id := generator.New()
		assert.NotEmpty(t, id, "생성된 ID는 비어있지 않아야 합니다")
		assert.False(t, ids[id], "중복된 ID가 생성되지 않아야 합니다")
		ids[id] = true

		// 짧은 대기 시간을 두어 다른 타임스탬프 보장
		time.Sleep(time.Microsecond)
	}
}

func TestTaskInstanceIDGenerator_ToRadixNotation62String(t *testing.T) {
	generator := &taskInstanceIDGenerator{}

	cases := []struct {
		value    int64
		expected string
	}{
		{value: 0, expected: ""},      // 0은 빈 문자열 반환
		{value: -1, expected: ""},     // 음수는 빈 문자열 반환
		{value: 1, expected: "1"},     // 1은 "1"
		{value: 10, expected: "a"},    // 10은 "a"
		{value: 35, expected: "z"},    // 35는 "z"
		{value: 36, expected: "A"},    // 36은 "A"
		{value: 61, expected: "Z"},    // 61은 "Z"
		{value: 62, expected: "10"},   // 62는 "10"
		{value: 123, expected: "1Z"},  // 123은 "1Z"
		{value: 3843, expected: "ZZ"}, // 3843은 "ZZ"
	}

	for _, c := range cases {
		result := generator.toRadixNotation62String(c.value)
		assert.Equal(t, c.expected, result, "value=%d에 대한 62진법 변환 결과가 일치해야 합니다", c.value)
	}
}

func TestTaskInstanceIDGenerator_Reverse(t *testing.T) {
	generator := &taskInstanceIDGenerator{}

	cases := []struct {
		input    []string
		expected []string
	}{
		{input: []string{}, expected: []string{}},
		{input: []string{"a"}, expected: []string{"a"}},
		{input: []string{"a", "b"}, expected: []string{"b", "a"}},
		{input: []string{"a", "b", "c"}, expected: []string{"c", "b", "a"}},
		{input: []string{"1", "2", "3", "4", "5"}, expected: []string{"5", "4", "3", "2", "1"}},
	}

	for _, c := range cases {
		// 원본 배열을 복사하여 사용 (reverse는 원본을 수정함)
		inputCopy := make([]string, len(c.input))
		copy(inputCopy, c.input)

		result := generator.reverse(inputCopy)
		assert.Equal(t, c.expected, result, "배열 역순 결과가 일치해야 합니다")
	}
}

func TestTaskContext_With(t *testing.T) {
	ctx := NewContext()

	// 값 설정
	ctx = ctx.With("key1", "value1")
	ctx = ctx.With("key2", 123)

	// 값 조회
	assert.Equal(t, "value1", ctx.Value("key1"), "설정한 값을 조회할 수 있어야 합니다")
	assert.Equal(t, 123, ctx.Value("key2"), "설정한 값을 조회할 수 있어야 합니다")
	assert.Nil(t, ctx.Value("key3"), "설정하지 않은 키는 nil을 반환해야 합니다")
}

func TestTaskContext_WithTask(t *testing.T) {
	ctx := NewContext()

	taskID := TaskID("TEST_TASK")
	commandID := TaskCommandID("TEST_COMMAND")

	ctx = ctx.WithTask(taskID, commandID)

	assert.Equal(t, taskID, ctx.Value(TaskCtxKeyTaskID), "TaskID가 설정되어야 합니다")
	assert.Equal(t, commandID, ctx.Value(TaskCtxKeyTaskCommandID), "TaskCommandID가 설정되어야 합니다")
}

func TestTaskContext_WithInstanceID(t *testing.T) {
	ctx := NewContext()

	instanceID := TaskInstanceID("test_instance_123")
	elapsedTime := int64(42)

	ctx = ctx.WithInstanceID(instanceID, elapsedTime)

	assert.Equal(t, instanceID, ctx.Value(TaskCtxKeyTaskInstanceID), "TaskInstanceID가 설정되어야 합니다")
	assert.Equal(t, elapsedTime, ctx.Value(TaskCtxKeyElapsedTimeAfterRun), "경과 시간이 설정되어야 합니다")
}

func TestTaskContext_WithError(t *testing.T) {
	ctx := NewContext()

	ctx = ctx.WithError()

	assert.Equal(t, true, ctx.Value(TaskCtxKeyErrorOccurred), "에러 상태가 설정되어야 합니다")
}

func TestTaskCommandConfig_EqualsTaskCommandID(t *testing.T) {
	cases := []struct {
		name             string
		configCommandID  TaskCommandID
		compareCommandID TaskCommandID
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
			configCommandID:  TaskCommandID("WatchPrice_" + taskCommandIDAnyString),
			compareCommandID: "WatchPrice_Product1",
			expectedResult:   true,
			description:      "와일드카드 패턴과 일치하면 true를 반환해야 합니다",
		},
		{
			name:             "와일드카드 매칭 - 불일치",
			configCommandID:  TaskCommandID("WatchPrice_" + taskCommandIDAnyString),
			compareCommandID: "WatchStock_Product1",
			expectedResult:   false,
			description:      "와일드카드 패턴과 일치하지 않으면 false를 반환해야 합니다",
		},
		{
			name:             "와일드카드 매칭 - 짧은 입력",
			configCommandID:  TaskCommandID("WatchPrice_" + taskCommandIDAnyString),
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
	testTaskID := TaskID("TEST_TASK_FIND_CONFIG")
	testCommandID := TaskCommandID("TEST_COMMAND")

	originalTasks := supportedTasks
	defer func() {
		// 테스트 후 원래 상태로 복원
		supportedTasks = originalTasks
	}()

	supportedTasks = make(map[TaskID]*TaskConfig)
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
		taskConfig, commandConfig, err := findConfigFromSupportedTask(TaskID("NON_EXISTENT"), testCommandID)

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrNotSupportedTask, err, "ErrNotSupportedTask 에러를 반환해야 합니다")
		assert.Nil(t, taskConfig, "Task 설정이 nil이어야 합니다")
		assert.Nil(t, commandConfig, "Command 설정이 nil이어야 합니다")
	})

	t.Run("존재하지 않는 Command를 찾는 경우", func(t *testing.T) {
		taskConfig, commandConfig, err := findConfigFromSupportedTask(testTaskID, TaskCommandID("NON_EXISTENT"))

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrNotSupportedCommand, err, "ErrNotSupportedCommand 에러를 반환해야 합니다")
		assert.Nil(t, taskConfig, "Task 설정이 nil이어야 합니다")
		assert.Nil(t, commandConfig, "Command 설정이 nil이어야 합니다")
	})
}

func TestTask_BasicMethods(t *testing.T) {
	testTask := &Task{
		ID:         TaskID("TEST_TASK"),
		CommandID:  TaskCommandID("TEST_COMMAND"),
		InstanceID: TaskInstanceID("test_instance_123"),
		NotifierID: "test_notifier",
		Canceled:   false,
	}

	t.Run("ID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, TaskID("TEST_TASK"), testTask.GetID(), "TaskID가 올바르게 반환되어야 합니다")
	})

	t.Run("CommandID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, TaskCommandID("TEST_COMMAND"), testTask.GetCommandID(), "TaskCommandID가 올바르게 반환되어야 합니다")
	})

	t.Run("InstanceID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, TaskInstanceID("test_instance_123"), testTask.GetInstanceID(), "TaskInstanceID가 올바르게 반환되어야 합니다")
	})

	t.Run("NotifierID 반환 테스트", func(t *testing.T) {
		assert.Equal(t, "test_notifier", testTask.GetNotifierID(), "NotifierID가 올바르게 반환되어야 합니다")
	})

	t.Run("Cancel 및 IsCanceled 테스트", func(t *testing.T) {
		assert.False(t, testTask.IsCanceled(), "초기 상태에서는 취소되지 않아야 합니다")

		testTask.Cancel()
		assert.True(t, testTask.IsCanceled(), "Cancel 호출 후에는 취소 상태여야 합니다")
	})

	t.Run("ElapsedTimeAfterRun 테스트", func(t *testing.T) {
		// runTime을 현재 시간으로 설정
		testTask.RunTime = time.Now()

		// 짧은 대기
		time.Sleep(100 * time.Millisecond)

		elapsed := testTask.ElapsedTimeAfterRun()
		assert.GreaterOrEqual(t, elapsed, int64(0), "경과 시간은 0 이상이어야 합니다")
		assert.LessOrEqual(t, elapsed, int64(2), "경과 시간은 2초 이하여야 합니다")
	})
}
