package fetcher

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPStatusError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *HTTPStatusError
		expected string
	}{
		{
			name: "Minimal fields",
			err: &HTTPStatusError{
				StatusCode: http.StatusNotFound,
				Status:     "404 Not Found",
			},
			expected: "HTTP 404 (404 Not Found)",
		},
		{
			name: "With URL",
			err: &HTTPStatusError{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				URL:        "https://example.com/api",
			},
			expected: "HTTP 500 (500 Internal Server Error) URL: https://example.com/api",
		},
		{
			name: "With BodySnippet",
			err: &HTTPStatusError{
				StatusCode:  http.StatusBadRequest,
				Status:      "400 Bad Request",
				BodySnippet: "invalid input",
			},
			expected: "HTTP 400 (400 Bad Request), Body: invalid input",
		},
		{
			name: "With Cause",
			err: &HTTPStatusError{
				StatusCode: http.StatusForbidden,
				Status:     "403 Forbidden",
				Cause:      errors.New("access denied"),
			},
			expected: "HTTP 403 (403 Forbidden): access denied",
		},
		{
			name: "With All Fields",
			err: &HTTPStatusError{
				StatusCode:  http.StatusTeapot,
				Status:      "418 I'm a teapot",
				URL:         "https://example.com/brew",
				BodySnippet: "short and stout",
				Cause:       errors.New("cannot brew coffee"),
			},
			expected: "HTTP 418 (418 I'm a teapot) URL: https://example.com/brew, Body: short and stout: cannot brew coffee",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestHTTPStatusError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	err := &HTTPStatusError{
		StatusCode: 500,
		Status:     "Internal Server Error",
		Cause:      cause,
	}

	// Direct Unwrap
	assert.Equal(t, cause, err.Unwrap())

	// errors.Unwrap
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestHTTPStatusError_ErrorChaining(t *testing.T) {
	sentinelErr := errors.New("sentinel error")

	err := &HTTPStatusError{
		StatusCode: 500,
		Status:     "Internal Server Error",
		Cause:      sentinelErr,
	}

	// errors.Is check
	assert.True(t, errors.Is(err, sentinelErr), "Should wrap sentinel error")

	// errors.As check
	var target *HTTPStatusError
	assert.True(t, errors.As(err, &target), "Should be castable to HTTPStatusError")
	assert.Equal(t, 500, target.StatusCode)
}

func TestHTTPStatusError_As_Target(t *testing.T) {
	// Scenario: Wrapped inside another error
	rootErr := &HTTPStatusError{
		StatusCode: 404,
		Status:     "Not Found",
	}
	wrappedErr := fmt.Errorf("wrapper: %w", rootErr)

	// Verify extraction via errors.As
	var extracted *HTTPStatusError
	require.True(t, errors.As(wrappedErr, &extracted))
	assert.Equal(t, 404, extracted.StatusCode)
	assert.Equal(t, "Not Found", extracted.Status)
}
