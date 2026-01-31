package fetcher

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestCheckResponseStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantType   apperrors.ErrorType
		wantStatus string
	}{
		{"200 OK", http.StatusOK, apperrors.Unknown, "OK"},
		{"400 Bad Request", http.StatusBadRequest, apperrors.InvalidInput, "Bad Request"},
		{"401 Unauthorized", http.StatusUnauthorized, apperrors.Forbidden, "Unauthorized"},
		{"403 Forbidden", http.StatusForbidden, apperrors.Forbidden, "Forbidden"},
		{"404 Not Found", http.StatusNotFound, apperrors.NotFound, "Not Found"},
		{"408 Request Timeout", http.StatusRequestTimeout, apperrors.Unavailable, "Request Timeout"},
		{"429 Too Many Requests", http.StatusTooManyRequests, apperrors.Unavailable, "Too Many Requests"},
		{"500 Internal Server Error", http.StatusInternalServerError, apperrors.Unavailable, "Internal Server Error"},
		{"503 Service Unavailable", http.StatusServiceUnavailable, apperrors.Unavailable, "Service Unavailable"},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, apperrors.Unavailable, "Gateway Timeout"},
		{"Other 4xx", http.StatusMethodNotAllowed, apperrors.ExecutionFailed, "Method Not Allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Status:     http.StatusText(tt.statusCode),
				Request: &http.Request{
					URL: &url.URL{Scheme: "https", Host: "example.com", Path: "/"},
				},
				Body: io.NopCloser(bytes.NewBufferString("")),
			}
			err := CheckResponseStatus(resp)

			if tt.statusCode >= 200 && tt.statusCode < 300 {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.True(t, apperrors.Is(err, tt.wantType), "expected error type %v, got %v", tt.wantType, err)

				var statusErr *HTTPStatusError
				if assert.ErrorAs(t, err, &statusErr) {
					assert.Equal(t, tt.statusCode, statusErr.StatusCode)
					assert.Equal(t, tt.wantStatus, statusErr.Status)
				}
			}
		})
	}
}

func TestCheckResponseStatus_WithAllowedStatuses(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Status:     "404 Not Found",
		Body:       io.NopCloser(bytes.NewBufferString("")),
	}

	// 1. Without allowedStatuses (default) -> should fail on 404
	err := CheckResponseStatus(resp)
	assert.Error(t, err)
	assert.True(t, apperrors.Is(err, apperrors.NotFound))

	// 2. With 404 in allowedStatuses -> should succeed
	err = CheckResponseStatus(resp, http.StatusNotFound)
	assert.NoError(t, err)

	// 3. With other allowedStatuses (not 404) -> should fail
	err = CheckResponseStatus(resp, http.StatusBadRequest, http.StatusForbidden)
	assert.Error(t, err)
}

func TestCheckResponseStatus_BodySnippetLimit(t *testing.T) {
	// 4KB보다 큰 긴 본문 생성 (4096 + 10 바이트)
	longBody := strings.Repeat("A", 4106)
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Internal Server Error",
		Body:       io.NopCloser(bytes.NewBufferString(longBody)),
		Request:    &http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}},
	}

	err := CheckResponseStatus(resp)
	assert.Error(t, err)

	var statusErr *HTTPStatusError
	assert.ErrorAs(t, err, &statusErr)

	// 본문 스니펫은 최대 4096 바이트까지만 캡처되어야 함
	assert.Len(t, statusErr.BodySnippet, 4096)
	assert.Equal(t, strings.Repeat("A", 4096), statusErr.BodySnippet)
}

func TestCheckResponseStatus_BodyReconstruction(t *testing.T) {
	originalBody := "response body content"
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Error",
		Body:       io.NopCloser(bytes.NewBufferString(originalBody)),
		Request:    &http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}},
	}

	// 1. CheckResponseStatus (Reconstruct = true)
	err := CheckResponseStatus(resp)
	assert.Error(t, err)

	// 에러 발생 후에도 본문을 다시 읽을 수 있어야 함
	readBody, readErr := io.ReadAll(resp.Body)
	assert.NoError(t, readErr)
	assert.Equal(t, originalBody, string(readBody))
}

func TestCheckResponseStatusWithoutReconstruct(t *testing.T) {
	originalBody := "response body content"
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Status:     "500 Error",
		Body:       io.NopCloser(bytes.NewBufferString(originalBody)),
		Request:    &http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}},
	}

	// 1. CheckResponseStatusWithoutReconstruct (Reconstruct = false)
	err := CheckResponseStatusWithoutReconstruct(resp)
	assert.Error(t, err)

	var statusErr *HTTPStatusError
	assert.ErrorAs(t, err, &statusErr)
	assert.Equal(t, originalBody, statusErr.BodySnippet) // 스니펫은 캡처됨

	// 재구성을 안 했으므로, 본문은 이미 읽혀진 상태여야 함 (다음 읽기 시 EOF 또는 빈 내용)
	// (LimitReader로 일부만 읽었으므로 나머지가 읽힐 수도 있지만, 중요한 건 처음부터 다시 읽을 수 없다는 점)
	// 여기서는 전체를 읽어버렸는지 LimitReader 특성에 따라 다를 수 있으나,
	// 일반적인 기대는 '재구성되지 않았다'는 것.
	// io.ReadAll을 다시 호출하면, 이미 읽은 부분(snippet)은 다시 나오지 않아야 함.
	remainingBody, _ := io.ReadAll(resp.Body)
	assert.Empty(t, remainingBody, "For short body, it should be fully consumed by snippet reader if not reconstructed")
}

func TestCheckResponseStatus_NilBody(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Status:     "400 Bad Request",
		Body:       nil, // Body가 nil인 경우
		Request:    &http.Request{URL: &url.URL{Scheme: "http", Host: "example.com"}},
	}

	err := CheckResponseStatus(resp)
	assert.Error(t, err)

	var statusErr *HTTPStatusError
	assert.ErrorAs(t, err, &statusErr)
	assert.Empty(t, statusErr.BodySnippet)
}

func TestCheckResponseStatus_Redaction(t *testing.T) {
	// 민감 정보가 포함된 URL 및 헤더
	rawURL := "https://example.com/api?token=secret123&key=value"
	reqURL, _ := url.Parse(rawURL)

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Authorization", "Bearer secret-token")
	header.Set("Cookie", "session=secret-session")

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Status:     "401 Unauthorized",
		Header:     header,
		Body:       io.NopCloser(bytes.NewBufferString("error")),
		Request:    &http.Request{URL: reqURL},
	}

	err := CheckResponseStatus(resp)
	assert.Error(t, err)

	var statusErr *HTTPStatusError
	assert.ErrorAs(t, err, &statusErr)

	// URL 마스킹 확인
	assert.NotContains(t, statusErr.URL, "secret123")
	assert.Contains(t, statusErr.URL, "token=xxxxx") // RedactURL 구현에 따라 다름 (xxxxx 또는 ***)

	// 헤더 마스킹 확인
	assert.Equal(t, "***", statusErr.Header.Get("Authorization"))
	assert.Equal(t, "***", statusErr.Header.Get("Cookie"))
	assert.Equal(t, "application/json", statusErr.Header.Get("Content-Type"))
}
