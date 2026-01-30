package fetcher

import (
	"net/http"
)

// StatusCodeFetcher HTTP 응답 상태 코드를 확인하고, 허용된 코드가 아니면 에러로 처리하는 미들웨어입니다.
type StatusCodeFetcher struct {
	delegate        Fetcher
	allowedStatuses []int
}

// NewStatusCodeFetcher 새로운 StatusCodeFetcher 인스턴스를 생성합니다.
// 기본적으로 200 OK만 허용합니다.
func NewStatusCodeFetcher(delegate Fetcher) *StatusCodeFetcher {
	return &StatusCodeFetcher{
		delegate: delegate,
	}
}

// NewStatusCodeFetcherWithOptions 허용할 상태 코드를 지정하여 StatusCodeFetcher 인스턴스를 생성합니다.
func NewStatusCodeFetcherWithOptions(delegate Fetcher, allowedStatuses ...int) *StatusCodeFetcher {
	return &StatusCodeFetcher{
		delegate:        delegate,
		allowedStatuses: allowedStatuses,
	}
}

// Do HTTP 요청을 수행하고 응답 상태 코드를 검사합니다.
func (f *StatusCodeFetcher) Do(req *http.Request) (*http.Response, error) {
	resp, err := f.delegate.Do(req)
	if err != nil {
		return resp, err
	}

	// 상태 코드 검사: 에러 발생 시 커넥션 누수 방지를 위해 바디를 닫고 nil Response를 반환합니다.
	// (에러 객체에 이미 BodySnippet 등이 포함되어 있습니다)
	if statusErr := CheckResponseStatusNoReconstruct(resp, f.allowedStatuses...); statusErr != nil {
		drainAndCloseBody(resp.Body)
		return nil, statusErr
	}

	return resp, nil
}
