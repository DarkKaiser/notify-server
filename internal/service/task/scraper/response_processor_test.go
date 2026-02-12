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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/transform"
)

// TestValidateResponse_StatusCodes HTTP 상태 코드에 따른 검증 로직을 테스트합니다.
func TestValidateResponse_StatusCodes(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
	}{
		{
			name:       "Success - 200 OK",
			statusCode: http.StatusOK,
		},
		{
			name:       "Success - 201 Created",
			statusCode: http.StatusCreated,
		},
		{
			name:       "Success - 204 No Content",
			statusCode: http.StatusNoContent,
		},
		{
			name:        "Error - 400 Bad Request",
			statusCode:  http.StatusBadRequest,
			body:        "Bad Request From Server",
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: []string{"HTTP 요청 실패", "400", "Bad Request From Server"},
		},
		{
			name:        "Error - 404 Not Found",
			statusCode:  http.StatusNotFound,
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: []string{"HTTP 요청 실패", "404"},
		},
		{
			name:        "Error - 429 Too Many Requests (Retryable)",
			statusCode:  http.StatusTooManyRequests,
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"HTTP 요청 실패", "429"},
		},
		{
			name:        "Error - 500 Internal Server Error (Retryable)",
			statusCode:  http.StatusInternalServerError,
			body:        "Server Panic",
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"HTTP 요청 실패", "500", "Server Panic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			mockFetcher := new(mocks.MockFetcher)
			s := New(mockFetcher).(*scraper)
			logger := applog.WithContext(context.Background())

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Request:    &http.Request{URL: nil},
			}

			// Act
			err := s.validateResponse(resp, requestParams{}, logger)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s", tt.errType)
				}
				for _, msg := range tt.errContains {
					assert.Contains(t, err.Error(), msg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateResponse_CustomValidator 사용자 정의 Validator 로직을 테스트합니다.
func TestValidateResponse_CustomValidator(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		body        string
		validator   func(*http.Response, *applog.Entry) error
		wantErr     bool
		errContains []string
	}{
		{
			name:       "Success - Validator Passes",
			statusCode: http.StatusOK,
			validator: func(resp *http.Response, logger *applog.Entry) error {
				return nil
			},
			wantErr: false,
		},
		{
			name:       "Error - Validator Fails",
			statusCode: http.StatusOK,
			body:       "Invalid Content",
			validator: func(resp *http.Response, logger *applog.Entry) error {
				return errors.New("custom validation error")
			},
			wantErr:     true,
			errContains: []string{"응답 검증 실패", "custom validation error", "Invalid Content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			s := New(new(mocks.MockFetcher)).(*scraper)
			logger := applog.WithContext(context.Background())

			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Request:    &http.Request{URL: nil},
			}

			params := requestParams{Validator: tt.validator}

			// Act
			err := s.validateResponse(resp, params, logger)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				for _, msg := range tt.errContains {
					assert.Contains(t, err.Error(), msg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestValidateResponse_Callback ResponseCallback 호출 로직을 테스트합니다.
func TestValidateResponse_Callback(t *testing.T) {
	// Arrange
	callbackCalled := false
	s := New(new(mocks.MockFetcher), WithResponseCallback(func(resp *http.Response) {
		callbackCalled = true
		// Verify callback receives safe copy
		assert.Equal(t, http.NoBody, resp.Body, "Callback should receive NoBody")
		assert.Nil(t, resp.Request, "Callback should receive nil Request")
	})).(*scraper)
	logger := applog.WithContext(context.Background())

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("body")),
		Request:    &http.Request{URL: nil},
	}

	// Act
	err := s.validateResponse(resp, requestParams{}, logger)

	// Assert
	assert.NoError(t, err)
	assert.True(t, callbackCalled, "Response callback should be called")
}

// TestReadResponseBodyWithLimit 응답 본문 읽기 및 크기 제한 로직을 테스트합니다.
func TestReadResponseBodyWithLimit(t *testing.T) {
	tests := []struct {
		name          string
		bodyContent   string
		maxSize       int64
		wantContent   string
		wantTruncated bool
	}{
		{
			name:          "Success - Under Limit",
			bodyContent:   "12345",
			maxSize:       10,
			wantContent:   "12345",
			wantTruncated: false,
		},
		{
			name:          "Success - Exact Limit",
			bodyContent:   "1234567890",
			maxSize:       10,
			wantContent:   "1234567890",
			wantTruncated: false,
		},
		{
			name:          "Success - Over Limit (Truncated)",
			bodyContent:   "12345678901",
			maxSize:       10,
			wantContent:   "1234567890",
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			s := New(new(mocks.MockFetcher), WithMaxResponseBodySize(tt.maxSize)).(*scraper)
			resp := &http.Response{
				Body: io.NopCloser(strings.NewReader(tt.bodyContent)),
			}

			// Act
			content, truncated, err := s.readResponseBodyWithLimit(context.Background(), resp)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.wantTruncated, truncated)
			assert.Equal(t, tt.wantContent, string(content))
		})
	}
}

// TestReadResponseBodyWithLimit_ContextCancel 컨텍스트 취소 시 읽기 중단 로직을 테스트합니다.
func TestReadResponseBodyWithLimit_ContextCancel(t *testing.T) {
	// Arrange
	s := New(new(mocks.MockFetcher)).(*scraper)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("some body")),
	}

	// Act
	_, _, err := s.readResponseBodyWithLimit(ctx, resp)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// TestReadErrorResponseBody 에러 응답 본문 읽기 및 인코딩 변환을 테스트합니다.
func TestReadErrorResponseBody(t *testing.T) {
	eucKrData, _ := io.ReadAll(transform.NewReader(strings.NewReader("한글"), korean.EUCKR.NewEncoder()))

	tests := []struct {
		name        string
		body        []byte
		contentType string
		want        string
	}{
		{
			name: "UTF-8 Body",
			body: []byte("Error Message"),
			want: "Error Message",
		},
		{
			name:        "EUC-KR Body with Charset",
			body:        eucKrData,
			contentType: "text/plain; charset=euc-kr",
			want:        "한글",
		},
		{
			name: "Truncation check (Over 1KB)",
			body: append(bytes.Repeat([]byte("a"), 1024), []byte("bc")...),
			want: strings.Repeat("a", 1024),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			s := New(new(mocks.MockFetcher)).(*scraper)
			resp := &http.Response{
				Body:   io.NopCloser(bytes.NewReader(tt.body)),
				Header: http.Header{"Content-Type": []string{tt.contentType}},
			}

			// Act
			got, err := s.readErrorResponseBody(resp)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestPreviewBody 응답 본문 미리보기 생성 로직을 검증합니다.
func TestPreviewBody(t *testing.T) {
	eucKrData, _ := io.ReadAll(transform.NewReader(strings.NewReader("한글"), korean.EUCKR.NewEncoder()))

	tests := []struct {
		name        string
		body        []byte
		contentType string
		want        string
		wantPrefix  bool // Exact match or Prefix match
	}{
		{
			name: "Empty Body",
			body: []byte{},
			want: "",
		},
		{
			name: "Short UTF-8",
			body: []byte("Hello World"),
			want: "Hello World",
		},
		{
			name:        "EUC-KR Conversion",
			body:        eucKrData,
			contentType: "text/html; charset=euc-kr",
			want:        "한글",
		},
		{
			name:       "Binary Data Detection",
			body:       []byte{0x00, 0x01, 0x02, 0x03},
			want:       "[바이너리 데이터]",
			wantPrefix: true,
		},
		{
			name: "Truncation with Ellipsis",
			body: bytes.Repeat([]byte("a"), 2000),
			want: strings.Repeat("a", 1024) + "...(생략됨)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			s := New(new(mocks.MockFetcher)).(*scraper)

			// Act
			got := s.previewBody(tt.body, tt.contentType)

			// Assert
			if tt.wantPrefix {
				assert.True(t, strings.HasPrefix(got, tt.want), "Expected prefix %q, got %q", tt.want, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

// TestIsCommonContentTypes Content-Type 헬퍼 함수들을 테스트합니다.
func TestIsCommonContentTypes(t *testing.T) {
	t.Run("isUTF8ContentType", func(t *testing.T) {
		assert.True(t, isUTF8ContentType("text/html; charset=utf-8"))
		assert.True(t, isUTF8ContentType("application/json; charset=UTF-8"))
		assert.False(t, isUTF8ContentType("text/html; charset=euc-kr"))
		assert.False(t, isUTF8ContentType("image/png"))
	})

	t.Run("isHTMLContentType", func(t *testing.T) {
		assert.True(t, isHTMLContentType("text/html"))
		assert.True(t, isHTMLContentType("text/html; charset=utf-8"))
		assert.True(t, isHTMLContentType("application/xhtml+xml"))
		assert.False(t, isHTMLContentType("application/json"))
		assert.False(t, isHTMLContentType("text/plain"))
	})
}
