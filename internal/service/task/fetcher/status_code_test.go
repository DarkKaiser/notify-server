package fetcher_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestStatusCodeFetcher_Factory(t *testing.T) {
	mockFetcher := mocks.NewMockFetcher()

	t.Run("NewStatusCodeFetcher (기본값)", func(t *testing.T) {
		f := fetcher.NewStatusCodeFetcher(mockFetcher)
		assert.NotNil(t, f)
		// delegate 등 내부 필드는 외부 패키지에서 접근 불가하므로 테스트 제외하거나 필요 시 리플렉션 사용
		// 여기서는 공개된 메서드로 동작 검증이 충분하므로 필드 검증은 생략
	})

	t.Run("NewStatusCodeFetcherWithOptions (옵션 지정)", func(t *testing.T) {
		allowed := []int{http.StatusOK, http.StatusCreated, http.StatusAccepted}
		f := fetcher.NewStatusCodeFetcherWithOptions(mockFetcher, allowed...)
		assert.NotNil(t, f)
	})

	t.Run("NewStatusCodeFetcherWithOptions (빈 옵션)", func(t *testing.T) {
		f := fetcher.NewStatusCodeFetcherWithOptions(mockFetcher)
		assert.NotNil(t, f)
	})
}

func TestStatusCodeFetcher_Do(t *testing.T) {
	tests := []struct {
		name               string
		allowedStatusCodes []int
		delegateResponse   *http.Response
		delegateError      error
		expectedError      bool
		expectedErrType    apperrors.ErrorType
		expectedSnippet    string
	}{
		{
			name:             "성공: 200 OK (기본 설정)",
			delegateResponse: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewBufferString("success"))},
			expectedError:    false,
		},
		{
			name:               "성공: 201 Created (사용자 정의 설정)",
			allowedStatusCodes: []int{http.StatusCreated},
			delegateResponse:   &http.Response{StatusCode: http.StatusCreated, Body: io.NopCloser(bytes.NewBufferString("created"))},
			expectedError:      false,
		},
		{
			name:             "실패: 404 Not Found (기본 설정)",
			delegateResponse: &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(bytes.NewBufferString("page not found"))},
			expectedError:    true,
			expectedErrType:  apperrors.NotFound,
			expectedSnippet:  "page not found",
		},
		{
			name:             "실패: 500 Internal Server Error",
			delegateResponse: &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewBufferString("server error"))},
			expectedError:    true,
			expectedErrType:  apperrors.Unavailable,
			expectedSnippet:  "server error",
		},
		{
			name:             "실패: 403 Forbidden",
			delegateResponse: &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(bytes.NewBufferString("access denied"))},
			expectedError:    true,
			expectedErrType:  apperrors.Forbidden,
			expectedSnippet:  "access denied",
		},
		{
			name:             "실패: 429 Too Many Requests",
			delegateResponse: &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(bytes.NewBufferString("rate limit"))},
			expectedError:    true,
			expectedErrType:  apperrors.Unavailable,
			expectedSnippet:  "rate limit",
		},
		{
			name:          "실패: Delegate 에러 발생",
			delegateError: errors.New("network error"),
			expectedError: true,
		},
		{
			name: "실패: Delegate 에러와 함께 부분 응답 반환 (리소스 정리 확인)",
			delegateResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBufferString("partial content")),
			},
			delegateError: errors.New("read error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 설정
			mockFetcher := mocks.NewMockFetcher()

			// 응답에 Request 설정이 필요한 경우 처리 (CheckResponseStatus 로깅 등)
			// 실제로는 nil이어도 동작하지만, 테스트의 명확성을 위해 필요시 설정 가능
			// 여기서는 Testify의 한계로 인해 Response 객체를 그대로 반환하도록 함
			mockFetcher.On("Do", mock.Anything).Return(tt.delegateResponse, tt.delegateError)

			// Fetcher 생성
			var f *fetcher.StatusCodeFetcher
			if tt.allowedStatusCodes != nil {
				f = fetcher.NewStatusCodeFetcherWithOptions(mockFetcher, tt.allowedStatusCodes...)
			} else {
				f = fetcher.NewStatusCodeFetcher(mockFetcher)
			}

			// 요청 실행
			req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
			resp, err := f.Do(req)

			// 검증
			if tt.expectedError {
				require.Error(t, err)
				if tt.expectedErrType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.expectedErrType), "에러 타입이 일치해야 합니다. got: %v", err)
				}
				if tt.expectedSnippet != "" {
					assert.Contains(t, err.Error(), tt.expectedSnippet)
				}
				assert.Nil(t, resp, "에러 발생 시 응답은 nil이어야 합니다")
			} else {
				require.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.delegateResponse.StatusCode, resp.StatusCode)
			}
		})
	}
}

func TestStatusCodeFetcher_ResourceSafety(t *testing.T) {
	t.Run("상태 코드 검증 실패 시 Body가 닫혀야 함", func(t *testing.T) {
		body := mocks.NewMockReadCloser("error content")

		mockFetcher := mocks.NewMockFetcher()
		mockFetcher.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       body,
		}, nil)

		f := fetcher.NewStatusCodeFetcher(mockFetcher)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		resp, err := f.Do(req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, int64(1), body.GetCloseCount(), "에러 발생 후 Body가 닫혀야 합니다")
	})

	t.Run("Delegate 에러 시 부분 응답의 Body가 닫혀야 함", func(t *testing.T) {
		body := mocks.NewMockReadCloser("partial")

		mockFetcher := mocks.NewMockFetcher()
		mockFetcher.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, errors.New("network error"))

		f := fetcher.NewStatusCodeFetcher(mockFetcher)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		resp, err := f.Do(req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, int64(1), body.GetCloseCount(), "Delegate 에러 시에도 응답 바디가 닫혀야 합니다")
	})

	t.Run("성공 시 Body는 닫히지 않아야 함", func(t *testing.T) {
		body := mocks.NewMockReadCloser("success")

		mockFetcher := mocks.NewMockFetcher()
		mockFetcher.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: http.StatusOK,
			Body:       body,
		}, nil)

		f := fetcher.NewStatusCodeFetcher(mockFetcher)
		req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
		resp, err := f.Do(req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int64(0), body.GetCloseCount(), "성공 시에는 Body가 열려 있어야 합니다")

		// 테스트 종료 후 닫기
		resp.Body.Close()
	})
}
