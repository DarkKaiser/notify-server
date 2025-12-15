package task

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// RetryFetcher Fetcher 인터페이스를 구현하며, HTTP 요청 실패 시 자동으로 재시도를 수행합니다.
// 단순한 재시도가 아닌, Exponential Backoff(지수 백오프)와 Jitter(무작위 지연) 전략을 결합하여
// 서버 부하를 최소화하면서도 성공 확률을 높이는 안정적인 재시도 메커니즘을 제공합니다.
type RetryFetcher struct {
	delegate   Fetcher
	maxRetries int
	retryDelay time.Duration
	maxDelay   time.Duration
}

// NewRetryFetcherFromConfig 설정값(재시도 횟수, 지연 시간 문자열)을 기반으로 RetryFetcher 인스턴스를 생성합니다.
//
// Parameters:
//   - maxRetries: 최대 재시도 횟수 (0-10 권장)
//   - retryDelayStr: 재시도 대기 시간 문자열 (최소 1초)
func NewRetryFetcherFromConfig(maxRetries int, retryDelayStr string) *RetryFetcher {
	retryDelay, err := time.ParseDuration(retryDelayStr)
	if err != nil {
		retryDelay, _ = time.ParseDuration(config.DefaultRetryDelay)
	}
	return NewRetryFetcher(NewHTTPFetcher(), maxRetries, retryDelay, 30*time.Second)
}

// NewRetryFetcher 새로운 RetryFetcher 인스턴스를 생성합니다.
// 안정적인 동작을 위해 maxRetries는 0~10 범위로 자동 보정되며, retryDelay는 최소 1초 이상으로 설정됩니다.
//
// Parameters:
//   - delegate: 실제 HTTP 요청을 수행할 원본 Fetcher
//   - maxRetries: 최대 재시도 횟수 (0-10 권장)
//   - retryDelay: 재시도 대기 시간 (최소 1초)
//   - maxDelay: 최대 대기 시간 (지수 백오프 증가 시 이 값을 넘지 않음)
func NewRetryFetcher(delegate Fetcher, maxRetries int, retryDelay time.Duration, maxDelay time.Duration) *RetryFetcher {
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

	// 최대 대기 시간은 초기 대기 시간보다 작을 수 없음
	if maxDelay < retryDelay {
		maxDelay = retryDelay
	}

	return &RetryFetcher{
		delegate:   delegate,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		maxDelay:   maxDelay,
	}
}

// Get 지정된 URL로 HTTP GET 요청을 전송합니다.
// 내부적으로 Do 메서드를 호출하여 재시도 정책이 적용된 안전한 요청을 수행합니다.
func (f *RetryFetcher) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, apperrors.Wrap(err, ErrTaskExecutionFailed, "failed to create request")
	}
	return f.Do(req)
}

// Do HTTP 요청을 수행하며, 실패 시 설정된 정책에 따라 재시도합니다.
//
// [재시도 전략 상세]
// 1. Exponential Backoff: 재시도 횟수가 증가할수록 대기 시간이 2배씩 증가합니다 (1초, 2초, 4초, ...).
// 2. Max Delay Cap: 대기 시간은 설정된 최대 시간(maxDelay)을 초과하지 않습니다.
// 3. Full Jitter: 대기 시간 범위 내에서 무작위 값을 더하거나 빼서(+/- 10%), 여러 클라이언트가 동시에 재시도하는 'Thundering Herd' 문제를 방지합니다.
// 4. Retry Conditions: 네트워크 오류, 5xx 서버 에러, 429(Too Many Requests) 응답 시 재시도합니다.
func (f *RetryFetcher) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for i := 0; i <= f.maxRetries; i++ {
		// 첫 번째 시도가 아닌 경우, 백오프(Backoff) 시간만큼 대기 후 재시도
		if i > 0 {
			// 지수 백오프 적용: 재시도 횟수가 늘어날수록 대기 시간도 증가 (2^(i-1))
			// 예: retryDelay가 1초라면, 1초 -> 2초 -> 4초 -> 8초 ...
			delay := f.retryDelay * time.Duration(1<<(i-1))

			// Max Delay Cap 적용
			if delay > f.maxDelay {
				delay = f.maxDelay
			}

			// Jitter(무작위성) 추가: +/- 10% 범위 내에서 랜덤하게 대기 시간을 조정하여
			// 여러 클라이언트가 동시에 재시도하는 'Thundering Herd' 문제 방지
			// math/rand/v2 사용 및 안전한 범위 계산
			jitterRange := int64(delay / 10)
			if jitterRange > 0 {
				jitter := time.Duration(rand.Int64N(jitterRange + 1))
				if rand.IntN(2) == 0 {
					delay += jitter
				} else {
					delay -= jitter
				}
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

			// Context가 취소되었는지 확인하며 대기 (Graceful Shutdown 지원)
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

		// 재시도 여부 판단 로직 호출
		if !f.shouldRetry(resp, err) {
			return resp, nil
		}

		lastErr = err
		if resp != nil {
			// 실패했지만 응답이 있는 경우 Body를 닫아야 함 (특히 500번대 에러)
			resp.Body.Close()
			if err == nil {
				// 에러 객체가 없지만 상태 코드가 500 이상인 경우 에러 생성
				lastErr = apperrors.New(ErrTaskExecutionFailed, fmt.Sprintf("HTTP status %s", resp.Status))
			}
		}
	}
	return nil, apperrors.Wrap(lastErr, ErrTaskExecutionFailed, "max retries exceeded")
}

// shouldRetry 응답 상태와 에러를 분석하여 재시도 수행 여부를 결정합니다.
//
// [재시도 대상]
// - 모든 네트워크 레벨 에러 (DNS, Connection Refused 등)
// - 5xx (Internal Server Error 등 서버 측 문제)
// - 429 (Too Many Requests - 속도 제한)
//
// [재시도 제외]
// - 2xx (성공)
// - 4xx (Client Error - 잘못된 요청, 권한 없음 등. 단, 429는 예외)
func (f *RetryFetcher) shouldRetry(resp *http.Response, err error) bool {
	// 에러가 있는 경우 (네트워크 오류 등) 재시도
	if err != nil {
		return true
	}

	// 응답이 없는 경우는 (이론적으로 드물지만) 재시도
	if resp == nil {
		return true
	}

	// 500번대 서버 에러는 재시도 (일시적인 장애일 가능성 높음)
	if resp.StatusCode >= 500 {
		return true
	}

	// 429 Too Many Requests는 재시도 (잠시 후 요청하면 성공할 가능성 있음)
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// 그 외 (2xx, 4xx 등)는 성공 또는 클라이언트 에러이므로 재시도하지 않음
	return false
}
