package strutil

import (
	"fmt"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Space Normalization Tests
// =============================================================================

// TestNormalizeSpace NormalizeSpace 함수의 공백 정규화 동작을 검증합니다.
func TestNormalizeSpace(t *testing.T) {
	t.Parallel()

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
			name: "Multiline string (become single line)",
			s: `
				라인    1
				라인2
				라인3
			`,
			expected: "라인 1 라인2 라인3",
		},
		{name: "Tabs and Newlines", s: "Word1\t\tWord2\n\nWord3", expected: "Word1 Word2 Word3"},
		{name: "Zero Width Space", s: "Hello\u200BWorld", expected: "Hello\u200BWorld"}, // ZWSP is considered a graphic char by Go, not space
		{name: "Ideographic Space", s: "Hello\u3000World", expected: "Hello World"},     // U+3000 is a space
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, NormalizeSpace(c.s))
		})
	}
}

// FuzzNormalizeSpace NormalizeSpace가 어떤 입력에도 패닉하지 않고 일관된 속성을 유지하는지 검증합니다.
func FuzzNormalizeSpace(f *testing.F) {
	f.Add("   Hello   World   ")
	f.Add("\t\n\r")
	f.Add("NoSpaces")

	f.Fuzz(func(t *testing.T, s string) {
		out := NormalizeSpace(s)

		// 속성 1: 결과의 길이는 원본보다 길 수 없음 (공백이 줄어들거나 같으므로)
		// 단, 유효하지 않은 UTF-8 문자열의 경우 range 루프가 RuneError(3바이트)로 변환하여 길이가 늘어날 수 있음
		if utf8.ValidString(s) {
			if len(out) > len(s) {
				t.Errorf("Output longer than valid input: len(out)=%d, len(in)=%d", len(out), len(s))
			}
		}

		// 속성 2: 결과에는 연속된 공백이 없어야 함
		if strings.Contains(out, "  ") {
			t.Errorf("Output contains double spaces: %q", out)
		}

		// 속성 3: 결과의 앞뒤에는 공백이 없어야 함
		if len(out) > 0 {
			if strings.HasPrefix(out, " ") || strings.HasSuffix(out, " ") {
				t.Errorf("Output has leading/trailing spaces: %q", out)
			}
		}

		// 속성 4: 멱등성 (Idempotency) - 이미 정규화된 문자열을 다시 정규화해도 변하지 않아야 함
		out2 := NormalizeSpace(out)
		if out != out2 {
			t.Errorf("Not idempotent: first=%q, second=%q", out, out2)
		}
	})
}

// TestNormalizeMultiline NormalizeMultiline 함수의 여러 줄 공백 정규화 동작을 검증합니다.
func TestNormalizeMultiline(t *testing.T) {
	t.Parallel()

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
			expected: "라인 1\n라인2\n\n라인3\n\n라인4\n\n라인5",
		},
		{
			name: "Complex multiline 2",
			s: ` 라인    1
		
		
			라인2
		
		
			라인3
			라인4
			라인5   `,
			expected: "라인 1\n\n라인2\n\n라인3\n라인4\n라인5",
		},
		{
			name: "Only newlines",
			s: `
					
			`,
			expected: "",
		},
		{
			name: "Values with wide indentation",
			s: `
					Item 1
					Item 2
			`,
			expected: "Item 1\nItem 2",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, NormalizeMultiline(c.s))
		})
	}
}

// =============================================================================
// Number Formatting Tests
// =============================================================================

// TestComma Comma 함수의 숫자 천 단위 구분 기호 포맷팅 동작을 검증합니다.
func TestComma(t *testing.T) {
	t.Parallel()

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
			// Edge Case: MinInt64 on 64-bit arch
			{math.MinInt64, "-9,223,372,036,854,775,808"},
			{math.MaxInt64, "9,223,372,036,854,775,807"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, Comma(tt.input))
		}
	})

	t.Run("int64", func(t *testing.T) {
		tests := []struct {
			input    int64
			expected string
		}{
			{math.MaxInt64, "9,223,372,036,854,775,807"},
			{math.MinInt64, "-9,223,372,036,854,775,808"},
			{-1, "-1"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, Comma(tt.input))
		}
	})

	t.Run("uint", func(t *testing.T) {
		tests := []struct {
			input    uint
			expected string
		}{
			{0, "0"},
			{1000, "1,000"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, Comma(tt.input))
		}
	})

	t.Run("uint64", func(t *testing.T) {
		tests := []struct {
			input    uint64
			expected string
		}{
			{math.MaxUint64, "18,446,744,073,709,551,615"},
			{0, "0"},
		}
		for _, tt := range tests {
			assert.Equal(t, tt.expected, Comma(tt.input))
		}
	})

}

