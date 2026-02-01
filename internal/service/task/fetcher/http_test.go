package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/stretchr/testify/assert"
)

// TestHTTPFetcher_Methods_Table consolidates generic HTTPFetcher method tests (Do, Get, User-Agent behavior)
func TestHTTPFetcher_Methods_Table(t *testing.T) {
	// Setup a test server that validates User-Agent and default headers
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent := r.Header.Get("User-Agent")
		accept := r.Header.Get("Accept")
		acceptLang := r.Header.Get("Accept-Language")

		if userAgent == "" || !strings.Contains(userAgent, "Mozilla/5.0") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if accept == "" || acceptLang == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	testFetcher := fetcher.NewHTTPFetcher()

	tests := []struct {
		name        string
		action      func() (*http.Response, error)
		expectError bool
	}{
		{
			name: "Do Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				req, _ := http.NewRequest("GET", ts.URL, nil)
				return testFetcher.Do(req)
			},
			expectError: false,
		},
		{
			name: "Get Request (Automatic User-Agent)",
			action: func() (*http.Response, error) {
				return fetcher.Get(context.Background(), testFetcher, ts.URL)
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := tt.action()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if resp != nil {
					defer resp.Body.Close()
					assert.Equal(t, http.StatusOK, resp.StatusCode)
				}
			}
		})
	}
}

// TestCheckResponseStatus_ErrorMessage verifies URL and Status in error message.
func TestCheckResponseStatus_ErrorMessage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	// URL.Redacted()는 쿼리 파라미터를 마스킹하지 않고UserInfo를 마스킹함
	testURL := "http://user:pass@example.com/path?query=secret"
	req, _ := http.NewRequest("GET", testURL, nil)

	// 수동으로 응답 객체 생성 (실제 서버 요청 없이 CheckResponseStatus만 테스트)
	resp := &http.Response{
		StatusCode: http.StatusServiceUnavailable,
		Status:     "503 Service Unavailable",
		Request:    req,
	}

	appErr := fetcher.CheckResponseStatus(resp)
	assert.Error(t, appErr)
	assert.Contains(t, appErr.Error(), "503 Service Unavailable")
	// redactURL은 user:password 뿐만 아니라 쿼리 파라미터 값도 마스킹함
	assert.Contains(t, appErr.Error(), "http://user:xxxxx@example.com/path?query=xxxxx")
	assert.NotContains(t, appErr.Error(), "pass", "Sensitive Info (password) should be redacted")
	assert.NotContains(t, appErr.Error(), "secret", "Query parameter values should be redacted")
}

// TestHTTPFetcher_Do_CloneRequest verifies that the original request is not mutated by Do.
func TestHTTPFetcher_Do_CloneRequest(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	f := fetcher.NewHTTPFetcher()
	req, _ := http.NewRequest("GET", ts.URL, nil)

	// 초기 헤더 상태 확인 (비어있음)
	assert.Empty(t, req.Header.Get("User-Agent"))
	assert.Empty(t, req.Header.Get("Accept"))

	resp, err := f.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// Do 실행 후에도 원본 요청의 헤더는 비어있어야 함 (clonedReq만 수정되었으므로)
	assert.Empty(t, req.Header.Get("User-Agent"), "Original request User-Agent should remain empty")
	assert.Empty(t, req.Header.Get("Accept"), "Original request Accept should remain empty")
}

func TestHTTPFetcher_NoCloneOnSameConfig(t *testing.T) {
	customTr := &http.Transport{
		MaxIdleConns: 50, // Not default (100)
	}

	// Case 1: Same config -> No clone
	f1 := fetcher.NewHTTPFetcher(
		fetcher.WithTransport(customTr),
		fetcher.WithMaxIdleConns(50),
	)
	assert.True(t, f1.GetTransport() == customTr, "Injected transport should be used directly if settings match")

	// Case 2: Different config -> Clone
	f2 := fetcher.NewHTTPFetcher(
		fetcher.WithTransport(customTr),
		fetcher.WithMaxIdleConns(60), // Different
	)
	assert.True(t, f2.GetTransport() != customTr, "Injected transport should be cloned if settings differ")
}
