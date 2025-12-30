package strutil

import (
	"fmt"
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
		{name: "With spaces", str: " myNameIsTom ", expected: "my_name_is_tom"},
		{name: "Acronyms", str: "JSONData", expected: "json_data"},
		{name: "Acronyms at end", str: "HTTPClient", expected: "http_client"},
		{name: "Foreign characters", str: "안녕Hello", expected: "안녕_hello"},
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
		{"Medium string (4 chars)", "abcd", "a***"},
		{"Medium string (5 chars)", "abcde", "abcd***"},
		{"Medium string (12 chars)", "123456789012", "1234***"},
		{"Long string (token)", "123456789:ABCdefGHIjklMNOpqrsTUVwxyz", "1234***wxyz"},
		{"Long string (general)", "this_is_a_very_long_secret_key", "this***_key"},
		// UTF-8 Tests
		{"Korean Short", "안녕", "***"},
		{"Korean Medium", "안녕하세요", "안녕하세***"},
		{"Korean Long", "안녕하세요반갑습니다", "안녕하세***"},
		{"Emoji Short", "😀😁😂", "***"},
		{"Emoji Long", "😀😁😂🤣😃😄😅😆😉😊😋😎", "😀😁😂🤣***"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, MaskSensitiveData(tt.input))
		})
	}
}

// =============================================================================
// HTML Tag Stripping Tests
// =============================================================================

// TestStripHTMLTags는 StripHTMLTags 함수의 HTML 태그 제거 동작을 검증합니다.
func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// 1. Basic Functionality
		{"Plain text", "Hello World", "Hello World"},
		{"Simple bold tag", "<b>Hello</b> World", "Hello World"},
		{"Tag with attributes", `<a href="http://example.com">Link</a>`, "Link"},
		{"Complex structure", "<div><span><b>Hello</b></span></div>", "Hello"},
		{"Nested tags", "<b><i>BoldItalic</i></b>", "BoldItalic"},
		{"Self-closing tag", "Hello<br/>World", "HelloWorld"},
		{"Multiple tags", "<h1>Title</h1><p>Paragraph</p>", "TitleParagraph"},
		{"Real-world Example", "삼성 갤럭시 <b>S25</b> <b>FE</b> 256GB 자급제", "삼성 갤럭시 S25 FE 256GB 자급제"},

		// 2. Advanced / Edge Case Functionality (Robustness)
		{"HTML Comment", "Hello <!-- comment --> World", "Hello  World"},
		{"HTML Comment with tags", "<div><!-- comment --></div>", ""},
		{"Incomplete Comment", "Hello <!-- comment", "Hello <!-- comment"},
		{"Math operator < (Not a tag)", "3 < 5", "3 < 5"},
		{"Math operator >", "5 > 3", "5 > 3"},
		{"Mixed math and tags", "<b>Values:</b> 3 < 5", "Values: 3 < 5"},

		// 3. HTML Entities
		{"HTML Entities: Ampersand", "Tom &amp; Jerry", "Tom & Jerry"},
		{"HTML Entities: Less Than", "3 &lt; 5", "3 < 5"},
		{"HTML Entities: Greater Than", "5 &gt; 3", "5 > 3"},
		{"HTML Entities: Quote", "&quot;Quote&quot;", "\"Quote\""},
		{"Complex Mix", "Start <b>&lt;Middle&gt;</b> End", "Start <Middle> End"},

		// 4. Complex Attributes (State Machine Verification)
		{"Attribute with single quotes", "<a title='foo'>Link</a>", "Link"},
		{"Attribute with double quotes", `<a title="foo">Link</a>`, "Link"},
		{"Attribute containing > in double quotes", `<a title="Greater > Than">Link</a>`, "Link"},
		{"Attribute containing > in single quotes", `<a title='Greater > Than'>Link</a>`, "Link"},
		{"Attribute containing <", `<div data-val="<">Content</div>`, "Content"},
		{"Nested mixed quotes 1", `<img src="foo.jpg" alt='It"s me'>`, ""},
		{"Nested mixed quotes 2", `<img src='foo.jpg' alt="It's me">`, ""},

		// 5. Fail-Fast & Regression Checks
		{"Tag candidate start with number", "<123>", "<123>"},
		{"Tag candidate start with space", "< a>", "< a>"},
		{"Tag candidate start with symbol", "<$100>", "<$100>"},
		{"Unclosed tag", "<b", "<b"},
		{"Unclosed quote in tag", `<a title="open`, `<a title="open`},
		{"Combo edge case", `Text < 5 but <b>Bold</b> and <a href=">">Link</a>`, `Text < 5 but Bold and Link`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, StripHTMLTags(tt.input))
		})
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

// TestHasAnyContent는 HasAnyContent 함수의 동작을 검증합니다.
func TestHasAnyContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		strs []string
		want bool
	}{
		// [Category 1] 기본 동작
		{"Single non-empty", []string{"hello"}, true},
		{"Single empty", []string{""}, false},
		{"Multiple with content middle", []string{"", "world", ""}, true},

		// [Category 2] 엣지 케이스
		{"Nil slice", nil, false},
		{"Empty slice", []string{}, false},
		{"All empty", []string{"", "", ""}, false},
		{"Whitespace only (Trim not applied)", []string{"   "}, true}, // HasAnyContent does not trim
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := HasAnyContent(tt.strs...)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkToSnakeCase(b *testing.B) {
	input := "ThisIsAVeryLongVariableNameForBenchmarkPurposes123"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ToSnakeCase(input)
	}
}

func BenchmarkNormalizeSpaces(b *testing.B) {
	input := "   This   is   a   test   string   with   many   spaces   "
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NormalizeSpaces(input)
	}
}

func BenchmarkFormatCommas(b *testing.B) {
	input := int64(123456789012345)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatCommas(input)
	}
}

func BenchmarkStripHTMLTags(b *testing.B) {
	input := `
		<html>
			<head><title>Benchmark</title></head>
			<body>
				<h1>Welcome</h1>
				<p>This is a <b>bold</b> paragraph with <a href="#">link</a>.</p>
				<div class="container">
					<span>Nested Content</span>
				</div>
			</body>
		</html>
	`
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = StripHTMLTags(input)
	}
}

func BenchmarkMaskSensitiveData(b *testing.B) {
	input := "1234567890123456"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MaskSensitiveData(input)
	}
}

// =============================================================================
// Examples (Documentation)
// =============================================================================

func ExampleToSnakeCase() {
	fmt.Println(ToSnakeCase("MyVariableName"))
	fmt.Println(ToSnakeCase("HTTPClient"))
	// Output:
	// my_variable_name
	// http_client
}

func ExampleNormalizeSpaces() {
	fmt.Println(NormalizeSpaces("  Hello   World  "))
	// Output: Hello World
}

func ExampleFormatCommas() {
	fmt.Println(FormatCommas(1234567))
	fmt.Println(FormatCommas(100))
	// Output:
	// 1,234,567
	// 100
}

func ExampleStripHTMLTags() {
	htmlStr := "<b>Bold</b> &amp; <i>Italic</i>"
	fmt.Println(StripHTMLTags(htmlStr))
	// Output: Bold & Italic
}

func ExampleMaskSensitiveData() {
	fmt.Println(MaskSensitiveData("1234567890123456"))
	fmt.Println(MaskSensitiveData("secret"))
	// Output:
	// 1234***3456
	// secr***
}
