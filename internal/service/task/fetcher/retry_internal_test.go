package fetcher

import (
	"context"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

// dummyFetcher is a local stub to avoid import cycles with the mocks package.
type dummyFetcher struct{}

func (d *dummyFetcher) Do(req *http.Request) (*http.Response, error) {
	return nil, nil
}

// TestNewRetryFetcher_Validation validates the constructor's logic for clamping parameters.
func TestNewRetryFetcher_Validation(t *testing.T) {
	mockDelegate := &dummyFetcher{}

	tests := []struct {
		name               string
		inputMaxRetries    int
		inputMinDelay      time.Duration
		inputMaxDelay      time.Duration
		expectedMaxRetries int
		expectedMinDelay   time.Duration
		expectedMaxDelay   time.Duration
	}{
		{
			name:               "Normal values",
			inputMaxRetries:    3,
			inputMinDelay:      2 * time.Second,
			inputMaxDelay:      10 * time.Second,
			expectedMaxRetries: 3,
			expectedMinDelay:   2 * time.Second,
			expectedMaxDelay:   10 * time.Second,
		},
		{
			name:               "MaxRetries too low (< 0)",
			inputMaxRetries:    -1,
			inputMinDelay:      time.Second,
			inputMaxDelay:      time.Second,
			expectedMaxRetries: 0,
			expectedMinDelay:   time.Second,
			expectedMaxDelay:   time.Second,
		},
		{
			name:               "MaxRetries too high (> 10)",
			inputMaxRetries:    100,
			inputMinDelay:      time.Second,
			inputMaxDelay:      time.Second,
			expectedMaxRetries: 10,
			expectedMinDelay:   time.Second,
			expectedMaxDelay:   time.Second,
		},
		{
			name:               "MinRetryDelay too short (< 1s)",
			inputMaxRetries:    3,
			inputMinDelay:      100 * time.Millisecond,
			inputMaxDelay:      10 * time.Second,
			expectedMaxRetries: 3,
			expectedMinDelay:   1 * time.Second,
			expectedMaxDelay:   10 * time.Second,
		},
		{
			name:               "MaxRetryDelay is zero (should use default)",
			inputMaxRetries:    3,
			inputMinDelay:      time.Second,
			inputMaxDelay:      0,
			expectedMaxRetries: 3,
			expectedMinDelay:   time.Second,
			expectedMaxDelay:   30 * time.Second, // defaultMaxRetryDelay
		},
		{
			name:               "MaxRetryDelay < MinRetryDelay (should clamp to min)",
			inputMaxRetries:    3,
			inputMinDelay:      5 * time.Second,
			inputMaxDelay:      2 * time.Second,
			expectedMaxRetries: 3,
			expectedMinDelay:   5 * time.Second,
			expectedMaxDelay:   5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewRetryFetcher(mockDelegate, tt.inputMaxRetries, tt.inputMinDelay, tt.inputMaxDelay)

			assert.Equal(t, tt.expectedMaxRetries, f.maxRetries, "maxRetries mismatch")
			assert.Equal(t, tt.expectedMinDelay, f.minRetryDelay, "minRetryDelay mismatch")
			assert.Equal(t, tt.expectedMaxDelay, f.maxRetryDelay, "maxRetryDelay mismatch")
		})
	}
}

