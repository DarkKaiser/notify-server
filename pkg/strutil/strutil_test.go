package strutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase(t *testing.T) {
	cases := []struct {
		str      string
		expected string
	}{
		{str: "My", expected: "my"},
		{str: "123", expected: "123"},
		{str: "123abc", expected: "123abc"},
		{str: "123abcDef", expected: "123abc_def"},
		{str: "123abcDefGHI", expected: "123abc_def_ghi"},
		{str: "123abcDefGHIj", expected: "123abc_def_gh_ij"},
		{str: "123abcDefGHIjK", expected: "123abc_def_gh_ij_k"},
		{str: "MyNameIsTom", expected: "my_name_is_tom"},
		{str: "myNameIsTom", expected: "my_name_is_tom"},
		{str: " myNameIsTom ", expected: " my_name_is_tom "},
		{str: " myNameIsTom  yourNameIsB", expected: " my_name_is_tom  your_name_is_b"},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, ToSnakeCase(c.str))
	}
}

func TestTrim(t *testing.T) {
	cases := []struct {
		s        string
		expected string
	}{
		{s: "테스트", expected: "테스트"},
		{s: "   테스트   ", expected: "테스트"},
		{s: "   하나 공백   ", expected: "하나 공백"},
		{s: "   다수    공백   ", expected: "다수 공백"},
		{s: "   다수    공백   여러개   ", expected: "다수 공백 여러개"},
		{s: "   @    특수문자   $   ", expected: "@ 특수문자 $"},
		{s: `
		
				라인    1
				라인2
		
		
				라인3
		
				라인4
		
		
				라인5

			`,
			expected: "라인 1 라인2 라인3 라인4 라인5"},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, NormalizeSpaces(c.s))
	}
}

func TestNormalizeMultiLineSpaces(t *testing.T) {
	cases := []struct {
		s        string
		expected string
	}{
		{s: "", expected: ""},
		{s: "   ", expected: ""},
		{s: "  a  ", expected: "a"},
		{s: `
		
				라인    1
				라인2
		
		
				라인3
		
				라인4
		
		
		
				라인5
		
		
			`,
			expected: "라인 1\r\n라인2\r\n\r\n라인3\r\n\r\n라인4\r\n\r\n라인5"},
		{s: ` 라인    1
		
		
			라인2
		
		
			라인3
			라인4
			라인5   `,
			expected: "라인 1\r\n\r\n라인2\r\n\r\n라인3\r\n라인4\r\n라인5"},
		{s: `


			`,
			expected: ""},
		{s: `

			1

			`,
			expected: "1"},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, NormalizeMultiLineSpaces(c.s))
	}
}

func TestFormatCommas(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		assert.Equal(t, "0", FormatCommas(0))
		assert.Equal(t, "100", FormatCommas(100))
		assert.Equal(t, "1,000", FormatCommas(1000))
		assert.Equal(t, "1,234,567", FormatCommas(1234567))
		assert.Equal(t, "-1,234,567", FormatCommas(-1234567))
	})

	t.Run("int64", func(t *testing.T) {
		assert.Equal(t, "9,223,372,036,854,775,807", FormatCommas(int64(9223372036854775807)))
		assert.Equal(t, "-9,223,372,036,854,775,808", FormatCommas(int64(-9223372036854775808)))
	})

	t.Run("uint", func(t *testing.T) {
		assert.Equal(t, "1,000", FormatCommas(uint(1000)))
	})

	t.Run("uint64", func(t *testing.T) {
		assert.Equal(t, "18,446,744,073,709,551,615", FormatCommas(uint64(18446744073709551615)))
	})
}

func TestSplitAndTrim(t *testing.T) {
	var notAssign []string

	cases := []struct {
		s        string
		sep      string
		expected []string
	}{
		{s: "1,2,3", sep: ",", expected: []string{"1", "2", "3"}},
		{s: ",1,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{s: ",1,  ,  ,2,3,,,", sep: ",", expected: []string{"1", "2", "3"}},
		{s: ",1,,2,3,", sep: "", expected: []string{",", "1", ",", ",", "2", ",", "3", ","}},
		{s: ",1,,2,3,", sep: ",,", expected: []string{",1", "2,3,"}},
		{s: "1,2,3", sep: "-", expected: []string{"1,2,3"}},
		{s: "", sep: "-", expected: notAssign},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, SplitAndTrim(c.s, c.sep))
	}
}

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"빈 문자열", "", ""},
		{"3자 이하 (1자)", "a", "***"},
		{"3자 이하 (2자)", "ab", "***"},
		{"3자 이하 (3자)", "abc", "***"},
		{"12자 이하 (4자)", "abcd", "abcd***"},
		{"12자 이하 (12자)", "123456789012", "1234***"},
		{"긴 문자열 (토큰)", "123456789:ABCdefGHIjklMNOpqrsTUVwxyz", "1234***wxyz"},
		{"긴 문자열 (일반)", "this_is_a_very_long_secret_key", "this***_key"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveData(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
