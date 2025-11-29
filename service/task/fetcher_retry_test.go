package task

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// RetryTestMockFetcher는 Fetcher 인터페이스의 Mock 구현체입니다.
type RetryTestMockFetcher struct {
	mock.Mock
}

func (m *RetryTestMockFetcher) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *RetryTestMockFetcher) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func TestRetryFetcher_Get_Success(t *testing.T) {
	mockFetcher := new(RetryTestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	expectedResp := &http.Response{StatusCode: 200}
	mockFetcher.On("Get", "http://example.com").Return(expectedResp, nil)

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Get", 1)
}

func TestRetryFetcher_Get_RetrySuccess(t *testing.T) {
	mockFetcher := new(RetryTestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 첫 번째, 두 번째 호출은 실패, 세 번째 호출은 성공
	mockFetcher.On("Get", "http://example.com").Return(nil, errors.New("network error")).Once()
	mockFetcher.On("Get", "http://example.com").Return(nil, errors.New("network error")).Once()
	expectedResp := &http.Response{StatusCode: 200}
	mockFetcher.On("Get", "http://example.com").Return(expectedResp, nil).Once()

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Get", 3)
}

func TestRetryFetcher_Get_MaxRetriesExceeded(t *testing.T) {
	mockFetcher := new(RetryTestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 모든 호출 실패
	mockFetcher.On("Get", "http://example.com").Return(nil, errors.New("network error"))

	resp, err := retryFetcher.Get("http://example.com")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "max retries exceeded")
	mockFetcher.AssertNumberOfCalls(t, "Get", 4) // 초기 호출 1회 + 재시도 3회
}

func TestRetryFetcher_Get_ServerErrorRetry(t *testing.T) {
	mockFetcher := new(RetryTestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 500 에러 발생 시 재시도
	errorResp := &http.Response{StatusCode: 500, Body: http.NoBody}
	successResp := &http.Response{StatusCode: 200}

	mockFetcher.On("Get", "http://example.com").Return(errorResp, nil).Once()
	mockFetcher.On("Get", "http://example.com").Return(successResp, nil).Once()

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, successResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Get", 2)
}
