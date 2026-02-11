package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html/charset"
)

func TestScraper_validateResponse(t *testing.T) {
	l := logrus.New()
	l.SetOutput(io.Discard)
	logger := logrus.NewEntry(l)
	s := &scraper{}

	tests := []struct {
		name          string
		statusCode    int
		body          string
		contentType   string
		validator     func(*http.Response, *applog.Entry) error
		expectedError string
	}{
		{
			name:       "Success_200_OK",
			statusCode: http.StatusOK,
			body:       "success",
		},
		{
			name:       "Success_204_NoContent",
			statusCode: http.StatusNoContent,
			body:       "",
		},
		{
			name:          "Error_404_NotFound",
			statusCode:    http.StatusNotFound,
			body:          "not found error",
			contentType:   "text/plain",
			expectedError: "HTTP 요청 실패",
		},
		{
			name:          "Error_500_InternalServerError",
			statusCode:    http.StatusInternalServerError,
			body:          "server error",
			contentType:   "text/plain",
			expectedError: "HTTP 요청 실패",
		},
		{
			name:       "CustomValidator_Success",
			statusCode: http.StatusOK,
			body:       `{"status": "ok"}`,
			validator: func(resp *http.Response, logger *applog.Entry) error {
				return nil
			},
		},
		{
			name:       "CustomValidator_Failure",
			statusCode: http.StatusOK,
			body:       `{"status": "error"}`,
			validator: func(resp *http.Response, logger *applog.Entry) error {
				return errors.New("custom validation error")
			},
			expectedError: "응답 검증 실패",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Header:     make(http.Header),
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}

			params := requestParams{
				URL:       "http://example.com",
				Validator: tt.validator,
			}

			err := s.validateResponse(resp, params, logger)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				if tt.statusCode != http.StatusOK && tt.statusCode != http.StatusNoContent {
					// Check if body snippet is included in the error message
					// Note: validationFailed error format might differ from HTTPRequestFailed
					// The implementation of validateResponse reads up to 1024 bytes for error body
					if len(tt.body) > 0 {
						assert.Contains(t, err.Error(), tt.body)
					}
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScraper_validateResponse_Callback_Safety(t *testing.T) {
	l := logrus.New()
	l.SetOutput(io.Discard)
	logger := logrus.NewEntry(l)

	// 원본 요청 객체 생성
	originalReq, _ := http.NewRequest("GET", "http://example.com", nil)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("success")),
		Header:     make(http.Header),
		Request:    originalReq, // 응답에 원본 요청 연결
	}

	var capturedRequest *http.Request
	callback := func(r *http.Response) {
		capturedRequest = r.Request
	}

	s := &scraper{
		responseCallback: callback,
	}

	params := requestParams{
		URL: "http://example.com",
	}

	// 실행
	err := s.validateResponse(resp, params, logger)
	require.NoError(t, err)

	// 검증: 콜백에 전달된 Response의 Request는 nil이어야 함
	assert.Nil(t, capturedRequest, "Callback received a non-nil Request object")

	// 원본 Response의 Request는 그대로 유지되어야 함 (얕은 복사로 인한 사이드 이펙트 방지 확인)
	assert.NotNil(t, resp.Request, "Original Response.Request should not be modified")
	assert.Equal(t, originalReq, resp.Request)
}

func TestScraper_readErrorResponseBody(t *testing.T) {
	s := &scraper{}

	tests := []struct {
		name        string
		body        string
		contentType string
		expected    string
	}{
		{
			name:     "ShortBody",
			body:     "short error message",
			expected: "short error message",
		},
		{
			name:     "LongBody_Truncated",
			body:     strings.Repeat("a", 1025),
			expected: strings.Repeat("a", 1024),
		},
		{
			name:        "EUC-KR_Converted_To_UTF8",
			body:        string([]byte{0xB0, 0xA1}), // "가" in EUC-KR
			contentType: "text/plain; charset=euc-kr",
			expected:    "가",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var bodyReader io.Reader
			if strings.Contains(tt.contentType, "euc-kr") {
				// Create EUC-KR reader
				eucKrReader, _ := charset.NewReaderLabel("euc-kr", strings.NewReader(tt.expected))
				bodyReader = eucKrReader

				// Wait, if I write UTF-8 "가" to EUC-KR reader... no.
				// I need to provide EUC-KR bytes.
				// 0xB0, 0xA1 is "가" in EUC-KR.
				bodyReader = bytes.NewReader([]byte{0xB0, 0xA1})
			} else {
				bodyReader = strings.NewReader(tt.body)
			}

			resp := &http.Response{
				Body:   io.NopCloser(bodyReader),
				Header: make(http.Header),
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}

			got, err := s.readErrorResponseBody(resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestScraper_readResponseBodyWithLimit(t *testing.T) {
	s := &scraper{maxResponseBodySize: 10} // Small limit for testing

	tests := []struct {
		name              string
		body              string
		statusCode        int
		expectedBody      string
		expectedTruncated bool
	}{
		{
			name:              "UnderLimit",
			body:              "12345",
			statusCode:        http.StatusOK,
			expectedBody:      "12345",
			expectedTruncated: false,
		},
		{
			name:              "OverLimit",
			body:              "12345678901", // 11 bytes
			statusCode:        http.StatusOK,
			expectedBody:      "1234567890", // 10 bytes
			expectedTruncated: true,
		},
		{
			name:              "ExactLimit",
			body:              "1234567890", // 10 bytes
			statusCode:        http.StatusOK,
			expectedBody:      "1234567890",
			expectedTruncated: false,
		},
		{
			name:              "NoContent_204",
			body:              "",
			statusCode:        http.StatusNoContent,
			expectedBody:      "",
			expectedTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}

			gotBody, gotTruncated, err := s.readResponseBodyWithLimit(context.Background(), resp)
			require.NoError(t, err)

			if tt.statusCode == http.StatusNoContent {
				assert.Nil(t, gotBody)
			} else {
				assert.Equal(t, tt.expectedBody, string(gotBody))
			}
			assert.Equal(t, tt.expectedTruncated, gotTruncated)
		})
	}
}

func TestScraper_previewBody(t *testing.T) {
	s := &scraper{}

	tests := []struct {
		name        string
		body        []byte
		contentType string
		expected    string
	}{
		{
			name:     "ShortText",
			body:     []byte("hello world"),
			expected: "hello world",
		},
		{
			name:     "LongText_Truncated",
			body:     bytes.Repeat([]byte("a"), 1025),
			expected: strings.Repeat("a", 1024) + "...(생략됨)",
		},
		{
			name:     "BinaryData",
			body:     []byte{0x00, 0x01, 0x02, 0x03},
			expected: "[바이너리 데이터] (4 바이트)",
		},
		{
			name:     "AllowedControlChars",
			body:     []byte("line1\nline2\tcad\r"),
			expected: "line1\nline2\tcad\r",
		},
		{
			name:        "EUC-KR_Conversion",
			body:        []byte{0xB0, 0xA1}, // "가" in EUC-KR
			contentType: "text/plain; charset=euc-kr",
			expected:    "가",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.previewBody(tt.body, tt.contentType)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestIsUTF8ContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{"Explicit UTF-8", "text/html; charset=utf-8", true},
		{"Explicit UTF-8 Uppercase", "application/json; charset=UTF-8", true},
		{"EUC-KR", "text/html; charset=euc-kr", false},
		{"No Charset", "text/plain", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isUTF8ContentType(tt.contentType))
		})
	}
}

func TestIsHTMLContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{"HTML", "text/html", true},
		{"HTML with Charset", "text/html; charset=utf-8", true},
		{"XHTML", "application/xhtml+xml", true},
		{"JSON", "application/json", false},
		{"Plain Text", "text/plain", false},
		{"Empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isHTMLContentType(tt.contentType))
		})
	}
}

func TestScraper_readResponseBodyWithLimit_ContextCanceled(t *testing.T) {
	s := &scraper{maxResponseBodySize: 1024}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 즉시 취소

	// 무한히 읽히는 Reader (Context 취소 테스트용)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&infiniteReader{}),
	}

	_, _, err := s.readResponseBodyWithLimit(ctx, resp)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "Expected context.Canceled error, got: %v", err)
}

type infiniteReader struct{}

func (r *infiniteReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'A'
	}
	return len(p), nil
}
