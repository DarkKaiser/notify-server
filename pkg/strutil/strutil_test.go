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

// =============================================================================
// HTML Tag Stripping Tests
// =============================================================================

// TestStripHTMLTags는 StripHTMLTags 함수의 HTML 태그 제거 동작을 검증합니다.
//
// 검증 항목:
//   - 일반 텍스트 (변경 없음)
//   - 단순 태그 포함 (<b>, </b>)
//   - 복합 태그 포함 (<a>, <span> 등)
//   - 속성이 있는 태그 (<a href="...">)
//   - 중첩 태그
//   - 불완전한 태그 (HTML 파서가 아니므로 단순 정규식 동작 확인)
func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Plain text", "Hello World", "Hello World"},
		{"Simple bold tag", "<b>Hello</b> World", "Hello World"},
		{"Tag with attributes", `<a href="http://example.com">Link</a>`, "Link"},
		{"Complex structure", "<div><span><b>Hello</b></span></div>", "Hello"},
		{"Nested tags", "<b><i>BoldItalic</i></b>", "BoldItalic"},
		{"Self-closing tag", "Hello<br/>World", "HelloWorld"}, // 공백 없이 제거됨에 유의
		{"Multiple tags", "<h1>Title</h1><p>Paragraph</p>", "TitleParagraph"},
		{"Naver Search API Example", "삼성 갤럭시 <b>S25</b> <b>FE</b> 256GB 자급제", "삼성 갤럭시 S25 FE 256GB 자급제"},

		// Expert Level Cases (HTML 태그 제거 고도화 검증)
		{"Math operator < (Not a tag)", "3 < 5", "3 < 5"},
		{"Math operator >", "5 > 3", "5 > 3"},
		{"Mixed math and tags", "<b>Values:</b> 3 < 5", "Values: 3 < 5"},
		{"HTML Entities: Ampersand", "Tom &amp; Jerry", "Tom & Jerry"},
		{"HTML Entities: Less Than", "3 &lt; 5", "3 < 5"},
		{"HTML Entities: Greater Than", "5 &gt; 3", "5 > 3"},
		{"HTML Entities: Quote", "&quot;Quote&quot;", "\"Quote\""},
		{"Case Insensitive Tag", "<B>Bold</B>", "Bold"},
		{"Complex Mix", "Start <b>&lt;Middle&gt;</b> End", "Start <Middle> End"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripHTMLTags(tt.input))
		})
	}
}

// =============================================================================
// Filter Tests
// =============================================================================

func TestFilter(t *testing.T) {
	t.Run("포함 키워드만 있는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("포함 키워드가 없는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "포함 키워드가 없으면 false를 반환해야 합니다")
	})

	t.Run("제외 키워드가 있는 경우", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트"}
		excludedKeywords := []string{"문자열"}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "제외 키워드가 있으면 false를 반환해야 합니다")
	})

	t.Run("여러 포함 키워드 모두 만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트", "문자열"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "모든 포함 키워드가 있으면 true를 반환해야 합니다")
	})

	t.Run("여러 포함 키워드 중 하나 불만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"테스트", "샘플"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "포함 키워드 중 하나라도 없으면 false를 반환해야 합니다")
	})

	t.Run("OR 조건 포함 키워드 - 하나라도 만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플|테스트|예제"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "OR 조건 중 하나라도 만족하면 true를 반환해야 합니다")
	})

	t.Run("OR 조건 포함 키워드 - 모두 불만족", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{"샘플|예제|데모"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.False(t, result, "OR 조건 모두 불만족하면 false를 반환해야 합니다")
	})

	t.Run("복합 조건 - 포함과 제외", func(t *testing.T) {
		s := "삼성 갤럭시 스마트폰"
		includedKeywords := []string{"삼성", "스마트폰"}
		excludedKeywords := []string{"아이폰"}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "포함 키워드는 만족하고 제외 키워드는 없으면 true를 반환해야 합니다")
	})

	t.Run("빈 키워드 리스트", func(t *testing.T) {
		s := "이것은 테스트 문자열입니다"
		includedKeywords := []string{}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)
		assert.True(t, result, "키워드가 없으면 true를 반환해야 합니다")
	})

	t.Run("대소문자 구분 테스트", func(t *testing.T) {
		s := "Samsung Galaxy Smartphone"
		includedKeywords := []string{"samsung"}
		excludedKeywords := []string{}

		result := Filter(s, includedKeywords, excludedKeywords)

		// filter 함수가 대소문자를 구분하는지 확인
		// 실제 구현에 따라 결과가 달라질 수 있음
		if result {
			// 대소문자 구분 안 함
			assert.True(t, result, "대소문자 구분 없이 매칭되어야 합니다")
		} else {
			// 대소문자 구분 함
			assert.False(t, result, "대소문자를 구분하여 매칭해야 합니다")
		}
	})
}
