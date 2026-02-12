package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestScraper_executeRequest_Comprehensive는 executeRequest의 전체 라이프사이클과 에러 처리를 검증합니다.
func TestScraper_executeRequest_Comprehensive(t *testing.T) {
	// Logrus Hook을 사용하여 로그 메시지를 검증합니다.
	hook := test.NewGlobal()

	type mockConfig struct {
		response *http.Response
		err      error
	}

	tests := []struct {
		name string
		// Input
		params     requestParams
		scraperOpt []Option

		// Mock
		mockCfg mockConfig

		// Context
		ctxSetup func() (context.Context, context.CancelFunc)

		// Verification
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
		checkResult func(*testing.T, fetchResult)
		checkLog    func(*testing.T, *test.Hook)
	}{
		// -------------------------------------------------------------------------
		// [Category 1: Success Scenarios]
		// -------------------------------------------------------------------------
		{
			name: "Success - Simple Request",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockCfg: mockConfig{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString("success body")),
					Header:     http.Header{"Content-Type": []string{"text/plain"}},
				},
			},
			checkResult: func(t *testing.T, res fetchResult) {
				assert.Equal(t, 200, res.Response.StatusCode)
				assert.Equal(t, "success body", string(res.Body))
				assert.False(t, res.IsTruncated)
			},
		},
		{
			name: "Success - Truncated Body",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com/large",
			},
			scraperOpt: []Option{
				WithMaxResponseBodySize(5),
			},
			mockCfg: mockConfig{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString("1234567890")),
					Header:     make(http.Header),
				},
			},
			checkResult: func(t *testing.T, res fetchResult) {
				assert.Equal(t, 200, res.Response.StatusCode)
				assert.Equal(t, 5, len(res.Body))
				assert.Equal(t, "12345", string(res.Body))
				assert.True(t, res.IsTruncated)
			},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "응답 본문 크기 제한 초과") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected warning log for truncated body")
			},
		},

		// -------------------------------------------------------------------------
		// [Category 2: Network & Request Failure]
		// -------------------------------------------------------------------------
		{
			name: "Error - Request Creation Failed (Invalid URL)",
			params: requestParams{
				Method: "GET",
				URL:    "://invalid-url",
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // Request creation fails typically due to bad URL parsing which is client side error
			errContains: []string{"HTTP 요청 생성 실패"},
		},
		{
			name: "Error - Network Failure (Connection Refused)",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockCfg: mockConfig{
				err: errors.New("connection refused"),
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"네트워크 오류"},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "HTTP 요청 전송 실패") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error log for network failure")
			},
		},

		// -------------------------------------------------------------------------
		// [Category 3: Validation Failure]
		// -------------------------------------------------------------------------
		{
			name: "Error - Validation Failure (404 Not Found)",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com/notfound",
			},
			mockCfg: mockConfig{
				response: &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(bytes.NewBufferString("not found")),
					Header:     make(http.Header),
				},
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // 404 is ExecutionFailed (Permanent Error)
			errContains: []string{"HTTP 요청 실패", "404"},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "HTTP 응답 유효성 검증 실패") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error log for validation failure")
			},
		},
		{
			name: "Error - Custom Validator Failure",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com/api",
				Validator: func(resp *http.Response, logger *applog.Entry) error {
					return errors.New("custom validation error")
				},
			},
			mockCfg: mockConfig{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error": "bad data"}`)),
					Header:     http.Header{"Content-Type": []string{"application/json"}},
				},
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed, // Custom validator returns standard error -> ExecutionFailed
			errContains: []string{"응답 검증 실패", "custom validation error"},
		},

		// -------------------------------------------------------------------------
		// [Category 4: Body Read Failure]
		// -------------------------------------------------------------------------
		{
			name: "Error - Body Read Failure",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com/api",
			},
			mockCfg: mockConfig{
				response: &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(&failReader{}), // Fails on Read
					Header:     make(http.Header),
				},
			},
			wantErr:     true,
			errType:     apperrors.Unavailable, // Read failure is considered transient/unavailable
			errContains: []string{"응답 본문 데이터 수신 실패"},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "응답 본문 읽기 실패") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error log for body read failure")
			},
		},

		// -------------------------------------------------------------------------
		// [Category 5: Context Cancellation]
		// -------------------------------------------------------------------------
		{
			name: "Error - Context Canceled Before Request",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			ctxSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			mockCfg: mockConfig{
				// Do should return error
				err: context.Canceled,
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"요청 중단"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()

			// 1. Setup Mock
			mockFetcher := new(mocks.MockFetcher)

			// Only set expectation if URL is valid, otherwise createAndSendRequest fails before fetching
			if !strings.Contains(tt.name, "Invalid URL") {
				if tt.mockCfg.response != nil {
					mockFetcher.On("Do", mock.Anything).Return(tt.mockCfg.response, nil)
				} else {
					mockFetcher.On("Do", mock.Anything).Return(nil, tt.mockCfg.err)
				}
			}

			// 2. Initialize Scraper (Direct Struct Instantiation for Internal Method Testing)
			s := &scraper{
				fetcher:             mockFetcher,
				maxRequestBodySize:  defaultMaxBodySize,
				maxResponseBodySize: defaultMaxBodySize,
			}
			// Apply options manually
			for _, opt := range tt.scraperOpt {
				opt(s)
			}

			// 3. Setup Context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxSetup != nil {
				ctx, cancel = tt.ctxSetup()
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			// 4. Execute
			result, _, err := s.executeRequest(ctx, tt.params)

			// 5. Verify Error
			if tt.wantErr {
				require.Error(t, err)
				if len(tt.errContains) > 0 {
					for _, msg := range tt.errContains {
						assert.Contains(t, err.Error(), msg)
					}
				}
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, result)
				}
			}

			// 6. Verify Logs
			if tt.checkLog != nil {
				tt.checkLog(t, hook)
			}
		})
	}
}

// failReader is a helper for testing read errors
type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read failed")
}
func (f *failReader) Close() error { return nil }
