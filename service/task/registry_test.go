package task

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

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

	r.RegisterForTest(testTaskID, &Config{
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

// TestRegistry_DeepCopy는 등록 시 Config 객체가 복사되어
// 원본 맵이 수정되더라도 레지스트리 내부 상태는 안전함을 보장합니다.
func TestRegistry_DeepCopy(t *testing.T) {
	r := newRegistry()
	taskID := ID("TEST_IMMUTABLE")
	cmdID := CommandID("TEST_CMD")

	// 1. 초기 Config 생성
	commands := []*CommandConfig{
		{
			ID:            cmdID,
			AllowMultiple: true,
			NewSnapshot:   dummyResultFn(),
		},
	}

	config := &Config{
		Commands: commands,
		NewTask:  dummyNewTask(),
	}

	// 2. 등록
	r.Register(taskID, config)

	// 3. 원본 슬라이스 변조 (새 커맨드 추가 등)
	commands[0].AllowMultiple = false // 원본 수정
	commands = append(commands, &CommandConfig{
		ID:            "HACKED_CMD",
		AllowMultiple: true,
		NewSnapshot:   dummyResultFn(),
	})

	// 4. 레지스트리에서 조회하여 불변성 확인
	result, err := r.findConfig(taskID, cmdID)
	assert.NoError(t, err)

	// 등록 시점의 값이 유지되어야 함 (AllowMultiple: true)
	assert.True(t, result.Command.AllowMultiple, "원본 슬라이스 변조가 레지스트리에 영향을 주면 안 됩니다")
	// "HACKED_CMD"는 등록되지 않아야 함
	_, errHack := r.findConfig(taskID, "HACKED_CMD")
	assert.Equal(t, ErrCommandNotSupported, errHack)
}

// TestRegistry_Concurrency_Stress는 과도한 동시성 요청 하에서 레지스트리의 안정성(Race Condition)을 검증합니다.
func TestRegistry_Concurrency_Stress(t *testing.T) {
	r := newRegistry()
	var wg sync.WaitGroup

	// Worker 개수
	readers := 50
	writers := 20
	iterations := 100

	// Writer: 랜덤하게 Task 등록
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Random delay to mix read/write operations
				time.Sleep(time.Duration(rand.Intn(5)) * time.Millisecond)

				taskID := ID(fmt.Sprintf("TASK_%d_%d", writerID, j))
				cmdID := CommandID("CMD")

				// 동시성 테스트에서는 Panic이 발생할 수 있는데(중복 ID 등), 여기서는 고유 ID를 생성한다고 가정하거나
				// Register 내부 Lock이 잘 동작하는지 확인
				r.Register(taskID, &Config{
					NewTask: dummyNewTask(),
					Commands: []*CommandConfig{{
						ID:          cmdID,
						NewSnapshot: dummyResultFn(),
					}},
				})
			}
		}(i)
	}

	// Reader: 무작위 Task 조회 시도 (존재하지 않을 수도 있음 - 에러 처리 확인)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Random Writer의 Task ID 추측
				targetWriter := rand.Intn(writers)
				targetIter := rand.Intn(iterations)
				taskID := ID(fmt.Sprintf("TASK_%d_%d", targetWriter, targetIter))

				_, _ = r.findConfig(taskID, "CMD")
			}
		}(i)
	}

	wg.Wait()
}
