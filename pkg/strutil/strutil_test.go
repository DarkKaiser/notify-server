package strutil

import (
	"strings"
	"testing"
	"time"

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

// MatchesKeywords Tests
// =============================================================================

func TestMatchesKeywords(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		input            string
		includedKeywords []string
		excludedKeywords []string
		want             bool
	}{
		// ===== 기본 시나리오 =====
		{
			name:             "빈 문자열, 빈 키워드",
			input:            "",
			includedKeywords: []string{},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "빈 문자열, 포함 키워드 있음",
			input:            "",
			includedKeywords: []string{"test"},
			excludedKeywords: []string{},
			want:             false,
		},
		{
			name:             "일반 문자열, 빈 키워드",
			input:            "Hello World",
			includedKeywords: []string{},
			excludedKeywords: []string{},
			want:             true,
		},

		// ===== 포함 키워드 (AND 조건) =====
		{
			name:             "단일 포함 키워드 - 매칭 성공",
			input:            "Go Programming Language",
			includedKeywords: []string{"programming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "단일 포함 키워드 - 매칭 실패",
			input:            "Go Programming Language",
			includedKeywords: []string{"python"},
			excludedKeywords: []string{},
			want:             false,
		},
		{
			name:             "다중 포함 키워드 - 모두 매칭",
			input:            "Go Programming Language Tutorial",
			includedKeywords: []string{"go", "programming", "tutorial"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "다중 포함 키워드 - 일부만 매칭",
			input:            "Go Programming Language",
			includedKeywords: []string{"go", "programming", "tutorial"},
			excludedKeywords: []string{},
			want:             false,
		},
		{
			name:             "부분 문자열 매칭",
			input:            "Golang is great",
			includedKeywords: []string{"lang"},
			excludedKeywords: []string{},
			want:             true,
		},

		// ===== 제외 키워드 (OR 조건) =====
		{
			name:             "단일 제외 키워드 - 포함됨 (실패)",
			input:            "Deprecated API",
			includedKeywords: []string{},
			excludedKeywords: []string{"deprecated"},
			want:             false,
		},
		{
			name:             "단일 제외 키워드 - 포함 안됨 (성공)",
			input:            "Modern API",
			includedKeywords: []string{},
			excludedKeywords: []string{"deprecated"},
			want:             true,
		},
		{
			name:             "다중 제외 키워드 - 하나라도 포함 (실패)",
			input:            "Legacy System",
			includedKeywords: []string{},
			excludedKeywords: []string{"deprecated", "legacy", "old"},
			want:             false,
		},
		{
			name:             "다중 제외 키워드 - 모두 불포함 (성공)",
			input:            "Modern System",
			includedKeywords: []string{},
			excludedKeywords: []string{"deprecated", "legacy", "old"},
			want:             true,
		},

		// ===== OR 조건 (파이프 구분자) =====
		{
			name:             "OR 조건 - 첫 번째 키워드 매칭",
			input:            "Go Tutorial",
			includedKeywords: []string{"Go|Rust|Python"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "OR 조건 - 중간 키워드 매칭",
			input:            "Rust Tutorial",
			includedKeywords: []string{"Go|Rust|Python"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "OR 조건 - 마지막 키워드 매칭",
			input:            "Python Tutorial",
			includedKeywords: []string{"Go|Rust|Python"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "OR 조건 - 모두 불매칭",
			input:            "Java Tutorial",
			includedKeywords: []string{"Go|Rust|Python"},
			excludedKeywords: []string{},
			want:             false,
		},
		{
			name:             "OR 조건 - 공백 포함",
			input:            "Web Development",
			includedKeywords: []string{"Web Dev|Mobile Dev|Backend"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "다중 OR 조건 - 모두 만족",
			input:            "Go Web Server",
			includedKeywords: []string{"Go|Rust", "Web|Mobile"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "다중 OR 조건 - 하나만 만족",
			input:            "Go Desktop App",
			includedKeywords: []string{"Go|Rust", "Web|Mobile"},
			excludedKeywords: []string{},
			want:             false,
		},

		// ===== 대소문자 구분 없음 =====
		{
			name:             "대소문자 - 모두 대문자",
			input:            "GO PROGRAMMING",
			includedKeywords: []string{"go", "programming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "대소문자 - 모두 소문자",
			input:            "go programming",
			includedKeywords: []string{"GO", "PROGRAMMING"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "대소문자 - 혼합",
			input:            "Go PrOgRaMmInG",
			includedKeywords: []string{"gO", "ProGramming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "대소문자 - 제외 키워드",
			input:            "DEPRECATED API",
			includedKeywords: []string{},
			excludedKeywords: []string{"deprecated"},
			want:             false,
		},

		// ===== 복합 조건 =====
		{
			name:             "복합 - 포함 AND + 제외 OR (성공)",
			input:            "Modern Go Web Server",
			includedKeywords: []string{"go", "web"},
			excludedKeywords: []string{"deprecated", "legacy"},
			want:             true,
		},
		{
			name:             "복합 - 포함 AND + 제외 OR (제외 키워드 포함)",
			input:            "Legacy Go Web Server",
			includedKeywords: []string{"go", "web"},
			excludedKeywords: []string{"deprecated", "legacy"},
			want:             false,
		},
		{
			name:             "복합 - 포함 AND + 제외 OR (포함 키워드 불만족)",
			input:            "Modern Python Web Server",
			includedKeywords: []string{"go", "web"},
			excludedKeywords: []string{"deprecated", "legacy"},
			want:             false,
		},
		{
			name:             "복합 - OR 조건 + 제외",
			input:            "Go Tutorial for Beginners",
			includedKeywords: []string{"Go|Rust|Python", "tutorial"},
			excludedKeywords: []string{"advanced"},
			want:             true,
		},

		// ===== 특수 문자 및 유니코드 =====
		{
			name:             "한글 키워드",
			input:            "이것은 테스트 문자열입니다",
			includedKeywords: []string{"테스트", "문자열"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "한글 제외 키워드",
			input:            "이것은 샘플 문자열입니다",
			includedKeywords: []string{"문자열"},
			excludedKeywords: []string{"테스트"},
			want:             true,
		},
		{
			name:             "이모지 포함",
			input:            "🚀 Go Programming 🎉",
			includedKeywords: []string{"go", "programming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "특수 문자 포함",
			input:            "C++ Programming & Development",
			includedKeywords: []string{"c++", "development"},
			excludedKeywords: []string{},
			want:             true,
		},

		// ===== 경계 조건 (Edge Cases) =====
		{
			name:             "매우 긴 문자열",
			input:            strings.Repeat("Go Programming ", 1000),
			includedKeywords: []string{"go", "programming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "매우 많은 포함 키워드",
			input:            "a b c d e f g h i j k l m n o p q r s t u v w x y z",
			includedKeywords: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "단일 문자 키워드",
			input:            "a",
			includedKeywords: []string{"a"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "공백만 있는 문자열",
			input:            "     ",
			includedKeywords: []string{"test"},
			excludedKeywords: []string{},
			want:             false,
		},
		{
			name:             "개행 문자 포함",
			input:            "Go\nProgramming\nLanguage",
			includedKeywords: []string{"go", "programming"},
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "탭 문자 포함",
			input:            "Go\tProgramming\tLanguage",
			includedKeywords: []string{"go", "programming"},
			excludedKeywords: []string{},
			want:             true,
		},

		// ===== nil 슬라이스 처리 =====
		{
			name:             "nil 포함 키워드",
			input:            "Go Programming",
			includedKeywords: nil,
			excludedKeywords: []string{},
			want:             true,
		},
		{
			name:             "nil 제외 키워드",
			input:            "Go Programming",
			includedKeywords: []string{"go"},
			excludedKeywords: nil,
			want:             true,
		},
		{
			name:             "모두 nil",
			input:            "Go Programming",
			includedKeywords: nil,
			excludedKeywords: nil,
			want:             true,
		},

		// ===== 실제 사용 사례 =====
		{
			name:             "상품명 필터링 - 성공",
			input:            "삼성 갤럭시 S24 스마트폰",
			includedKeywords: []string{"삼성", "스마트폰"},
			excludedKeywords: []string{"아이폰", "중고"},
			want:             true,
		},
		{
			name:             "상품명 필터링 - 제외 키워드 포함",
			input:            "삼성 갤럭시 S24 중고 스마트폰",
			includedKeywords: []string{"삼성", "스마트폰"},
			excludedKeywords: []string{"아이폰", "중고"},
			want:             false,
		},
		{
			name:             "공연 제목 필터링 - OR 조건",
			input:            "뮤지컬 캣츠 - 서울 공연",
			includedKeywords: []string{"뮤지컬|연극|콘서트", "서울"},
			excludedKeywords: []string{"취소", "연기"},
			want:             true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MatchesKeywords(tt.input, tt.includedKeywords, tt.excludedKeywords)
			assert.Equal(t, tt.want, got, "MatchesKeywords() = %v, want %v", got, tt.want)
		})
	}
}

// TestMatchesKeywords_Performance 성능 테스트
func TestMatchesKeywords_Performance(t *testing.T) {
	if testing.Short() {
		t.Skip("성능 테스트는 -short 플래그 사용 시 건너뜁니다")
	}

	largeInput := strings.Repeat("Go Programming Language Tutorial for Beginners ", 10000)
	includedKeywords := []string{"go", "programming", "tutorial"}
	excludedKeywords := []string{"advanced", "expert"}

	start := time.Now()
	for i := 0; i < 1000; i++ {
		MatchesKeywords(largeInput, includedKeywords, excludedKeywords)
	}
	duration := time.Since(start)

	t.Logf("1000회 실행 시간: %v (평균: %v/op)", duration, duration/1000)

	// 성능 기준: 1000회 실행이 10초 이내여야 함 (평균 10ms/op)
	// Docker 환경의 제한된 리소스를 고려한 기준
	if duration > 10*time.Second {
		t.Errorf("성능 기준 미달: %v > 10s", duration)
	}
}