// TestIsRetriable tests the error classification logic.
func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "Context canceled",
			err:      context.Canceled,
			expected: false,
		},
		{
			name:     "Context deadline exceeded (raw)",
			err:      context.DeadlineExceeded,
			expected: true, // Should be treated as temporary
		},
		{
			name:     "URL Error - 10 redirects",
			err:      &url.Error{Err: errors.New("stopped after 10 redirects")},
			expected: false,
		},
		{
			name:     "URL Error - invalid control character",
			err:      &url.Error{Err: errors.New("invalid control character in URL")},
			expected: false,
		},
		{
			name:     "URL Error - unsupported protocol",
			err:      &url.Error{Err: errors.New("unsupported protocol scheme \"ftp\"")},
			expected: false,
		},
		{
			name:     "URL Error - other (generic)",
			err:      &url.Error{Err: errors.New("some other error")},
			expected: true, // Default to retriable if not specifically excluded
		},
		{
			name:     "Cert Error - HostnameError",
			err:      x509.HostnameError{},
			expected: false,
		},
		{
			name:     "Cert Error - UnknownAuthorityError",
			err:      x509.UnknownAuthorityError{},
			expected: false,
		},
		{
			name:     "Cert Error - CertificateInvalidError",
			err:      x509.CertificateInvalidError{},
			expected: false,
		},
		{
			name:     "Net Error - Timeout",
			err:      &net.OpError{Err: context.DeadlineExceeded}, // Simulating timeout
			expected: true,
		},
		{
			name:     "AppError - Unavailable (generic)",
			err:      apperrors.New(apperrors.Unavailable, "generic unavailable"),
			expected: true,
		},
		{
			name:     "AppError - Wrapped HTTPStatusError 500",
			err:      apperrors.Wrap(&HTTPStatusError{StatusCode: 500}, apperrors.Unavailable, "wrapped 500"),
			expected: true,
		},
		{
			name:     "AppError - Wrapped HTTPStatusError 501 (Not Implemented)",
			err:      apperrors.Wrap(&HTTPStatusError{StatusCode: http.StatusNotImplemented}, apperrors.Unavailable, "wrapped 501"),
			expected: false,
		},
		{
			name:     "AppError - Wrapped HTTPStatusError 505 (Version Not Supported)",
			err:      apperrors.Wrap(&HTTPStatusError{StatusCode: http.StatusHTTPVersionNotSupported}, apperrors.Unavailable, "wrapped 505"),
			expected: false,
		},
		{
			name:     "AppError - ExecutionFailed",
			err:      apperrors.New(apperrors.ExecutionFailed, "execution failed"),
			expected: false,
		},
		{
			name:     "AppError - InvalidInput",
			err:      apperrors.New(apperrors.InvalidInput, "invalid input"),
			expected: false,
		},
		{
			name:     "AppError - Forbidden",
			err:      apperrors.New(apperrors.Forbidden, "forbidden"),
			expected: false,
		},
		{
			name:     "AppError - NotFound",
			err:      apperrors.New(apperrors.NotFound, "not found"),
			expected: false,
		},
		{
			name:     "Unknown generic error",
			err:      errors.New("random explosion"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isRetriable(tt.err))
		})
	}
}

// TestIsIdempotentMethod tests method idempotency classification.
func TestIsIdempotentMethod(t *testing.T) {
	tests := []struct {
		method   string
		expected bool
	}{
		{http.MethodGet, true},
		{http.MethodHead, true},
		{http.MethodOptions, true},
		{http.MethodTrace, true},
		{http.MethodPut, true},
		{http.MethodDelete, true},
		{http.MethodPost, false},
		{http.MethodPatch, false},
		{"INVALID_METHOD", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			assert.Equal(t, tt.expected, isIdempotentMethod(tt.method))
		})
	}
}

// TestParseRetryAfter tests the parsing of Retry-After headers.
func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name          string
		value         string
		expectedDelay time.Duration
		expectedValid bool
		delta         time.Duration // tolerance for time comparisons
	}{
		{
			name:          "Empty value",
			value:         "",
			expectedDelay: 0,
			expectedValid: false,
		},
		{
			name:          "Seconds format",
			value:         "120",
			expectedDelay: 120 * time.Second,
			expectedValid: true,
		},
		{
			name:          "Seconds format (zero)",
			value:         "0",
			expectedDelay: 0,
			expectedValid: true,
		},
		{
			name:          "Seconds format (negative - invalid strictly but scanf might parse, logic checks >=0)",
			value:         "-10",
			expectedDelay: 0,
			expectedValid: false,
		},
		{
			name:          "HTTP Date format (Future)",
			value:         time.Now().UTC().Add(1 * time.Hour).Format(http.TimeFormat),
			expectedDelay: 1 * time.Hour,
			expectedValid: true,
			delta:         1 * time.Second,
		},
		{
			name:          "HTTP Date format (Past)",
			value:         time.Now().UTC().Add(-1 * time.Hour).Format(http.TimeFormat),
			expectedDelay: 0,
			expectedValid: true,
		},
		{
			name:          "Invalid format",
			value:         "tomorrow",
			expectedDelay: 0,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			delay, valid := parseRetryAfter(tt.value)
			assert.Equal(t, tt.expectedValid, valid, "validity mismatch")
			if tt.expectedValid {
				if tt.delta > 0 {
					assert.InDelta(t, tt.expectedDelay, delay, float64(tt.delta), "delay duration valid but outside delta")
				} else {
					assert.Equal(t, tt.expectedDelay, delay, "delay duration mismatch")
				}
			}
		})
	}
}
