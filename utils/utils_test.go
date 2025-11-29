package utils

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestCheckErr(t *testing.T) {
	cases := []struct {
		err      error
		expected bool
	}{
		{
			err:      nil,
			expected: false,
		}, {
			err:      errors.New("error"),
			expected: true,
		},
	}

	defer func() { log.StandardLogger().ExitFunc = nil }()

	var occurredFatal bool
	log.StandardLogger().ExitFunc = func(int) { occurredFatal = true }

	for _, c := range cases {
		occurredFatal = false
		CheckErr(c.err)

		assert.Equal(t, c.expected, occurredFatal)
	}
}

// TestCheckErr_WithMockHandler는 의존성 주입 패턴을 사용한 테스트입니다.
func TestCheckErr_WithMockHandler(t *testing.T) {
	t.Run("에러가 있을 때 핸들러 호출", func(t *testing.T) {
		mock := &MockErrorHandler{}
		SetErrorHandler(mock)
		defer ResetErrorHandler()

		testErr := errors.New("test error")
		CheckErr(testErr)

		assert.True(t, mock.Called, "에러 핸들러가 호출되어야 합니다")
		assert.Equal(t, testErr, mock.HandledError, "핸들된 에러가 일치해야 합니다")
	})

	t.Run("에러가 없을 때 핸들러 미호출", func(t *testing.T) {
		mock := &MockErrorHandler{}
		SetErrorHandler(mock)
		defer ResetErrorHandler()

		CheckErr(nil)

		assert.False(t, mock.Called, "에러가 없으면 핸들러가 호출되지 않아야 합니다")
		assert.Nil(t, mock.HandledError, "핸들된 에러가 없어야 합니다")
	})
}

func TestSetErrorHandler(t *testing.T) {
	mock := &MockErrorHandler{}
	SetErrorHandler(mock)
	defer ResetErrorHandler()

	testErr := errors.New("handler test")
	CheckErr(testErr)

	assert.True(t, mock.Called, "설정한 핸들러가 호출되어야 합니다")
	assert.Equal(t, testErr, mock.HandledError, "에러가 올바르게 전달되어야 합니다")
}

func TestResetErrorHandler(t *testing.T) {
	mock := &MockErrorHandler{}
	SetErrorHandler(mock)

	ResetErrorHandler()

	// 리셋 후 새로운 핸들러로 테스트
	newMock := &MockErrorHandler{}
	SetErrorHandler(newMock)
	defer ResetErrorHandler()

	CheckErr(errors.New("test"))
	assert.True(t, newMock.Called, "새로운 핸들러가 정상 작동해야 합니다")
}

func TestMockErrorHandler_Reset(t *testing.T) {
	mock := &MockErrorHandler{}

	testErr := errors.New("test")
	mock.Handle(testErr)

	assert.True(t, mock.Called, "호출되어야 함")
	assert.Equal(t, testErr, mock.HandledError, "에러가 저장되어야 함")

	mock.Reset()

	assert.False(t, mock.Called, "리셋 후 Called는 false여야 함")
	assert.Nil(t, mock.HandledError, "리셋 후 HandledError는 nil이어야 함")
}

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

func TestContains(t *testing.T) {
	cases := []struct {
		list     []string
		item     string
		expected bool
	}{
		{list: []string{"A1", "B1", "C1"}, item: "", expected: false},
		{list: []string{"A1", "B1", "C1"}, item: "A1", expected: true},
		{list: []string{"A1", "B1", "C1"}, item: "a1", expected: false},
		{list: []string{"A1", "B1", "C1"}, item: "A2", expected: false},
		{list: []string{"A1", "B1", "C1"}, item: "A1 ", expected: false},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, Contains(c.list, c.item))
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
		assert.Equal(t, c.expected, Trim(c.s))
	}
}

func TestTrimMultiLine(t *testing.T) {
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
		assert.Equal(t, c.expected, TrimMultiLine(c.s))
	}
}

func TestFormatCommas(t *testing.T) {
	cases := []struct {
		num      int
		expected string
	}{
		{num: 0, expected: "0"},
		{num: 100, expected: "100"},
		{num: 1000, expected: "1,000"},
		{num: 1234567, expected: "1,234,567"},
		{num: -1234567, expected: "-1,234,567"},
	}

	for _, c := range cases {
		assert.Equal(t, c.expected, FormatCommas(c.num))
	}
}

func TestSplitExceptEmptyItems(t *testing.T) {
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
		assert.Equal(t, c.expected, SplitExceptEmptyItems(c.s, c.sep))
	}
}
