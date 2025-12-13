package task

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          ID
		expectedErr bool
	}{
		{
			name:        "유효한 ID",
			id:          "TASK-1",
			expectedErr: false,
		},
		{
			name:        "빈 ID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       ID
		expected bool
	}{
		{
			name:     "값이 있음",
			id:       "TASK-1",
			expected: false,
		},
		{
			name:     "값이 없음 (Empty)",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestID_String(t *testing.T) {
	t.Parallel()
	id := ID("TASK-1")
	assert.Equal(t, "TASK-1", id.String())
}

func TestCommandID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          CommandID
		expectedErr bool
	}{
		{
			name:        "유효한 CommandID",
			id:          "CMD-1",
			expectedErr: false,
		},
		{
			name:        "빈 CommandID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       CommandID
		expected bool
	}{
		{
			name:     "값이 있음",
			id:       "CMD-1",
			expected: false,
		},
		{
			name:     "값이 없음 (Empty)",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestCommandID_String(t *testing.T) {
	t.Parallel()
	id := CommandID("CMD-1")
	assert.Equal(t, "CMD-1", id.String())
}

func TestCommandID_Match(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		pattern        CommandID
		target         CommandID
		expectedResult bool
	}{
		{
			name:           "정확히 일치 (Exact Match)",
			pattern:        "WatchPrice",
			target:         "WatchPrice",
			expectedResult: true,
		},
		{
			name:           "일치하지 않음",
			pattern:        "WatchPrice",
			target:         "WatchStock",
			expectedResult: false,
		},
		{
			name:           "와일드카드 매칭 (Prefix Match)",
			pattern:        "WatchPrice_*",
			target:         "WatchPrice_Product1",
			expectedResult: true,
		},
		{
			name:           "와일드카드 매칭 실패",
			pattern:        "WatchPrice_*",
			target:         "WatchStock_Product1",
			expectedResult: false,
		},
		{
			name:           "와일드카드이지만 타겟이 더 짧은 경우",
			pattern:        "WatchPrice_*",
			target:         "Watch",
			expectedResult: false,
		},
		{
			name:           "전체 와일드카드 (*)",
			pattern:        "*",
			target:         "AnyCommand",
			expectedResult: true, // 로직상 Suffix가 *이면 Prefix는 ""이므로 항상 True여야 함
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, c.expectedResult, c.pattern.Match(c.target))
		})
	}
}

func TestInstanceID_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		id          InstanceID
		expectedErr bool
	}{
		{
			name:        "유효한 InstanceID",
			id:          "INST-1",
			expectedErr: false,
		},
		{
			name:        "빈 InstanceID",
			id:          "",
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.id.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInstanceID_IsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		id       InstanceID
		expected bool
	}{
		{
			name:     "값이 있음",
			id:       "INST-1",
			expected: false,
		},
		{
			name:     "값이 없음 (Empty)",
			id:       "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.id.IsEmpty())
		})
	}
}

func TestInstanceID_String(t *testing.T) {
	t.Parallel()
	id := InstanceID("INST-1")
	assert.Equal(t, "INST-1", id.String())
}

func TestRunBy_Validity_And_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		runBy          RunBy
		expectedString string
		isValid        bool
		validateErr    bool
	}{
		{
			name:           "User (사용자 실행)",
			runBy:          RunByUser,
			expectedString: "User",
			isValid:        true,
			validateErr:    false,
		},
		{
			name:           "Scheduler (스케줄러 실행)",
			runBy:          RunByScheduler,
			expectedString: "Scheduler",
			isValid:        true,
			validateErr:    false,
		},
		{
			name:           "Unknown (알 수 없음)",
			runBy:          RunByUnknown,
			expectedString: "Unknown",
			isValid:        false,
			validateErr:    true,
		},
		{
			name:           "Invalid Value (잘못된 값)",
			runBy:          RunBy(999),
			expectedString: "Unknown",
			isValid:        false,
			validateErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// String() Check
			assert.Equal(t, tt.expectedString, tt.runBy.String())
			// IsValid() Check
			assert.Equal(t, tt.isValid, tt.runBy.IsValid())
			// Validate() Check
			err := tt.runBy.Validate()
			if tt.validateErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSubmitRequest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		req         *SubmitRequest
		expectedErr bool
	}{
		{
			name: "정상 요청",
			req: &SubmitRequest{
				TaskID:    "TASK-1",
				CommandID: "CMD-1",
				RunBy:     RunByUser,
			},
			expectedErr: false,
		},
		{
			name: "필수값 누락: TaskID",
			req: &SubmitRequest{
				TaskID:    "",
				CommandID: "CMD-1",
				RunBy:     RunByUser,
			},
			expectedErr: true,
		},
		{
			name: "필수값 누락: CommandID",
			req: &SubmitRequest{
				TaskID:    "TASK-1",
				CommandID: "",
				RunBy:     RunByUser,
			},
			expectedErr: true,
		},
		{
			name: "잘못된 실행 주체 (RunBy)",
			req: &SubmitRequest{
				TaskID:    "TASK-1",
				CommandID: "CMD-1",
				RunBy:     RunByUnknown,
			},
			expectedErr: true,
		},
		{
			name: "NotifierID 포함 (정상)",
			req: &SubmitRequest{
				TaskID:     "TASK-1",
				CommandID:  "CMD-1",
				RunBy:      RunByUser,
				NotifierID: "TEST-NOTIFIER",
			},
			expectedErr: false,
		},
		{
			name: "잘못된 NotifierID (공백)",
			req: &SubmitRequest{
				TaskID:     "TASK-1",
				CommandID:  "CMD-1",
				RunBy:      RunByUser,
				NotifierID: "   ",
			},
			expectedErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.req.Validate()
			if tt.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
