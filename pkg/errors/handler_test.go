package errors

import (
	"errors"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// MockErrorHandler는 테스트용 에러 핸들러입니다.
type MockErrorHandler struct {
	HandledError error
	Called       bool
}

// Handle은 에러를 기록하고 호출 여부를 표시합니다.
func (m *MockErrorHandler) Handle(err error) {
	m.Called = true
	m.HandledError = err
}

// Reset은 MockErrorHandler의 상태를 초기화합니다.
func (m *MockErrorHandler) Reset() {
	m.Called = false
	m.HandledError = nil
}

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
