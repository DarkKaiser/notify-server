package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
