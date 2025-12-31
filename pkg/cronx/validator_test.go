package cronx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestValidate_Comprehensive 는 Cron 표현식 유효성 검증 로직을 상세하게 테스트합니다.
//
// 테스트 목표:
// 1. 프로젝트 표준인 "6필드 (초 단위 포함)" 형식을 정확히 준수하는지 확인
// 2. 잘못된 형식(5필드, 가비지 값)에 대해 명확한 에러를 반환하는지 검증
// 3. 사용자 정의 에러 메시지 포맷팅이 올바른지 확인
func TestValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		spec          string
		isValid       bool
		errorContains string // 에러 메시지에 포함되어야 할 문자열 (실패 케이스용)
	}

	testGroups := []struct {
		groupName string
		cases     []testCase
	}{
		{
			groupName: "1. Valid Cases (6 Fields - Seconds Included)",
			cases: []testCase{
				{name: "Every 5 minutes at 0 seconds", spec: "0 */5 * * * *", isValid: true},
				{name: "Specific Time (10:30:00)", spec: "0 30 10 * * *", isValid: true},
				{name: "Complex Range", spec: "0 0-30/5 9-17 * * MON-FRI", isValid: true},
				{name: "Trailing Spaces (Should be trimmed)", spec: " 0 * * * * * ", isValid: true},
			},
		},
		{
			groupName: "2. Valid Cases (Descriptors)",
			cases: []testCase{
				{name: "@daily", spec: "@daily", isValid: true},
				{name: "@hourly", spec: "@hourly", isValid: true},
				{name: "@every duration", spec: "@every 1h30m", isValid: true},
			},
		},
		{
			groupName: "3. Invalid Cases (Project Constraints)",
			cases: []testCase{
				{
					name:          "5 Fields (Standard Cron - Not Supported)",
					spec:          "*/5 * * * *",
					isValid:       false,
					errorContains: "expected exactly 6 fields",
				},
				{
					name:          "7 Fields (Too Many)",
					spec:          "* * * * * * *",
					isValid:       false,
					errorContains: "expected exactly 6 fields",
				},
			},
		},
		{
			groupName: "4. Invalid Cases (Syntax Errors)",
			cases: []testCase{
				{
					name:          "Garbage String",
					spec:          "invalid-cron",
					isValid:       false,
					errorContains: "Cron 표현식 파싱 실패",
				},
				{
					name:          "Empty String",
					spec:          "",
					isValid:       false,
					errorContains: "empty spec string",
				},
				{
					name:          "Invalid Range (Seconds > 59)",
					spec:          "70 * * * * *",
					isValid:       false,
					errorContains: "Cron 표현식 파싱 실패", // 상세 메시지는 라이브러리 버전에 따라 다를 수 있으므로 래퍼 메시지만 확인
				},
			},
		},
	}

	for _, tg := range testGroups {
		tg := tg // Capture loop variable
		t.Run(tg.groupName, func(t *testing.T) {
			t.Parallel()

			for _, tc := range tg.cases {
				tc := tc // Capture loop variable
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()

					err := Validate(tc.spec)

					if tc.isValid {
						assert.NoError(t, err, "유효한 표현식이어야 합니다: %q", tc.spec)
					} else {
						assert.Error(t, err, "유효하지 않은 표현식이어야 합니다: %q", tc.spec)
						if tc.errorContains != "" {
							// 대소문자 무시하고 포함 여부 확인 (라이브러리 에러 메시지 변경 대비)
							assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tc.errorContains),
								"에러 메시지에 예상 문구가 포함되어야 합니다")
						}

						// 사용자가 추가한 래핑 메시지 확인 ("Cron 표현식 파싱 실패")
						// 단, "empty spec string" 같은 일부 에러는 robfig 내부에서 바로 리턴될 수도 있으니
						// 래핑이 적용되었는지 확인 (Garbage String 케이스 등)
						if strings.Contains(tc.name, "Garbage") {
							assert.Contains(t, err.Error(), "Cron 표현식 파싱 실패", "에러가 올바르게 래핑되어야 합니다")
						}
					}
				})
			}
		})
	}
}
