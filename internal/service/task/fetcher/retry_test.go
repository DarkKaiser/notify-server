package fetcher_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRetryFetcher_Do validates the core retry loop and policy enforcement in a black-box manner.
func TestRetryFetcher_Do(t *testing.T) {
	// Common variables
	dummyURL := "http://example.com"
	errNetwork := errors.New("dial tcp: i/o timeout")

	tests := []struct {
		name              string
		method            string
		maxRetries        int
		minRetryDelay     time.Duration
		setupMock         func(*mocks.MockFetcher)
		wantErr           bool
		errCheck          func(error) bool
		expectedCallCount int
	}{
		{
			name:          "Success on first attempt",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("ok", 200), nil).Once()
			},
			wantErr:           false,
			expectedCallCount: 1,
		},
		{
			name:          "Fail with 500 then Success",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				// Fail once
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("error", 500), nil).Once()
				// Success next
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("ok", 200), nil).Once()
			},
			wantErr:           false,
			expectedCallCount: 2,
		},
		{
			name:          "Fail with 429 then Success",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("wait", 429), nil).Once()
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("ok", 200), nil).Once()
			},
			wantErr:           false,
			expectedCallCount: 2,
		},
		{
			name:          "Network Error then Success",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errNetwork).Once()
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("ok", 200), nil).Once()
			},
			wantErr:           false,
			expectedCallCount: 2,
		},
		{
			name:          "Max retries exceeded (Status Codes)",
			method:        http.MethodGet,
			maxRetries:    2,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				// Initial + 2 retries = 3 calls
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("error", 500), nil).Times(3)
			},
			wantErr: true,
			errCheck: func(err error) bool {
				// Cause is ErrMaxRetriesExceeded, so errors.Is works here.
				// Error string is "HTTP 500 ..."
				return errors.Is(err, fetcher.ErrMaxRetriesExceeded) &&
					strings.Contains(err.Error(), "HTTP 500")
			},
			expectedCallCount: 3,
		},
		{
			name:          "Max retries exceeded (Network Errors)",
			method:        http.MethodGet,
			maxRetries:    2,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errNetwork).Times(3)
			},
			wantErr: true,
			errCheck: func(err error) bool {
				// Wrapper creates new AppError, so strict Is(ErrMaxRetriesExceeded) fails.
				// Check type and content.
				return apperrors.Is(err, apperrors.Unavailable) &&
					strings.Contains(err.Error(), errNetwork.Error())
			},
			expectedCallCount: 3,
		},
		{
			name:          "Non-retriable Status Code (404)",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("not found", 404), nil).Once()
			},
			wantErr:           false, // 404 is a valid HTTP response, simply returned
			expectedCallCount: 1,
		},
		{
			name:          "Non-retriable Status Code (501 Not Implemented)",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("not implemented", 501), nil).Once()
			},
			wantErr:           false, // Returning response as-is (unless delegate errors, but here delegate returns resp)
			expectedCallCount: 1,
		},
		{
			name:          "Non-retriable Error (Context Canceled)",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, context.Canceled).Once()
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, context.Canceled)
			},
			expectedCallCount: 1,
		},
		{
			name:          "Non-idempotent method (POST) - Should retry ONLY if status is 429/5xx? No, logic says non-idempotent = retries disabled",
			method:        http.MethodPost,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				// Even if 500, POST should not retry. Retries disabled means maxRetries=0.
				// If query fails (here with 500 status), it returns error due to logic in RetryFetcher enforcing success?
				// Yes, if effectiveMaxRetries=0 and it fails, it returns ErrMaxRetriesExceeded wrapped error.
				m.On("Do", mock.Anything).Return(mocks.NewMockResponse("error", 500), nil).Once()
			},
			wantErr: true, // Expect error because 500 is considered failure and we struck out.
			errCheck: func(err error) bool {
				return errors.Is(err, fetcher.ErrMaxRetriesExceeded)
			},
			expectedCallCount: 1,
		},
		{
			name:          "Context Deadline Exceeded from delegate - Should stop",
			method:        http.MethodGet,
			maxRetries:    3,
			minRetryDelay: time.Millisecond,
			setupMock: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded).Once()
			},
			wantErr: true,
			errCheck: func(err error) bool {
				return errors.Is(err, context.DeadlineExceeded)
			},
			expectedCallCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mocks.MockFetcher{}
			tt.setupMock(mockFetcher)

			// Using 10ms max delay to speed up tests
			f := fetcher.NewRetryFetcher(mockFetcher, tt.maxRetries, tt.minRetryDelay, 10*time.Millisecond)

			req, _ := http.NewRequest(tt.method, dummyURL, nil)
			// Mock GetBody for POST requests just in case, though idempotency check might skip it
			if tt.method == http.MethodPost {
				req.GetBody = func() (io.ReadCloser, error) {
					return io.NopCloser(strings.NewReader("")), nil
				}
			}

			// Some tests need context with timeout? Here we use background mostly.
			if strings.Contains(tt.name, "Deadline") {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				defer cancel()
				req = req.WithContext(ctx)
				time.Sleep(2 * time.Millisecond) // Ensure it expires
			}

			resp, err := f.Do(req)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errCheck != nil {
					assert.True(t, tt.errCheck(err), "Error validation failed: %v", err)
				}
			} else {
				assert.NoError(t, err)
			}

			if resp != nil {
				resp.Body.Close()
			}

			mockFetcher.AssertNumberOfCalls(t, "Do", tt.expectedCallCount)
		})
	}
}

