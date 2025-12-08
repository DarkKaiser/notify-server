package task

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRetryFetcher_Get_Success(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	expectedResp := &http.Response{StatusCode: 200}
	// Get calls Do internally now
	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.URL.String() == "http://example.com" && req.Method == http.MethodGet
	})).Return(expectedResp, nil)

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Do", 1)
}

func TestRetryFetcher_Get_RetrySuccess(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 첫 번째, 두 번째 호출은 실패, 세 번째 호출은 성공
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("network error")).Once()
	expectedResp := &http.Response{StatusCode: 200}
	mockFetcher.On("Do", mock.Anything).Return(expectedResp, nil).Once()

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Do", 3)
}

func TestRetryFetcher_Get_MaxRetriesExceeded(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 모든 호출 실패
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("network error"))

	resp, err := retryFetcher.Get("http://example.com")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "max retries exceeded")
	mockFetcher.AssertNumberOfCalls(t, "Do", 4) // 초기 호출 1회 + 재시도 3회
}

func TestRetryFetcher_Get_ServerErrorRetry(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 500 에러 발생 시 재시도
	errorResp := &http.Response{StatusCode: 500, Body: http.NoBody}
	successResp := &http.Response{StatusCode: 200}

	mockFetcher.On("Do", mock.Anything).Return(errorResp, nil).Once()
	mockFetcher.On("Do", mock.Anything).Return(successResp, nil).Once()

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, successResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Do", 2)
}

func TestRetryFetcher_Get_InvalidURL(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 잘못된 URL로 인한 http.NewRequest 에러 테스트
	// 제어 문자가 포함된 URL 등
	resp, err := retryFetcher.Get(":")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to create request")
	mockFetcher.AssertNotCalled(t, "Do")
}

func TestRetryFetcher_Do_ContextCancelled(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	// Retry delay large enough to ensure we hit the cancel in the test
	retryFetcher := NewRetryFetcher(mockFetcher, 3, 100*time.Millisecond)

	// First call fails
	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("fail")).Once()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://example.com", nil)

	// Cancel shortly after starting to ensure we catch it during the sleep
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := retryFetcher.Do(req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "error should be context.Canceled")
	mockFetcher.AssertNumberOfCalls(t, "Do", 1) // Should only try once
}

func TestRetryFetcher_Do_RequestBodyRetry(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	bodyContent := []byte("test body")
	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewBuffer(bodyContent))

	// http.NewRequest는 GetBody를 자동으로 설정해줍니다.

	// 첫 번째 호출 실패 (Body 읽음)
	mockFetcher.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		// Body가 잘 전달되었는지 확인
		buf := new(bytes.Buffer)
		if r.Body != nil {
			io.Copy(buf, r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(buf.Bytes())) // Body 복구 for next read inside mock if needed (though not needed here)
		}
		return bytes.Equal(buf.Bytes(), bodyContent)
	})).Return(nil, errors.New("network error")).Once()

	// 두 번째 호출 성공 (Body 복구되어야 함)
	expectedResp := &http.Response{StatusCode: 200}
	mockFetcher.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		buf := new(bytes.Buffer)
		if r.Body != nil {
			io.Copy(buf, r.Body)
		}
		return bytes.Equal(buf.Bytes(), bodyContent)
	})).Return(expectedResp, nil).Once()

	resp, err := retryFetcher.Do(req)

	assert.NoError(t, err)
	assert.Equal(t, expectedResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Do", 2)
}

func TestRetryFetcher_Get_TooManyRequestsRetry(t *testing.T) {
	mockFetcher := new(TestMockFetcher)
	retryFetcher := NewRetryFetcher(mockFetcher, 3, time.Millisecond)

	// 429 에러 발생 시 재시도
	rateLimitResp := &http.Response{StatusCode: http.StatusTooManyRequests, Body: http.NoBody}
	successResp := &http.Response{StatusCode: 200}

	mockFetcher.On("Do", mock.Anything).Return(rateLimitResp, nil).Once()
	mockFetcher.On("Do", mock.Anything).Return(successResp, nil).Once()

	resp, err := retryFetcher.Get("http://example.com")

	assert.NoError(t, err)
	assert.Equal(t, successResp, resp)
	mockFetcher.AssertNumberOfCalls(t, "Do", 2)
}
