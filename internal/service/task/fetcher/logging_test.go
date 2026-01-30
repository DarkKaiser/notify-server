package fetcher_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestLoggingFetcher_Do tests various scenarios for LoggingFetcher.Do
// including successful requests, errors, and URL redaction.
func TestLoggingFetcher_Do(t *testing.T) {
	tests := []struct {
		name          string
		reqURL        string
		reqMethod     string
		mockSetup     func(*mocks.MockFetcher)
		expectedLevel logrus.Level
		expectedMsg   string
		checkFields   func(*testing.T, logrus.Fields)
		expectedError string
	}{
		{
			name:      "Success (Debug Level)",
			reqURL:    "http://example.com/ok",
			reqMethod: http.MethodGet,
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: 200,
					Status:     "200 OK",
				}, nil)
			},
			expectedLevel: logrus.DebugLevel,
			expectedMsg:   "HTTP 요청 성공: 정상 처리 완료",
			checkFields: func(t *testing.T, f logrus.Fields) {
				assert.Equal(t, "GET", f["method"])
				assert.Equal(t, "http://example.com/ok", f["url"])
				assert.Equal(t, 200, f["status_code"])
				assert.Equal(t, "200 OK", f["status"])
				assert.NotEmpty(t, f["duration"])
				assert.Equal(t, "task.fetcher", f["component"])
			},
		},
		{
			name:      "Error (Error Level)",
			reqURL:    "http://example.com/error",
			reqMethod: http.MethodPost,
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errors.New("network fail"))
			},
			expectedLevel: logrus.ErrorLevel,
			expectedMsg:   "HTTP 요청 실패: 요청 처리 중 에러 발생",
			expectedError: "network fail",
			checkFields: func(t *testing.T, f logrus.Fields) {
				assert.Equal(t, "POST", f["method"])
				assert.Equal(t, "network fail", f["error"])
				assert.Equal(t, "task.fetcher", f["component"])
			},
		},
		{
			name:      "Error with Response (Error Level + Status)",
			reqURL:    "http://example.com/500",
			reqMethod: http.MethodGet,
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: 500,
					Status:     "500 Internal Server Error",
				}, errors.New("server error"))
			},
			expectedLevel: logrus.ErrorLevel,
			expectedMsg:   "HTTP 요청 실패: 요청 처리 중 에러 발생",
			expectedError: "server error",
			checkFields: func(t *testing.T, f logrus.Fields) {
				assert.Equal(t, 500, f["status_code"])
				assert.Equal(t, "server error", f["error"])
			},
		},
		{
			name:      "URL Redaction (Sensitive Data)",
			reqURL:    "http://user:pass@example.com/path?token=secret",
			reqMethod: http.MethodGet,
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(&http.Response{
					StatusCode: 200,
				}, nil)
			},
			expectedLevel: logrus.DebugLevel,
			expectedMsg:   "HTTP 요청 성공: 정상 처리 완료",
			checkFields: func(t *testing.T, f logrus.Fields) {
				// Assert that password and query params are redacted
				assert.NotContains(t, f["url"], "pass")
				assert.NotContains(t, f["url"], "secret")
				assert.Contains(t, f["url"], "xxxxx") // RedactURL uses "xxxxx" for masking
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup Log Hook
			hook := &testHook{}
			logrus.AddHook(hook)
			defer func() {
				// Clean up hook (though in parallel tests we might need better isolation if running parallel)
				// logging tests run sequentially here by default
				logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
			}()
			logrus.SetLevel(logrus.DebugLevel) // Ensure Debug logs are captured

			// Setup Mock
			mockFetcher := &mocks.MockFetcher{}
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			f := fetcher.NewLoggingFetcher(mockFetcher)
			req, _ := http.NewRequest(tt.reqMethod, tt.reqURL, nil)

			// Execute
			_, err := f.Do(req)

			// Verify Error
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify Log
			require.NotEmpty(t, hook.entries, "Log entry should be recorded")
			lastEntry := hook.entries[len(hook.entries)-1]

			assert.Equal(t, tt.expectedLevel, lastEntry.Level)
			assert.Equal(t, tt.expectedMsg, lastEntry.Message)

			if tt.checkFields != nil {
				tt.checkFields(t, lastEntry.Data)
			}

			// Verify Context Propagation (Basic)
			// logging.go: WithContext(req.Context()) is called.
			// logrus entry doesn't expose Context easily in Data map unless specially handled,
			// but we can assume if code calls WithContext it works if pkg/log tests verify wrapper.
			// Here we focus on fields.
		})
	}
}

// TestLoggingFetcher_TimeMeasurement verifies that duration is roughly correct.
func TestLoggingFetcher_TimeMeasurement(t *testing.T) {
	hook := &testHook{}
	logrus.AddHook(hook)
	defer logrus.StandardLogger().ReplaceHooks(make(logrus.LevelHooks))
	logrus.SetLevel(logrus.DebugLevel)

	mockFetcher := &mocks.MockFetcher{}
	mockFetcher.On("Do", mock.Anything).Run(func(args mock.Arguments) {
		time.Sleep(10 * time.Millisecond)
	}).Return(&http.Response{StatusCode: 200}, nil)

	f := fetcher.NewLoggingFetcher(mockFetcher)
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	f.Do(req)

	require.NotEmpty(t, hook.entries)
	entry := hook.entries[0]
	durationStr, ok := entry.Data["duration"].(string)
	require.True(t, ok)
	duration, err := time.ParseDuration(durationStr)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, duration, 10*time.Millisecond)
}

// TestLoggingFetcher_ContextPropagation verifies context values are passed to log fields if supported
// (Normally pkg/log handles this, here we ensure metadata is attached)
func TestLoggingFetcher_ContextPropagation(t *testing.T) {
	// applog.WithContext is called. We trust applog tests for that.
	// But we can check if req context is passed to delegate.
	mockFetcher := &mocks.MockFetcher{}
	mockFetcher.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		return r.Context().Value("key") == "val"
	})).Return(&http.Response{StatusCode: 200}, nil)

	f := fetcher.NewLoggingFetcher(mockFetcher)
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	ctx := context.WithValue(req.Context(), "key", "val")

	f.Do(req.Clone(ctx))

	mockFetcher.AssertExpectations(t)
}

// =============================================================================
// Helper
// =============================================================================

// Reuse testHook from pkg/log if possible, but package is different.
// Simple local hook implementation.
type testHook struct {
	entries []*logrus.Entry
}

func (h *testHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *testHook) Fire(e *logrus.Entry) error {
	h.entries = append(h.entries, e)
	return nil
}
