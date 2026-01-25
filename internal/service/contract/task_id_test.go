package contract

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTaskID_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        TaskID
		isValid   bool
		wantError string
	}{
		{
			name:    "유효한 TaskID",
			id:      "CRAWLER_NEWS",
			isValid: true,
		},
		{
			name:      "빈 TaskID",
			id:        "",
			isValid:   false,
			wantError: "TaskID는 필수입니다",
		},
		{
			name:      "공백만 있는 TaskID",
			id:        "   ",
			isValid:   false,
			wantError: "TaskID는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Validate Checks
			err := tt.id.Validate()
			if tt.isValid {
				assert.NoError(t, err)
				assert.False(t, tt.id.IsEmpty())
			} else {
				assert.Error(t, err)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			}

			// String Conversion Check
			assert.Equal(t, string(tt.id), tt.id.String())
		})
	}
}

func TestTaskCommandID_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        TaskCommandID
		isValid   bool
		wantError string
	}{
		{
			name:    "유효한 CommandID",
			id:      "START",
			isValid: true,
		},
		{
			name:      "빈 CommandID",
			id:        "",
			isValid:   false,
			wantError: "TaskCommandID는 필수입니다",
		},
		{
			name:      "공백만 있는 CommandID",
			id:        "   ",
			isValid:   false,
			wantError: "TaskCommandID는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.id.Validate()
			if tt.isValid {
				assert.NoError(t, err)
				assert.False(t, tt.id.IsEmpty())
			} else {
				assert.Error(t, err)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			}
			assert.Equal(t, string(tt.id), tt.id.String())
		})
	}
}

func TestTaskCommandID_Match(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		pattern   TaskCommandID
		target    TaskCommandID
		wantMatch bool
	}{
		{
			name:      "정확히 일치 (Exact Match)",
			pattern:   "START",
			target:    "START",
			wantMatch: true,
		},
		{
			name:      "일치하지 않음",
			pattern:   "START",
			target:    "STOP",
			wantMatch: false,
		},
		{
			name:      "와일드카드 접두어 매칭 (Prefix Match)",
			pattern:   "CMD_*",
			target:    "CMD_START",
			wantMatch: true,
		},
		{
			name:      "와일드카드 접두어 매칭 (긴 타겟)",
			pattern:   "CMD_*",
			target:    "CMD_WITH_VERY_LONG_NAME",
			wantMatch: true,
		},
		{
			name:      "와일드카드 매칭 실패 (접두어 불일치)",
			pattern:   "CMD_*",
			target:    "OTHER_CMD",
			wantMatch: false,
		},
		{
			name:      "와일드카드 매칭 실패 (타겟이 더 짧음)",
			pattern:   "LongCommand_*",
			target:    "Long",
			wantMatch: false,
		},
		{
			name:      "전체 와일드카드 (*)",
			pattern:   "*",
			target:    "ANYTHING",
			wantMatch: true,
		},
		{
			name:      "전체 와일드카드 (*) - 빈 타겟 (안전장치)",
			pattern:   "*",
			target:    "",
			wantMatch: false,
		},
		{
			name:      "빈 패턴과 매칭 (정확히 빈 값만 일치하지만 Validate 때문에 실제론 거의 발생 안 함)",
			pattern:   "",
			target:    "SOMETHING",
			wantMatch: false,
		},
		{
			name:      "타겟이 빈 값이면 항상 실패 (안전장치 검증)",
			pattern:   "CMD",
			target:    "",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantMatch, tt.pattern.Match(tt.target))
		})
	}
}

func TestTaskInstanceID_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		id        TaskInstanceID
		isValid   bool
		wantError string
	}{
		{
			name:    "유효한 InstanceID",
			id:      "UUID-1234-5678",
			isValid: true,
		},
		{
			name:      "빈 InstanceID",
			id:        "",
			isValid:   false,
			wantError: "TaskInstanceID는 필수입니다",
		},
		{
			name:      "공백만 있는 InstanceID",
			id:        "   ",
			isValid:   false,
			wantError: "TaskInstanceID는 필수입니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.id.Validate()
			if tt.isValid {
				assert.NoError(t, err)
				assert.False(t, tt.id.IsEmpty())
			} else {
				assert.Error(t, err)
				if tt.wantError != "" {
					assert.Contains(t, err.Error(), tt.wantError)
				}
			}
			assert.Equal(t, string(tt.id), tt.id.String())
		})
	}
}
