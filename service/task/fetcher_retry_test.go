package task

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRetryFetcher_Do_RetryLogic tests the core retry decision logic using a table-driven approach.
func TestRetryFetcher_Do_RetryLogic(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		respErr       error
		shouldRetry   bool
		expectedCalls int
	}{
		{
			name:          "Success (200) - No Retry",
			status:        http.StatusOK,
			shouldRetry:   false,
			expectedCalls: 1,
		},
		{
			name:          "Not Found (404) - No Retry",
			status:        http.StatusNotFound,
			shouldRetry:   false,
			expectedCalls: 1,
		},
		{
			name:          "Bad Request (400) - No Retry",
			status:        http.StatusBadRequest,
			shouldRetry:   false,
			expectedCalls: 1,
		},
		{
			name:          "Internal Server Error (500) - Retry",
			status:        http.StatusInternalServerError,
			shouldRetry:   true,
			expectedCalls: 4, // Initial + 3 Retries
		},
		{
			name:          "Too Many Requests (429) - Retry",
			status:        http.StatusTooManyRequests,
			shouldRetry:   true,
			expectedCalls: 4,
		},
		{
			name:          "Network Error - Retry",
			respErr:       errors.New("connection refused"),
			shouldRetry:   true,
			expectedCalls: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &TestMockFetcher{}
			// MaxRetries: 3, MinDelay: 1ms (fast test)
			retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

			// Setup Mock
			call := mockFetcher.On("Do", mock.Anything)
			if tt.respErr != nil {
				call.Return(nil, tt.respErr)
			} else {
				resp := NewMockResponse("", tt.status)
				call.Return(resp, nil)
			}

			req, _ := http.NewRequest("GET", "http://example.com", nil)
			_, err := retryFetcher.Do(req)

			// Validation
			if tt.shouldRetry {
				// If it retries until exhaustion, it should return an error (max retries exceeded)
				// or the wrapped error. Currently implemented to ensure error return on exhaustion.
				assert.Error(t, err)
			} else {
				if tt.status >= 400 && tt.status != 429 {
					// 4xx errors (except 429) are considered effective success in terms of transport
					// so Do returns (resp, nil)
					assert.NoError(t, err)
				}
			}

			mockFetcher.AssertNumberOfCalls(t, "Do", tt.expectedCalls)
		})
	}
}

// TestRetryFetcher_ContextCancel verifies that retry loop aborts immediately on context cancel.
func TestRetryFetcher_ContextCancel(t *testing.T) {
	mockFetcher := &TestMockFetcher{}
	// Set a long delay to ensure it would hang if context cancel is ignored
	retryFetcher := NewRetryFetcher(mockFetcher, 3, 2*time.Second)

	// Mock always fails
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("fail"))

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel strictly after start but before delay finishes
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

	start := time.Now()
	_, err := retryFetcher.Do(req)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "Expected context.Canceled error")

	// Should return fast (approx 50ms + overhead), definitely not 2s
	assert.Less(t, duration, 500*time.Millisecond, "Should abort retry wait immediately")

	// Should have tried at least once
	mockFetcher.AssertNumberOfCalls(t, "Do", 1)
}

// TestRetryFetcher_BodyClose verifies that response bodies are closed on retry-triggering failures
// to prevent file descriptor leaks.
func TestRetryFetcher_BodyClose(t *testing.T) {
	mockFetcher := &TestMockFetcher{}
	retryFetcher := NewRetryFetcher(mockFetcher, 1, time.Millisecond) // 1 Retry

	// Mock 500 response with a Body that tracks Close() calls
	mockBody := &MockReadCloser{data: bytes.NewBufferString("error")}
	resp := &http.Response{
		StatusCode: 500,
		Status:     "500 Server Error",
		Body:       mockBody,
	}

	// Always return the same resp behavior
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	retryFetcher.Do(req)

	// Should be closed twice:
	// 1. After first attempt fails (500)
	// 2. After retry attempt fails (500)
	assert.Equal(t, 2, mockBody.closeCount)
}

// MockReadCloser is a helper for tracking Close calls
type MockReadCloser struct {
	data       *bytes.Buffer
	closeCount int
}

func (m *MockReadCloser) Read(p []byte) (n int, err error) {
	return m.data.Read(p)
}

func (m *MockReadCloser) Close() error {
	m.closeCount++
	return nil
}
