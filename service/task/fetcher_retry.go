package task

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// RetryFetcher Fetcher 인터페이스를 구현하며, HTTP 요청 실패 시 자동으로 재시도를 수행합니다.
// 지수 백오프(Exponential Backoff)와 Jitter를 사용하여 재시도 간격을 조절합니다.
type RetryFetcher struct {
	delegate   Fetcher
	maxRetries int
	retryDelay time.Duration
}

// NewRetryFetcher 새로운 RetryFetcher 인스턴스를 생성합니다.
// maxRetries는 0~10 범위로 제한되며, retryDelay는 최소 1초로 설정됩니다.
func NewRetryFetcher(delegate Fetcher, maxRetries int, retryDelay time.Duration) *RetryFetcher {
	// 재시도 횟수 검증 (음수 방지 및 최대값 제한)
	if maxRetries < 0 {
		maxRetries = 0
	}
	if maxRetries > 10 {
		maxRetries = 10 // 과도한 재시도 방지
	}

	// 재시도 지연 시간 검증
	if retryDelay < time.Second {
		retryDelay = time.Second // 최소 1초
	}

	return &RetryFetcher{
		delegate:   delegate,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
	}
}

// Get 지정된 URL로 GET 요청을 전송합니다. 내부적으로 Do 메서드를 호출하여 재시도 로직을 수행합니다.
func (f *RetryFetcher) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrTaskExecutionFailed, "failed to create request")
	}
	return f.Do(req)
}

// Do HTTP 요청을 수행하며, 실패하거나 500 이상의 에러 발생 시 재시도합니다.
func (f *RetryFetcher) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= f.maxRetries; i++ {
		// 첫 번째 시도가 아닌 경우, 백오프(Backoff) 시간만큼 대기 후 재시도
		if i > 0 {
			// 지수 백오프 적용: 재시도 횟수가 늘어날수록 대기 시간도 증가 (2^(i-1))
			// 예: retryDelay가 1초라면, 1초 -> 2초 -> 4초 -> 8초 ...
			delay := f.retryDelay * time.Duration(1<<(i-1))

			// Jitter(무작위성) 추가: +/- 10% 범위 내에서 랜덤하게 대기 시간을 조정하여
			// 여러 클라이언트가 동시에 재시도하는 'Thundering Herd' 문제 방지
			jitter := time.Duration(rand.Int63n(int64(delay/10 + 1)))
			if rand.Intn(2) == 0 {
				delay += jitter
			} else {
				delay -= jitter
			}

			if delay < 0 {
				delay = f.retryDelay
			}

			applog.WithComponentAndFields("task.fetcher", log.Fields{
				"url":         req.URL.String(),
				"retry":       i,
				"max_retries": f.maxRetries,
				"delay":       delay.String(),
			}).Warn("HTTP 요청 실패, 재시도 대기 중")

			// Context가 취소되었는지 확인하며 대기
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err() // 요청 컨텍스트가 종료되면 재시도 중단
			case <-time.After(delay):
				// 대기 시간 종료 후 루프 계속 진행
			}
		}

		// 재시도 시, 요청 본문(Body)이 이미 읽혔을 수 있으므로 GetBody를 통해 복구 시도
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err == nil {
				req.Body = body
			}
		}

		resp, err := f.delegate.Do(req)
		// 에러가 없고, 서버 에러(500번대)가 아니며, 429(Too Many Requests)가 아니면 성공으로 간주하고 반환
		if err == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			// 실패했지만 응답이 있는 경우 Body를 닫아야 함 (특히 500번대 에러)
			resp.Body.Close()
			if err == nil {
				// 에러 객체가 없지만 상태 코드가 500 이상인 경우 에러 생성
				lastErr = apperrors.New(apperrors.ErrTaskExecutionFailed, fmt.Sprintf("HTTP status %s", resp.Status))
			}
		}
	}
	return nil, apperrors.Wrap(lastErr, apperrors.ErrTaskExecutionFailed, "max retries exceeded")
}
