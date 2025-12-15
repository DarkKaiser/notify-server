package task

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/config"
	"github.com/stretchr/testify/assert"
)

// 헬퍼 함수: 더미 NewTaskFunc 생성
func dummyNewTask() NewTaskFunc {
	return func(InstanceID, *SubmitRequest, *config.AppConfig) (Handler, error) {
		return nil, nil
	}
}

// 헬퍼 함수: 더미 NewSnapshotFunc 생성
func dummyResultFn() NewSnapshotFunc {
	return func() interface{} { return struct{}{} }
}

func TestCommandConfig_EqualsCommandID(t *testing.T) {
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
			config := &CommandConfig{
				ID: c.configCommandID,
			}

			result := config.ID.Match(c.compareCommandID)
			assert.Equal(t, c.expectedResult, result, c.description)
		})
	}
}

func TestFindConfig(t *testing.T) {
	// 테스트용 임시 Task 등록
	testTaskID := ID("TEST_TASK_FIND_CONFIG")
	testCommandID := CommandID("TEST_COMMAND")

	// 독립적인 레지스트리 인스턴스 생성
	r := newRegistry()

	r.registerForTest(testTaskID, &Config{
		Commands: []*CommandConfig{
			{
				ID:            testCommandID,
				AllowMultiple: true,
				NewSnapshot:   dummyResultFn(),
			},
		},
		NewTask: nil,
	})

	t.Run("존재하는 Task와 Command를 찾는 경우", func(t *testing.T) {
		searchResult, err := r.findConfig(testTaskID, testCommandID)

		assert.NoError(t, err, "에러가 발생하지 않아야 합니다")
		assert.NotNil(t, searchResult, "검색 결과는 nil이 아니어야 합니다")
		assert.NotNil(t, searchResult.Task, "Task 설정을 찾아야 합니다")
		assert.NotNil(t, searchResult.Command, "Command 설정을 찾아야 합니다")
		assert.Equal(t, testCommandID, searchResult.Command.ID, "올바른 Command 설정을 반환해야 합니다")
	})

	t.Run("존재하지 않는 Task를 찾는 경우", func(t *testing.T) {
		searchResult, err := r.findConfig(ID("NON_EXISTENT"), testCommandID)

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrTaskNotSupported, err, "ErrTaskNotSupported 에러를 반환해야 합니다")
		assert.Nil(t, searchResult, "검색 결과는 nil이어야 합니다")
	})

	t.Run("존재하지 않는 Command를 찾는 경우", func(t *testing.T) {
		searchResult, err := r.findConfig(testTaskID, CommandID("NON_EXISTENT"))

		assert.Error(t, err, "에러가 발생해야 합니다")
		assert.Equal(t, ErrCommandNotSupported, err, "ErrCommandNotSupported 에러를 반환해야 합니다")
		assert.Nil(t, searchResult, "검색 결과는 nil이어야 합니다")
	})
}

func TestRegistry_Register_Validation(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedPanic string
	}{
		{
			name:          "Config is nil",
			config:        nil,
			expectedPanic: "태스크 설정(config)은 nil일 수 없습니다",
		},
		{
			name: "NewTask is nil",
			config: &Config{
				NewTask: nil,
				Commands: []*CommandConfig{
					{
						ID:            "DummyCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedPanic: "NewTask는 nil일 수 없습니다",
		},
		{
			name: "CommandConfigs is empty",
			config: &Config{
				NewTask:  dummyNewTask(),
				Commands: []*CommandConfig{},
			},
			expectedPanic: "Commands는 비어있을 수 없습니다",
		},
		{
			name: "CommandID is empty",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedPanic: "CommandID는 비어있을 수 없습니다",
		},
		{
			name: "NewSnapshot is nil",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "SafeCommand",
						AllowMultiple: true,
						// NewSnapshot missing
					},
				},
			},
			expectedPanic: "NewSnapshot은 nil일 수 없습니다",
		},
		{
			name: "Duplicate CommandID",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "DuplicateCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
					{
						ID:            "DuplicateCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedPanic: "중복된 CommandID입니다: DuplicateCommand",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newRegistry()
			assert.PanicsWithValue(t, tt.expectedPanic, func() {
				r.Register("INVALID_TASK", tt.config)
			})
		})
	}

	// Duplicate TaskID 테스트 (별도 처리 필요)
	t.Run("Duplicate TaskID", func(t *testing.T) {
		taskID := ID("DUPLICATE_TASK_ID")
		r := newRegistry()

		// 먼저 정상 등록
		r.Register(taskID, &Config{
			NewTask: dummyNewTask(),
			Commands: []*CommandConfig{
				{
					ID:            "SomeCommand",
					AllowMultiple: true,
					NewSnapshot:   dummyResultFn(),
				},
			},
		})

		// 동일 ID로 재등록 시 패닉 발생 확인
		assert.PanicsWithValue(t, fmt.Sprintf("중복된 TaskID입니다: %s", taskID), func() {
			r.Register(taskID, &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "OtherCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			})
		})
	})
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectedError string
	}{
		{
			name: "NewTask is nil",
			config: &Config{
				NewTask: nil,
				Commands: []*CommandConfig{
					{
						ID:            "DummyCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedError: "NewTask는 nil일 수 없습니다",
		},
		{
			name: "CommandConfigs is empty",
			config: &Config{
				NewTask:  dummyNewTask(),
				Commands: []*CommandConfig{},
			},
			expectedError: "Commands는 비어있을 수 없습니다",
		},
		{
			name: "CommandID is empty",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedError: "CommandID는 비어있을 수 없습니다",
		},
		{
			name: "NewSnapshot is nil",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "SafeCommand",
						AllowMultiple: true,
						// NewSnapshot missing
					},
				},
			},
			expectedError: "NewSnapshot은 nil일 수 없습니다",
		},
		{
			name: "NewSnapshot returns nil",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "NilDataCommand",
						AllowMultiple: true,
						NewSnapshot: func() interface{} {
							return nil
						},
					},
				},
			},
			expectedError: "NewSnapshot 결과값은 nil일 수 없습니다",
		},
		{
			name: "Duplicate CommandID",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "DuplicateCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
					{
						ID:            "DuplicateCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedError: "중복된 CommandID입니다: DuplicateCommand",
		},
		{
			name: "Valid Config",
			config: &Config{
				NewTask: dummyNewTask(),
				Commands: []*CommandConfig{
					{
						ID:            "ValidCommand",
						AllowMultiple: true,
						NewSnapshot:   dummyResultFn(),
					},
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