// FuzzComma Comma 함수가 무작위 정수 입력에 대해 패닉하지 않는지 검증합니다.
func FuzzComma(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(1000))
	f.Add(int64(-1000))
	f.Add(int64(math.MaxInt64))
	f.Add(int64(math.MinInt64))

	f.Fuzz(func(t *testing.T, n int64) {
		s := Comma(n)
		if s == "" {
			t.Error("Comma returned empty string")
		}
		// 기본 검증: 1000 이상이면 쉼표가 있어야 함 (절댓값 기준)
		// MinInt64는 Abs 계산 시 오버플로우가 나므로 제외하거나 별도 처리 필요하지만,
		// 여기선 간단히 길이 체크 정도만 수행
		if n > 999 || n < -999 {
			if !strings.Contains(s, ",") {
				t.Errorf("Expected commas for %d, got %q", n, s)
			}
		}
	})
}

// =============================================================================
// String Splitting Tests
// =============================================================================

// TestSplitClean SplitClean 함수의 문자열 분리 및 트림 동작을 검증합니다.
func TestSplitClean(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		s        string
		sep      string
		expected []string
	}{
		{name: "Comma separated", s: "1,2,3", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Comma separated with empty", s: ",1,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Comma separated with spaces", s: ",1,  ,  ,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{name: "Multi-char separator", s: ",1,,2,3,", sep: ",,", expected: []string{",1", "2,3,"}},
		{name: "Separator not found", s: "1,2,3", sep: "-", expected: []string{"1,2,3"}},
		{name: "Empty string", s: "", sep: "-", expected: nil},
		{name: "Only separators", s: ",,,", sep: ",", expected: nil},
		// Empty separator case: strings.Split behavior (split by char)
		// Clean should remove empty strings if any, but char split usually has no empty unless original is empty
		{name: "Empty separator (char split)", s: "ab c", sep: "", expected: []string{"a", "b", "c"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.expected, SplitClean(c.s, c.sep))
		})
	}
}

// =============================================================================
// Sensitive Data Masking Tests
// =============================================================================

// TestMask Mask 함수의 민감 정보 마스킹 동작을 검증합니다.
func TestMask(t *testing.T) {
	t.Parallel()

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
			assert.Equal(t, tt.expected, Mask(tt.input))
		})
	}
}

// =============================================================================
// HTML Tag Stripping Tests
// =============================================================================

// TestStripHTML StripHTML 함수의 HTML 태그 제거 동작을 검증합니다.
func TestStripHTML(t *testing.T) {
	t.Parallel()

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
			assert.Equal(t, tt.expected, StripHTML(tt.input))
		})
	}
}

// FuzzStripHTML StripHTML 함수가 임의의 깨진 HTML 입력에 대해 패닉하지 않는지 검증합니다.
func FuzzStripHTML(f *testing.F) {
	f.Add("<html><body>Hello</body></html>")
	f.Add("<a href='test'>")
	f.Add("<!-- comment -->")
	f.Add("<broken html")

	f.Fuzz(func(t *testing.T, s string) {
		// Garbage In, Garbage Out: 입력이 유효하지 않은 UTF-8이면 출력도 그럴 수 있음.
		// 이 함수는 HTML 태그 제거가 목적이지 인코딩 복구가 목적이 아니므로, 유효한 문자열에 대해서만 검증.
		if !utf8.ValidString(s) {
			return
		}

		out := StripHTML(s)

		// 1. 결과는 유효한 UTF-8이어야 함 (html.UnescapeString 결과물)
		if !utf8.ValidString(out) {
			t.Errorf("Produced invalid UTF-8: %q", out)
		}

		// 2. 결과에 명백한 완전한 태그('<b>', '</div>' 등)가 남아있지 않아야 함
		// 단, '<'나 '>' 자체는 엔티티 디코딩이나 태그가 아닌 문자로 존재할 수 있으므로 느슨하게 검사
		if strings.Contains(out, "<html>") || strings.Contains(out, "</div>") {
			t.Errorf("Output seems to contain tags: %q", out)
		}
	})
}

// =============================================================================
// Helper Function Tests
// =============================================================================

// TestAnyContent AnyContent 함수의 동작을 검증합니다.
func TestAnyContent(t *testing.T) {
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
		{"Whitespace only (Trim applied)", []string{"   "}, false}, // AnyContent trims spaces
		{
			name: "Unicode whitespace",
			strs: []string{"\u3000", "\u200B"},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AnyContent(tt.strs...)
			assert.Equal(t, tt.want, got)
		})
	}
}

// =============================================================================
// Truncate Tests (New Added)
// =============================================================================

