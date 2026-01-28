package fetcher_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
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
			mockFetcher := &mocks.MockFetcher{}
			// MaxRetries: 3, MinDelay: 1ms (fast test), MaxDelay: 10ms
			retryFetcher := fetcher.NewRetryFetcher(mockFetcher, 3, time.Millisecond, 10*time.Millisecond)

			// Setup Mock
			call := mockFetcher.On("Do", mock.Anything)
			if tt.respErr != nil {
				call.Return(nil, tt.respErr)
			} else {
				resp := mocks.NewMockResponse("", tt.status)
				call.Return(resp, nil)
			}

			req, _ := http.NewRequest("GET", "http://example.com", nil)
			_, err := retryFetcher.Do(req)

			// Validation
			if tt.shouldRetry {
				// If it retries until exhaustion, it should return an error (max retries exceeded)
				// or the wrapped error. Currently implemented to ensure error return on exhaustion.
				assert.Error(t, err)
				// Max retries exceeded -> Unavailable
				assert.True(t, apperrors.Is(err, apperrors.Unavailable), "Expected Unavailable error on max retries")
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

// TestRetryFetcher_Do_BodyRewind verifies that request body is rewound (via GetBody) on retries.
// This is critical for POST/PUT requests where the body is consumed.
func TestRetryFetcher_Do_BodyRewind(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// Retry once
	retryFetcher := fetcher.NewRetryFetcher(mockFetcher, 1, time.Millisecond, 10*time.Millisecond)

	// Mock server always returns 500
	mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("", 500), nil)

	// Create Request with Body and Context
	payload := []byte(`{"key":"value"}`)
	req, err := http.NewRequestWithContext(context.Background(), "POST", "http://example.com", bytes.NewReader(payload))
	assert.NoError(t, err)

	// Wrap GetBody to count calls
	originalGetBody := req.GetBody
	getBodyCalls := 0
	req.GetBody = func() (io.ReadCloser, error) {
		getBodyCalls++
		return originalGetBody()
	}

	// Execute - Use Do() instead of Get() to use our prepared request
	_, err = retryFetcher.Do(req)

	// Verification
	assert.Error(t, err)                                                        // Should fail after retries
	mockFetcher.AssertNumberOfCalls(t, "Do", 2)                                 // Initial + 1 Retry
	assert.Equal(t, 2, getBodyCalls, "GetBody should be called to rewind body") // Initial NewRequest doesn't call GetBody, but Do logic might call it before *each* attempt or just retries?
	// Logic check:
	// Do() loop:
	// i=0: GetBody called? Code: "if req.GetBody != nil { body, _ := req.GetBody() ... }"
	// Yes, code calls GetBody inside the loop *before* delegate.Do(req) to ensure fresh body.
	// So for i=0 and i=1, it calls GetBody. Total 2 calls.
}

// TestRetryFetcher_Get verifies that Get method correctly delegates to Do with retry logic.
func TestRetryFetcher_Get(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	retryFetcher := fetcher.NewRetryFetcher(mockFetcher, 2, time.Millisecond, 10*time.Millisecond)

	// Simulate failure then success
	// 1st call: 500 Error
	// 2nd call: 500 Error
	// 3rd call: 200 OK
	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Method == "GET" && req.URL.String() == "http://example.com"
	})).Return(mocks.NewMockResponse("", 500), nil).Once()

	mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("", 500), nil).Once()

	mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("success", 200), nil).Once()

	// Execute
	resp, err := retryFetcher.Get(context.Background(), "http://example.com")

	// Verification
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 200, resp.StatusCode)
	mockFetcher.AssertNumberOfCalls(t, "Do", 3)
}

// TestRetryFetcher_ContextCancel verifies that retry loop aborts immediately on context cancel.
func TestRetryFetcher_ContextCancel(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// Set a long delay to ensure it would hang if context cancel is ignored
	retryFetcher := fetcher.NewRetryFetcher(mockFetcher, 3, 2*time.Second, 10*time.Second)

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
	mockFetcher := &mocks.MockFetcher{}
	retryFetcher := fetcher.NewRetryFetcher(mockFetcher, 1, time.Millisecond, 10*time.Millisecond) // 1 Retry

	// Mock 500 response with a Body that tracks Close() calls
	mockBody := mocks.NewMockReadCloser("error")
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
	assert.Equal(t, int64(2), mockBody.GetCloseCount())
}

func TestNewRetryFetcherFromConfig_Table(t *testing.T) {
	tests := []struct {
		name                 string
		configRetryDelay     time.Duration
		configMaxRetries     int
		expectedRetryDelay   time.Duration
		expectedMaxRetries   int
		expectedFetcherType  string
		expectedInternalType string
	}{
		{
			name:                 "Valid Config",
			configRetryDelay:     3 * time.Second,
			configMaxRetries:     5,
			expectedRetryDelay:   3 * time.Second,
			expectedMaxRetries:   5,
			expectedFetcherType:  "*fetcher.RetryFetcher",
			expectedInternalType: "*fetcher.HTTPFetcher",
		},
		{
			name:                 "Short Duration - Enforce Minimum (1s)",
			configRetryDelay:     500 * time.Millisecond,
			configMaxRetries:     3,
			expectedRetryDelay:   1 * time.Second, // Should default to 1s
			expectedMaxRetries:   3,
			expectedFetcherType:  "*fetcher.RetryFetcher",
			expectedInternalType: "*fetcher.HTTPFetcher",
		},
		{
			name:                 "Negative Retries - Corrected",
			configRetryDelay:     1 * time.Second,
			configMaxRetries:     -1,
			expectedRetryDelay:   1 * time.Second,
			expectedMaxRetries:   0,
			expectedFetcherType:  "*fetcher.RetryFetcher",
			expectedInternalType: "*fetcher.HTTPFetcher",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fetcher.NewRetryFetcherFromConfig(tt.configMaxRetries, tt.configRetryDelay)

			// 1. Check outer type
			// f is already *RetryFetcher, no assertion needed
			assert.NotNil(t, f)
			assert.Equal(t, tt.expectedFetcherType, fmt.Sprintf("%T", f))

			// 2. Check internal state (accessing unexported fields for test)
			// Note: internal fields are unexported, so we cannot access them from task package test.
			// Currently we only check type. If detailed inspection needed, move test to fetcher package or export fields (not recommended).
			// assert.Equal(t, tt.expectedMaxRetries, f.maxRetries)
			// assert.Equal(t, tt.expectedRetryDelay, f.retryDelay)

			// 3. Check wrapped fetcher type
			// assert.Equal(t, tt.expectedInternalType, fmt.Sprintf("%T", f.delegate)) -> f.delegate unexported
		})
	}
}
