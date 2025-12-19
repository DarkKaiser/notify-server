package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Case Conversion Tests
// =============================================================================

// TestToSnakeCase는 ToSnakeCase 함수의 CamelCase/PascalCase를 snake_case로 변환하는 동작을 검증합니다.
//
// 검증 항목:
//   - 빈 문자열 처리
//   - 단순 문자열 (소문자 변환)
//   - 숫자 포함 문자열
//   - CamelCase 변환
//   - PascalCase 변환
//   - 공백 포함 문자열
func TestToSnakeCase(t *testing.T) {
	cases := []struct {
		name     string
		str      string
		expected string
	}{
		{name: "Empty string", str: "", expected: ""},
		{name: "Simple", str: "My", expected: "my"},
		{name: "Numeric", str: "123", expected: "123"},
		{name: "Numeric and letters", str: "123abc", expected: "123abc"},
		{name: "CamelCase 1", str: "123abcDef", expected: "123abc_def"},
		{name: "CamelCase 2", str: "123abcDefGHI", expected: "123abc_def_ghi"},
		{name: "CamelCase 3", str: "123abcDefGHIj", expected: "123abc_def_gh_ij"},
		{name: "CamelCase 4", str: "123abcDefGHIjK", expected: "123abc_def_gh_ij_k"},
		{name: "PascalCase", str: "MyNameIsTom", expected: "my_name_is_tom"},
		{name: "camelCase", str: "myNameIsTom", expected: "my_name_is_tom"},
		{name: "With spaces", str: " myNameIsTom ", expected: " my_name_is_tom "},
		{name: "With spaces and camelCase", str: " myNameIsTom  yourNameIsB", expected: " my_name_is_tom  your_name_is_b"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, ToSnakeCase(c.str))
		})
	}
}

// =============================================================================
// Space Normalization Tests
// =============================================================================

// TestNormalizeSpaces는 NormalizeSpaces 함수의 공백 정규화 동작을 검증합니다.
//
// 검증 항목:
//   - 한글 문자열 (변경 없음)
//   - 앞뒤 공백 제거
//   - 단일 공백 유지
//   - 연속된 공백을 하나로 축약
//   - 복잡한 공백 패턴
//   - 특수 문자 포함
//   - 여러 줄 문자열 (한 줄로 축약)
func TestNormalizeSpaces(t *testing.T) {
	cases := []struct {
		name     string
		s        string
		expected string
	}{
		{name: "Korean", s: "테스트", expected: "테스트"},
		{name: "Surrounding spaces", s: "   테스트   ", expected: "테스트"},
		{name: "Single space inside", s: "   하나 공백   ", expected: "하나 공백"},
		{name: "Multiple spaces inside", s: "   다수    공백   ", expected: "다수 공백"},
		{name: "Complex spaces", s: "   다수    공백   여러개   ", expected: "다수 공백 여러개"},
		{name: "Special characters", s: "   @    특수문자   $   ", expected: "@ 특수문자 $"},
		{
			name: "Multiline string",
			s: `
		
				라인    1
				라인2
		
		
				라인3
		
				라인4
		
		
				라인5

			`,
			expected: "라인 1 라인2 라인3 라인4 라인5",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, NormalizeSpaces(c.s))
		})
	}
}

// TestNormalizeMultiLineSpaces는 NormalizeMultiLineSpaces 함수의 여러 줄 공백 정규화 동작을 검증합니다.
//
// 검증 항목:
//   - 빈 문자열
//   - 공백만 있는 문자열
//   - 앞뒤 공백 제거
//   - 복잡한 여러 줄 문자열
//   - 연속된 빈 줄을 하나로 축약
//   - 앞뒤 빈 줄 제거
func TestNormalizeMultiLineSpaces(t *testing.T) {
	cases := []struct {
		name     string
		s        string
		expected string
	}{
		{name: "Empty", s: "", expected: ""},
		{name: "Only spaces", s: "   ", expected: ""},
		{name: "Surrounding spaces with char", s: "  a  ", expected: "a"},
		{
			name: "Complex multiline",
			s: `
		
				라인    1
				라인2
		
		
				라인3

				라인4



				라인5


			`,
			expected: "라인 1\r\n라인2\r\n\r\n라인3\r\n\r\n라인4\r\n\r\n라인5",
		},
		{
			name: "Complex multiline 2",
			s: ` 라인    1


			라인2


			라인3
			라인4
			라인5   `,
			expected: "라인 1\r\n\r\n라인2\r\n\r\n라인3\r\n라인4\r\n라인5",
		},
		{
			name: "Empty lines",
			s: `

			`,
			expected: "",
		},
		{
			name: "Single value with newlines",
			s: `

			1

			`,
			expected: "1",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, NormalizeMultiLineSpaces(c.s))
		})
	}
}

