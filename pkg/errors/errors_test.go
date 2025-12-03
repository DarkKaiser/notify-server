package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	err := New(ErrInvalidInput, "invalid input")
	assert.Error(t, err)
	assert.Equal(t, "invalid input", err.Error())
	assert.Equal(t, ErrInvalidInput, GetType(err))
}

func TestWrap(t *testing.T) {
	originalErr := errors.New("original error")
	err := Wrap(originalErr, ErrInternal, "internal error")

	assert.Error(t, err)
	assert.Equal(t, "internal error: original error", err.Error())
	assert.Equal(t, ErrInternal, GetType(err))
	assert.Equal(t, originalErr, Cause(err))
	assert.Equal(t, originalErr, errors.Unwrap(err))
}

func TestIs(t *testing.T) {
	err := New(ErrNotFound, "not found")
	assert.True(t, Is(err, ErrNotFound))
	assert.False(t, Is(err, ErrInternal))

	wrappedErr := Wrap(err, ErrInternal, "internal error")
	// Is checks the type of the *AppError* itself, not the wrapped error's type if the outer is AppError
	// In our implementation, Is checks if the error is an AppError and if its Type matches.
	assert.True(t, Is(wrappedErr, ErrInternal))
	assert.False(t, Is(wrappedErr, ErrNotFound))
}

func TestAs(t *testing.T) {
	err := New(ErrForbidden, "forbidden")
	var appErr *AppError
	assert.True(t, As(err, &appErr))
	assert.Equal(t, ErrForbidden, appErr.Type)
}

func TestGetType(t *testing.T) {
	err := New(ErrUnauthorized, "unauthorized")
	assert.Equal(t, ErrUnauthorized, GetType(err))

	stdErr := errors.New("std error")
	assert.Equal(t, ErrUnknown, GetType(stdErr))
}

func TestCause(t *testing.T) {
	rootErr := errors.New("root error")
	err := Wrap(rootErr, ErrInternal, "wrapped")
	assert.Equal(t, rootErr, Cause(err))

	assert.Nil(t, Cause(rootErr))
}
