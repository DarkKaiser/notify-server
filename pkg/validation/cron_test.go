package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Cron Expression Validation Tests
// =============================================================================

// TestValidateRobfigCronExpression은 Cron 표현식 유효성 검사를 검증합니다.
//
// 검증 항목:
//   - 표준 Cron (5 필드) - 6 필드 설정으로 인해 거부됨
//   - 확장 Cron (6 필드) - 초 단위 포함
//   - 특수 표현식 (@daily, @hourly 등)
//   - 잘못된 형식 (필드 부족, 잘못된 문자)
//   - 빈 문자열
func TestValidateRobfigCronExpression(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{
			name:    "Standard Cron (5 fields - invalid due to strict 6 fields setting)",
			spec:    "0 5 * * *", // 5 fields
			wantErr: true,
		},
		{
			name:    "Extended Cron (6 fields - with seconds)",
			spec:    "0 */5 * * * *", // 5분마다 (0초)
			wantErr: false,
		},
		{
			name:    "Daily at midnight",
			spec:    "@daily",
			wantErr: false,
		},
		{
			name:    "Invalid Cron (too few fields)",
			spec:    "* * *",
			wantErr: true,
		},
		{
			name:    "Invalid Cron (garbage)",
			spec:    "invalid-cron",
			wantErr: true,
		},
		{
			name:    "Empty string",
			spec:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRobfigCronExpression(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				// 에러 메시지 검증 (apperrors 의존성 제거됨, 문자열 포함 여부로 확인)
				assert.True(t, strings.Contains(err.Error(), "Cron 표현식 파싱 실패"), "Error message mismatch: %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
