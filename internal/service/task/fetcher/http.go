package fetcher

import (
	"context"
	"net/http"
	"time"
)

// HTTPFetcher 기본 타임아웃(30초) 및 User-Agent 자동 추가 기능이 내장된 HTTP 클라이언트 구현체입니다.
type HTTPFetcher struct {
	client *http.Client
}

// NewHTTPFetcher 기본 타임아웃(30초) 설정이 포함된 새로운 HTTPFetcher 인스턴스를 생성합니다.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Get 지정된 URL로 HTTP GET 요청을 전송합니다.
// User-Agent 헤더가 설정되지 않은 경우, 크롬 브라우저 값으로 자동 설정됩니다.
func (h *HTTPFetcher) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return h.Do(req)
}

// Do 커스텀 HTTP 요청을 실행합니다.
// 요청 헤더에 User-Agent가 없는 경우, 기본값(Chrome)을 자동으로 추가하여 봇 차단을 방지합니다.
func (h *HTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	// User-Agent가 설정되지 않은 경우 기본값(Chrome) 설정
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}
	return h.client.Do(req)
}