// =============================================================================
// Number Formatting Tests
// =============================================================================

// TestFormatCommas는 FormatCommas 함수의 숫자 천 단위 구분 기호 포맷팅 동작을 검증합니다.
//
// 검증 항목:
//   - int 타입 (0, 양수, 음수)
//   - int64 타입 (최대값, 최소값)
//   - uint 타입
//   - uint64 타입 (최대값)
func TestFormatCommas(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		tests := []struct {
			input    int
			expected string
		}{
			{0, "0"},
			{100, "100"},
			{1000, "1,000"},
			{1234567, "1,234,567"},
			{-1234567, "-1,234,567"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, FormatCommas(tt.input))
		}
	})

	t.Run("int64", func(t *testing.T) {
		tests := []struct {
			input    int64
			expected string
		}{
			{9223372036854775807, "9,223,372,036,854,775,807"},
			{-9223372036854775808, "-9,223,372,036,854,775,808"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, FormatCommas(tt.input))
		}
	})

	t.Run("uint", func(t *testing.T) {
		tests := []struct {
			input    uint
			expected string
		}{
			{1000, "1,000"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, FormatCommas(tt.input))
		}
	})

	t.Run("uint64", func(t *testing.T) {
		tests := []struct {
			input    uint64
			expected string
		}{
			{18446744073709551615, "18,446,744,073,709,551,615"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, FormatCommas(tt.input))
		}
	})
}

// =============================================================================
// String Splitting Tests
// =============================================================================

// TestSplitAndTrim은 SplitAndTrim 함수의 문자열 분리 및 트림 동작을 검증합니다.
//
// 검증 항목:
//   - 쉼표로 구분된 문자열
//   - 빈 항목 제거
//   - 공백 포함 항목 트림
//   - 빈 구분자
//   - 여러 문자 구분자
//   - 구분자가 없는 경우
//   - 빈 문자열 (nil 반환)
func TestSplitAndTrim(t *testing.T) {
	var notAssign []string

	cases := []struct {
		name     string
		s        string
		sep      string
		expected []string
	}{
		{name: "Comma separated", s: "1,2,3", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Comma separated with empty", s: ",1,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Comma separated with spaces", s: ",1,  ,  ,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Empty separator", s: ",1,,2,3,", sep: "", expected: []string{",", "1", ",", ",", "2", ",", "3", ","}},
		{name: "Multi-char separator", s: ",1,,2,3,", sep: ",,", expected: []string{",1", "2,3,"}},
		{name: "Separator not found", s: "1,2,3", sep: "-", expected: []string{"1,2,3"}},
		{name: "Empty string", s: "", sep: "-", expected: notAssign},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, SplitAndTrim(c.s, c.sep))
		})
	}
}

// =============================================================================
// Sensitive Data Masking Tests
// =============================================================================

// TestMaskSensitiveData는 MaskSensitiveData 함수의 민감 정보 마스킹 동작을 검증합니다.
//
// 검증 항목:
//   - 빈 문자열
//   - 짧은 문자열 (1-3자) - 전체 마스킹
//   - 중간 길이 문자열 (4-12자) - 앞 4자 표시
//   - 긴 문자열 (13자 이상) - 앞 4자 + 마스킹 + 뒤 4자
func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Short string (1 char)", "a", "***"},
		{"Short string (2 chars)", "ab", "***"},
		{"Short string (3 chars)", "abc", "***"},
		{"Medium string (4 chars)", "abcd", "abcd***"},
		{"Medium string (12 chars)", "123456789012", "1234***"},
		{"Long string (token)", "123456789:ABCdefGHIjklMNOpqrsTUVwxyz", "1234***wxyz"},
		{"Long string (general)", "this_is_a_very_long_secret_key", "this***_key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