// TestRetryFetcher_ContextCancellation_DuringWait tests that cancellation during the backoff wait aborts the retry.
func TestRetryFetcher_ContextCancellation_DuringWait(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// First call fails
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("temp error")).Once()

	f := fetcher.NewRetryFetcher(mockFetcher, 5, 200*time.Millisecond, 1*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)

	// Cancel context after a short delay, shorter than the retry wait (200ms)
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := f.Do(req)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "Should return context canceled error")
	assert.Less(t, duration, 300*time.Millisecond, "Should return immediately after cancellation")
}

// TestRetryFetcher_GetBody_Lifecycle checks if GetBody is called correctly and errors are handled.
func TestRetryFetcher_GetBody_Lifecycle(t *testing.T) {
	t.Run("GetBody failure aborts retries", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		// First failure to trigger retry logic
		mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("fail", 500), nil).Once()

		f := fetcher.NewRetryFetcher(mockFetcher, 3, time.Millisecond, time.Millisecond)

		req, _ := http.NewRequest(http.MethodGet, "http://example.com", strings.NewReader("body"))
		req.GetBody = func() (io.ReadCloser, error) {
			return nil, errors.New("getBody failed")
		}

		_, err := f.Do(req)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "getBody failed")
		// Should stop after 0 attempts (GetBody called before Do)
		mockFetcher.AssertNumberOfCalls(t, "Do", 0)
	})

	t.Run("GetBody is used to rewind body", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		// Fail twice, then succeed
		mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("fail", 500), nil).Twice()
		mockFetcher.On("Do", mock.Anything).Return(mocks.NewMockResponse("ok", 200), nil).Once()

		f := fetcher.NewRetryFetcher(mockFetcher, 3, time.Millisecond, time.Millisecond)

		getBodyCount := int32(0)
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", strings.NewReader("body"))
		req.GetBody = func() (io.ReadCloser, error) {
			atomic.AddInt32(&getBodyCount, 1)
			return io.NopCloser(strings.NewReader("body")), nil
		}

		_, err := f.Do(req)

		assert.NoError(t, err)
		// 3 calls total (1 initial + 2 retries). GetBody called for ALL 3 attempts.
		assert.Equal(t, int32(3), atomic.LoadInt32(&getBodyCount))
		mockFetcher.AssertNumberOfCalls(t, "Do", 3)
	})

	t.Run("Missing GetBody for checks", func(t *testing.T) {
		// If Body is present but GetBody is nil, NewRetryFetcher logic (or Do logic) might block it if configured
		// Logic in Do: if req.Body != nil && req.GetBody == nil && f.maxRetries > 0 -> Error
		mockFetcher := &mocks.MockFetcher{}
		f := fetcher.NewRetryFetcher(mockFetcher, 3, time.Millisecond, time.Millisecond)

		req, _ := http.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("data"))
		req.GetBody = nil // Explicitly nil

		_, err := f.Do(req)

		assert.Error(t, err)
		assert.Equal(t, fetcher.ErrMissingGetBody, err)
		mockFetcher.AssertNotCalled(t, "Do")
	})
}
