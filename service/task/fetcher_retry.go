package task

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

// RetryFetcher는 Fetcher 인터페이스를 구현하며, 실패 시 재시도를 수행합니다.
type RetryFetcher struct {
	delegate   Fetcher
	maxRetries int
	retryDelay time.Duration
}

// NewRetryFetcher는 새로운 RetryFetcher 인스턴스를 생성합니다.
func NewRetryFetcher(delegate Fetcher, maxRetries int, retryDelay time.Duration) *RetryFetcher {
	return &RetryFetcher{
		delegate:   delegate,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

func (f *RetryFetcher) Get(url string) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= f.maxRetries; i++ {
		if i > 0 {
			log.Warnf("HTTP 요청 실패, 재시도 중... (%d/%d) URL: %s", i, f.maxRetries, url)
			time.Sleep(f.retryDelay)
		}

		resp, err := f.delegate.Get(url)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			// 500번대 에러인 경우 body를 닫고 재시도
			resp.Body.Close()
			if err == nil {
				lastErr = fmt.Errorf("HTTP status %s", resp.Status)
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

func (f *RetryFetcher) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= f.maxRetries; i++ {
		if i > 0 {
			log.Warnf("HTTP 요청 실패, 재시도 중... (%d/%d) URL: %s", i, f.maxRetries, req.URL.String())
			time.Sleep(f.retryDelay)
		}

		// Request Body가 있는 경우, 읽은 후에는 다시 읽을 수 없으므로 처리가 필요할 수 있음.
		// 하지만 여기서는 단순 GET 요청 위주라고 가정하고 진행.
		// 필요하다면 GetBody를 사용하여 Body를 복구해야 함.
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err == nil {
				req.Body = body
			}
		}

		resp, err := f.delegate.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			resp.Body.Close()
			if err == nil {
				lastErr = fmt.Errorf("HTTP status %s", resp.Status)
			}
		}
	}
	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}
