package fetcher_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMimeTypeFetcher_Do(t *testing.T) {
	// 공통 테스트 데이터
	urlStr := "http://example.com"
	req := httptest.NewRequest(http.MethodGet, urlStr, nil)

	tests := []struct {
		name               string
		allowedMimeTypes   []string
		allowEmptyMimeType bool
		responseHeader     string // Delegate가 반환할 Content-Type 헤더
		delegateErr        error  // Delegate가 반환할 에러
		wantErr            bool
		errContains        string
	}{
		// 1. 정상 케이스
		{
			name:             "허용된 Content-Type (HTML)",
			allowedMimeTypes: []string{"text/html"},
			responseHeader:   "text/html; charset=utf-8",
			wantErr:          false,
		},
		{
			name:             "허용된 Content-Type (JSON)",
			allowedMimeTypes: []string{"text/html", "application/json"},
			responseHeader:   "application/json",
			wantErr:          false,
		},
		{
			name:             "대소문자 무시 (Text/HTML)",
			allowedMimeTypes: []string{"text/html"},
			responseHeader:   "Text/HTML; charset=utf-8",
			wantErr:          false,
		},
		{
			name:             "파라미터가 많은 Content-Type",
			allowedMimeTypes: []string{"multipart/form-data"},
			responseHeader:   "multipart/form-data; boundary=something; charset=utf-8",
			wantErr:          false,
		},

		// 2. 비정상 케이스 (허용되지 않음)
		{
			name:             "허용되지 않은 Content-Type (ZIP)",
			allowedMimeTypes: []string{"text/html"},
			responseHeader:   "application/zip",
			wantErr:          true,
			errContains:      "지원하지 않는 미디어 타입입니다",
		},
		{
			name:             "허용되지 않은 Content-Type (HTML 아님)",
			allowedMimeTypes: []string{"application/json"},
			responseHeader:   "text/html",
			wantErr:          true,
			errContains:      "지원하지 않는 미디어 타입입니다",
		},

		// 3. Content-Type 누락 처리
		{
			name:               "Content-Type 없음 (허용 설정)",
			allowedMimeTypes:   []string{"text/html"},
			allowEmptyMimeType: true,
			responseHeader:     "",
			wantErr:            false,
		},
		{
			name:               "Content-Type 없음 (거부 설정)",
			allowedMimeTypes:   []string{"text/html"},
			allowEmptyMimeType: false,
			responseHeader:     "",
			wantErr:            true,
			errContains:        "Content-Type 헤더가 누락되어 요청을 처리할 수 없습니다",
		},

		// 4. 파싱 실패 및 폴백
		{
			name:             "비표준 Content-Type 헤더 (폴백 동작 - 허용됨)",
			allowedMimeTypes: []string{"text/html"},
			// 세미콜론은 있지만 형식이 이상함. 폴백 로직은 세미콜론 앞을 사용 -> "text/html" 추출 -> 통과
			responseHeader: "text/html; invalid-parameter",
			wantErr:        false,
		},
		{
			name:             "비표준 Content-Type 헤더 (폴백 동작 - 거부됨)",
			allowedMimeTypes: []string{"application/json"},
			// 폴백 로직이 "text/html"을 추출하지만 allowed 목록에 없음 -> 실패
			responseHeader: "text/html; invalid-parameter",
			wantErr:        true,
			errContains:    "지원하지 않는 미디어 타입입니다",
		},

		// 5. 빈 목록 (모두 허용)
		{
			name:             "빈 allowedMimeTypes (모든 타입 허용 - HTML)",
			allowedMimeTypes: []string{}, // 빈 슬라이스
			responseHeader:   "text/html",
			wantErr:          false,
		},
		{
			name:             "빈 allowedMimeTypes (모든 타입 허용 - ZIP)",
			allowedMimeTypes: []string{}, // 빈 슬라이스
			responseHeader:   "application/zip",
			wantErr:          false,
		},

		// 6. Delegate 에러
		{
			name:             "Delegate Fetcher 에러 발생",
			allowedMimeTypes: []string{"text/html"},
			delegateErr:      errors.New("network error"),
			wantErr:          true,
			errContains:      "network error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock 설정
			mockFetcher := mocks.NewMockFetcher()
			response := &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString("body content")),
			}
			if tt.responseHeader != "" {
				response.Header.Set("Content-Type", tt.responseHeader)
			}

			// Delegate 동작 정의
			if tt.delegateErr != nil {
				mockFetcher.On("Do", mock.Anything).Return(nil, tt.delegateErr)
			} else {
				mockFetcher.On("Do", mock.Anything).Return(response, nil)
			}

			// MimeTypeFetcher 생성 및 실행
			f := fetcher.NewMimeTypeFetcher(mockFetcher, tt.allowedMimeTypes, tt.allowEmptyMimeType)
			resp, err := f.Do(req)

			// 검증
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				if resp != nil {
					_ = resp.Body.Close() // 리소스 정리
				}
			}

			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestMimeTypeFetcher_ResourceManagement(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)

	t.Run("검증 실패 시 Response Body가 닫혀야 함 (drainAndCloseBody)", func(t *testing.T) {
		// Mock Body 생성 (Close 호출 추적)
		mockBody := mocks.NewMockReadCloser("some content")

		// Mock Fetcher 설정
		mockFetcher := mocks.NewMockFetcher()
		response := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"Content-Type": []string{"application/zip"}}, // 비허용 타입
			Body:       mockBody,
		}
		mockFetcher.On("Do", mock.Anything).Return(response, nil)

		// Fetcher 실행
		f := fetcher.NewMimeTypeFetcher(mockFetcher, []string{"text/html"}, false)
		resp, err := f.Do(req)

		// 검증
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "지원하지 않는 미디어 타입입니다")
		assert.Nil(t, resp)

		// Body가 닫혔는지 확인
		assert.Equal(t, int64(1), mockBody.GetCloseCount(), "검증 실패 시 Body.Close()가 호출되어야 합니다")
		// drainAndCloseBody는 내부적으로 CopyBuffer를 통해 Read를 수행함 (limit reader)
		// Read 호출 여부는 drainAndCloseBody가 실제로 데이터를 읽었는지 확인하는 척도
		// (MockReadCloser 구현에 readCount 추적이 있다면 확인 가능)
		// 현재 mocks.MockReadCloser에는 readCount 추적 기능이 있는지 확인 필요 -> 확인됨 (WasRead())
		assert.True(t, mockBody.WasRead(), "drainAndCloseBody()는 커넥션 재사용을 위해 데이터를 읽어야 합니다")
	})

	t.Run("Delegate 에러 시 Response가 있으면 Body가 닫혀야 함", func(t *testing.T) {
		mockBody := mocks.NewMockReadCloser("partial content")
		mockFetcher := mocks.NewMockFetcher()

		// 에러와 함께 Response 반환 (예: 리다이렉트 중단 등)
		response := &http.Response{
			Body: mockBody,
		}
		mockFetcher.On("Do", mock.Anything).Return(response, errors.New("some error"))

		f := fetcher.NewMimeTypeFetcher(mockFetcher, []string{"text/html"}, false)
		resp, err := f.Do(req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, int64(1), mockBody.GetCloseCount())
		assert.True(t, mockBody.WasRead())
	})

	t.Run("Content-Type 누락(거부 설정) 시 Body가 닫혀야 함", func(t *testing.T) {
		mockBody := mocks.NewMockReadCloser("content without type")
		mockFetcher := mocks.NewMockFetcher()

		response := &http.Response{
			Header: make(http.Header), // Content-Type 없음
			Body:   mockBody,
		}
		mockFetcher.On("Do", mock.Anything).Return(response, nil)

		f := fetcher.NewMimeTypeFetcher(mockFetcher, []string{"text/html"}, false) // false = 거부
		resp, err := f.Do(req)

		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Contains(t, err.Error(), "Content-Type 헤더가 누락")
		assert.Equal(t, int64(1), mockBody.GetCloseCount())
		assert.True(t, mockBody.WasRead())
	})

	t.Run("정상 성공 시 Body는 닫히지 않고 반환되어야 함", func(t *testing.T) {
		mockBody := mocks.NewMockReadCloser("valid html")
		mockFetcher := mocks.NewMockFetcher()

		response := &http.Response{
			Header: http.Header{"Content-Type": []string{"text/html"}},
			Body:   mockBody,
		}
		mockFetcher.On("Do", mock.Anything).Return(response, nil)

		f := fetcher.NewMimeTypeFetcher(mockFetcher, []string{"text/html"}, false)
		resp, err := f.Do(req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, mockBody, resp.Body)
		assert.Equal(t, int64(0), mockBody.GetCloseCount(), "성공 시 미들웨어가 Body를 닫으면 안 됩니다")
	})
}