// TestTruncate Truncate 함수의 문자열 줄임 동작을 검증합니다.
// 멀티바이트 문자(한글, 이모지 등)와 다양한 엣지 케이스를 포함합니다.
func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		// [Category 1] 기본 동작
		{"Short string", "hello", 10, "hello"},
		{"Exact length", "hello", 5, "hello"},
		{"Long string", "hello world", 5, "hello..."},
		{"Empty string", "", 5, ""},

		// [Category 2] 멀티바이트 (한글)
		{"Korean short", "안녕하세요", 10, "안녕하세요"},
		{"Korean exact", "안녕하세요", 5, "안녕하세요"},
		{"Korean long", "안녕하세요 반갑", 5, "안녕하세요..."},

		// [Category 3] 멀티바이트 (이모지)
		{"Emoji short", "😀😁😂", 10, "😀😁😂"},
		{"Emoji exact", "😀😁😂", 3, "😀😁😂"},
		{"Emoji long", "😀😁😂🤣😃", 3, "😀😁😂..."},

		// [Category 4] 엣지 케이스
		{"Zero limit", "hello", 0, ""},
		{"Negative limit", "hello", -5, ""},
		{"Limit 1", "hello", 1, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Truncate(tt.input, tt.limit); got != tt.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tt.input, tt.limit, got, tt.want)
			}
		})
	}
}

// FuzzTruncate Truncate 함수가 임의의 입력과 길이에 대해 안전하게 동작하는지 검증합니다.
func FuzzTruncate(f *testing.F) {
	f.Add("hello world", 5)
	f.Add("안녕하세요", 2)
	f.Add("😀😁😂", 1)
	f.Add("", 10)

	f.Fuzz(func(t *testing.T, s string, limit int) {
		got := Truncate(s, limit)

		// 1. 길이는 항상 limit + 3 ("...") 이하여야 함 (limit > 0 일 때)
		// Rune count 기준이므로 바이트 길이는 다를 수 있음에 유의
		runeCount := utf8.RuneCountInString(got)
		if limit > 0 {
			if strings.HasSuffix(got, "...") {
				// 원본보다 짧거나 같아야 함 (Rune 수)
				// 잘린 경우 길이는 limit + 3 ("...")
				if runeCount > limit+3 {
					t.Errorf("Result too long: limit=%d, got len=%d (%q)", limit, runeCount, got)
				}
			} else {
				// 잘리지 않은 경우, limit 이하여야 하고 원본과 같아야 함
				if runeCount > limit {
					// 원본 자체가 limit보다 커서 잘려야 했는데 안 잘린 케이스
					// 단, RuneCountInString은 유효하지 않은 UTF-8을 RuneError(1 rune)로 치환하므로
					// 원본이 유효한 UTF-8인 경우만 검증
					if utf8.ValidString(s) {
						t.Errorf("Result should be truncated but wasn't: limit=%d, got len=%d (%q)", limit, runeCount, got)
					}
				}
			}
		} else {
			// limit <= 0 이면 빈 문자열
			if got != "" {
				t.Errorf("Expected empty string for limit=%d, got %q", limit, got)
			}
		}
	})
}

// =============================================================================
// Benchmarks
// =============================================================================

func BenchmarkNormalizeSpace(b *testing.B) {
	input := "   This   is   a   test   string   with   many   spaces   "
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NormalizeSpace(input)
	}
}

func BenchmarkComma(b *testing.B) {
	input := int64(123456789012345)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Comma(input)
	}
}

func BenchmarkStripHTML(b *testing.B) {
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
		_ = StripHTML(input)
	}
}

func BenchmarkMask(b *testing.B) {
	input := "1234567890123456"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Mask(input)
	}
}

func BenchmarkTruncate(b *testing.B) {
	input := "This is a very long string that needs to be truncated for testing purposes."
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Truncate(input, 20)
	}
}

// =============================================================================
// Examples (Documentation)
// =============================================================================

func ExampleNormalizeSpace() {
	fmt.Println(NormalizeSpace("  Hello   World  "))
	// Output: Hello World
}

func ExampleComma() {
	fmt.Println(Comma(1234567))
	fmt.Println(Comma(100))
	// Output:
	// 1,234,567
	// 100
}

func ExampleStripHTML() {
	htmlStr := "<b>Bold</b> &amp; <i>Italic</i>"
	fmt.Println(StripHTML(htmlStr))
	// Output: Bold & Italic
}

func ExampleTruncate() {
	fmt.Println(Truncate("Hello World", 5))
	fmt.Println(Truncate("안녕하세요", 2))
	// Output:
	// Hello...
	// 안녕...
}
