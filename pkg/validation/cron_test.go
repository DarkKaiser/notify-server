package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidateCronExpression은 ValidateCronExpression 함수의 정상 동작 여부를 검증합니다.
//
// 배경:
//   - 이 함수는 내부적으로 pkg/cronx 패키지의 StandardParser를 사용하여 검증을 수행합니다.
//   - 따라서 상세한 파싱 로직에 대한 테스트는 pkg/cronx/parser_test.go에서 수행되었으므로,
//     여기서는 함수 호출 및 에러 전파(Error Propagation)가 올바르게 이루어지는지에 집중합니다.
//
// 검증 항목:
//   - 정상 케이스: 6필드 Cron, Descriptor(@daily)
//   - 실패 케이스: 5필드 Cron(미지원), 잘못된 형식, 빈 문자열
func TestValidateCronExpression(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		spec          string
		wantErr       bool
		errorContains string // 예상되는 에러 메시지 일부 (옵션)
	}{
		// =================================================================
		// Success Cases (Valid)
		// =================================================================
		{
			name:    "Extended Cron (6 fields) - Valid",
			spec:    "0 */5 * * * *", // 5분마다 0초에
			wantErr: false,
		},
		{
			name:    "Descriptor - @daily",
			spec:    "@daily",
			wantErr: false,
		},

		// =================================================================
		// Failure Cases (Invalid)
		// =================================================================
		{
			name:          "Standard Cron (5 fields) - Not Supported",
			spec:          "0 5 * * *",
			wantErr:       true,
			errorContains: "expected exactly 6 fields", // 6필드만 허용됨
		},
		{
			name:          "Invalid Format - Garbage",
			spec:          "invalid-cron-expression",
			wantErr:       true,
			errorContains: "Cron 표현식 파싱 실패", // 함수가 래핑한 에러 메시지 확인
		},
		{
			name:          "Invalid Format - Too Few Fields",
			spec:          "* * *",
			wantErr:       true,
			errorContains: "expected exactly 6 fields",
		},
		{
			name:          "Empty String",
			spec:          "",
			wantErr:       true,
			errorContains: "empty spec string",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCronExpression(tt.spec)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains, "에러 메시지에 예상 문구가 포함되어야 함")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
