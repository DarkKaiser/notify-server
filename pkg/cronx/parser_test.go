package cronx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStandardParser_Spec은 StandardParser가 지원하는 Cron 표현식 스펙을 검증합니다.
//
// 검증 항목:
//   - 확장 6필드 (초 단위 포함) 지원 확인
//   - 표준 5필드 미지원 확인 (의도된 설계)
//   - 특수 Descriptor (@daily, @every) 지원 확인
//   - 잘못된 형식 및 범위 검증
func TestStandardParser_Spec(t *testing.T) {
	t.Parallel()

	parser := StandardParser()
	require.NotNil(t, parser, "StandardParser는 nil을 반환하면 안 됩니다")

	tests := []struct {
		name      string
		spec      string
		wantErr   bool
		errSubstr string // 에러 메시지에 포함되어야 할 문구
	}{
		// =================================================================
		// Success Cases (Valid Specs)
		// =================================================================
		{
			name: "Extended Cron (6 fields) - Seconds",
			spec: "30 * * * * *", // 매분 30초마다
		},
		{
			name: "Extended Cron (6 fields) - Step",
			spec: "0 */5 * * * *", // 5분마다 0초에
		},
		{
			name: "Extended Cron (6 fields) - Month Name",
			spec: "0 0 1 1 JAN *", // 1월 1일 0시 0분 0초
		},
		{
			name: "Descriptor - @daily",
			spec: "@daily", // 매일 자정 (0 0 0 * * *)
		},
		{
			name: "Descriptor - @hourly",
			spec: "@hourly", // 매시간 정각 (0 0 * * * *)
		},
		{
			name: "Descriptor - @every",
			spec: "@every 1h30m", // 1시간 30분 간격
		},

		// =================================================================
		// Failure Cases (Invalid Specs / Unsupported)
		// =================================================================
		{
			name:      "Standard Cron (5 fields) - Not Supported",
			spec:      "* * * * *", // 분 시 일 월 요일 (초 필드 누락)
			wantErr:   true,
			errSubstr: "expected exactly 6 fields", // robfig/cron의 에러 메시지
		},
		{
			name:      "Too Few Fields",
			spec:      "* * *",
			wantErr:   true,
			errSubstr: "expected exactly 6 fields",
		},
		{
			name:      "Invalid Seconds (Range 0-59)",
			spec:      "60 * * * * *",
			wantErr:   true,
			errSubstr: "above maximum",
		},
		{
			name:      "Invalid Field Value (Garbage)",
			spec:      "invalid * * * * *",
			wantErr:   true,
			errSubstr: "invalid",
		},
		{
			name:      "Empty String",
			spec:      "",
			wantErr:   true,
			errSubstr: "empty",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schedule, err := parser.Parse(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
				assert.Nil(t, schedule)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, schedule)
			}
		})
	}
}

// TestStandardParser_NextSchedule은 파싱된 스케줄이 예상대로 다음 실행 시간을 계산하는지 검증합니다.
func TestStandardParser_NextSchedule(t *testing.T) {
	t.Parallel()

	parser := StandardParser()

	// 기준 시간: 2024-01-01 00:00:00 (월요일)
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		spec     string
		expected time.Time
	}{
		{
			name:     "Every 30 seconds",
			spec:     "*/30 * * * * *",
			expected: now.Add(30 * time.Second), // 00:00:00 -> 00:00:30
		},
		{
			name:     "Every 10 minutes",
			spec:     "0 */10 * * * *",
			expected: now.Add(10 * time.Minute), // 00:00:00 -> 00:10:00
		},
		{
			name:     "Descriptor @daily",
			spec:     "@daily",
			expected: now.Add(24 * time.Hour), // 00:00:00 -> 익일 00:00:00
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schedule, err := parser.Parse(tt.spec)
			require.NoError(t, err)

			next := schedule.Next(now)
			assert.Equal(t, tt.expected, next, "다음 실행 시간이 예상과 일치해야 합니다")
		})
	}
}
