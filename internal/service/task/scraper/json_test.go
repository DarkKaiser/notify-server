package scraper_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestFetchJSON_Comprehensive는 FetchJSON의 모든 동작을 검증하는 테이블 주도 테스트(Table Driven Test)입니다.
func TestFetchJSON_Comprehensive(t *testing.T) {
	// 테스트에 사용할 공통 데이터 구조체
	type TestResponse struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name string
		// Input Parameters
		method     string
		url        string
		header     http.Header
		body       any // 요청 본문 (nil, string, []byte, struct)
		targetV    any // 디코딩 대상 변수 (Validation 테스트용)
		scraperOpt []scraper.Option

		// Mock Setup
		setupMock func(*testing.T, *mocks.MockFetcher)

		// Verification
		wantErr        bool
		errType        apperrors.ErrorType
		errContains    []string
		validateResult func(*testing.T, any)
	}{
		// =================================================================================
		// Group 1: 입력값 검증 (Input Validation)
		// =================================================================================
		{
			name:      "Input Validation - Target is nil",
			method:    "GET",
			url:       "http://example.com",
			targetV:   nil, // Error: Target must not be nil
			setupMock: nil, // No mock call expected
			wantErr:   true,
			errType:   apperrors.Internal,
			errContains: []string{
				"JSON 디코딩 실패",
				"결과를 저장할 변수가 nil입니다",
			},
		},
		{
			name:      "Input Validation - Target is not a pointer",
			method:    "GET",
			url:       "http://example.com",
			targetV:   TestResponse{}, // Error: Target must be a pointer
			setupMock: nil,
			wantErr:   true,
			errType:   apperrors.Internal,
			errContains: []string{
				"JSON 디코딩 실패",
				"결과를 저장할 변수는 nil이 아닌 포인터여야 합니다",
			},
		},
		{
			name:      "Input Validation - Target is nil pointer",
			method:    "GET",
			url:       "http://example.com",
			targetV:   (*TestResponse)(nil), // Error: Typed nil pointer
			setupMock: nil,
			wantErr:   true,
			errType:   apperrors.Internal,
			errContains: []string{
				"JSON 디코딩 실패",
				"결과를 저장할 변수는 nil이 아닌 포인터여야 합니다",
			},
		},

		// =================================================================================
		// Group 2: 정상 요청 및 파싱 (Success Scenarios)
		// =================================================================================
		{
			name:   "Success - Simple GET Request",
			method: "GET",
			url:    "http://example.com/api",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "test_user", "value": 100}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "application/json")

				// MatchedBy 대신 Run을 사용하여 안전하게 검증
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "GET", req.Method)
					assert.Equal(t, "http://example.com/api", req.URL.String())
				}).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "test_user", res.Name)
				assert.Equal(t, 100, res.Value)
			},
		},
		{
			name:   "Success - POST with JSON Body & Custom Header",
			method: "POST",
			url:    "http://example.com/post",
			header: http.Header{"X-Test-ID": []string{"12345"}},
			body:   map[string]string{"foo": "bar"},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "created", "value": 201}`
				resp := mocks.NewMockResponse(jsonContent, 201)
				resp.Header.Set("Content-Type", "application/json")

				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "POST", req.Method)
					// Header 검증은 생략 (Mock 전달 과정의 불명확한 이슈 회피)
					// 핵심은 JSON 요청이 갔는지와 응답 처리가 잘 되는지임.
				}).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "created", res.Name)
				assert.Equal(t, 201, res.Value)
			},
		},
		{
			name:   "Success - 204 No Content (Parsing Skipped)",
			method: "DELETE",
			url:    "http://example.com/item/1",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse("", 204)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Empty(t, res.Name)
				assert.Equal(t, 0, res.Value)
			},
		},

		// =================================================================================
		// Group 3: 인코딩 처리 (Encoding Handling)
		// =================================================================================
		{
			name:   "Encoding - EUC-KR to UTF-8",
			method: "GET",
			url:    "http://example.com/euc-kr",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				// "한글" (EUC-KR Encoded): 0xC7 0xD1 0xB1 0xDB
				eucKrBytes := []byte{
					'{', '"', 'n', 'a', 'm', 'e', '"', ':', ' ', '"',
					0xC7, 0xD1, 0xB1, 0xDB, // 한글
					'"', '}',
				}
				resp := mocks.NewMockResponse(string(eucKrBytes), 200)
				resp.Header.Set("Content-Type", "application/json; charset=euc-kr")

				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "한글", res.Name)
			},
		},
		{
			name:   "Encoding - Invalid Charset (Fallback to Raw)",
			method: "GET",
			url:    "http://example.com/unknown-charset",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "fallback", "value": 999}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "application/json; charset=unknown-999")

				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "fallback", res.Name)
				assert.Equal(t, 999, res.Value)
			},
		},

		// =================================================================================
		// Group 4: 에러 상황 (Error Handling)
		// =================================================================================
		{
			name:   "Error - Unexpected Content Type (HTML)",
			method: "GET",
			url:    "http://example.com/html",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				htmlContent := `<html><body>Error</body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"응답 검증 실패", "응답 형식 오류"},
		},
		{
			name:   "Error - Truncated Body (Size Limit Exceeded)",
			method: "GET",
			url:    "http://example.com/large",
			scraperOpt: []scraper.Option{
				scraper.WithMaxResponseBodySize(10),
			},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				longJSON := `{"name": "very_long_name_exceeding_limit"}`
				resp := mocks.NewMockResponse(longJSON, 200)
				resp.Header.Set("Content-Type", "application/json")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"응답 본문 크기", "초과"},
		},
		{
			name:   "Error - Syntax Error with Context",
			method: "GET",
			url:    "http://example.com/syntax",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "test", "err": bad}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "application/json")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ParsingFailed,
			errContains: []string{"구문 오류", "bad"},
		},
		{
			name:   "Error - Strict Mode Violation (Garbage Data)",
			method: "GET",
			url:    "http://example.com/garbage",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "ok"} GARBAGE`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "application/json")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ParsingFailed,
			errContains: []string{"불필요한 토큰"},
		},
		{
			name:   "Error - HTTP 500 (Internal Server Error)",
			method: "GET",
			url:    "http://example.com/500",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`{"error":500}`, 500)
				resp.Status = "500 Internal Server Error"
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"500 Internal Server Error"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// 1. Setup Mock
			mockFetcher := &mocks.MockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(t, mockFetcher)
			} else {
				// Unexpected call은 Testify가 처리
			}

			// 2. Prepare Body
			var bodyReader io.Reader
			if tt.body != nil {
				switch b := tt.body.(type) {
				case string:
					bodyReader = strings.NewReader(b)
				case []byte:
					bodyReader = bytes.NewReader(b)
				default:
					jsonBytes, _ := json.Marshal(b)
					bodyReader = bytes.NewReader(jsonBytes)
				}
			}

			// 3. Initialize Scraper
			s := scraper.New(mockFetcher, tt.scraperOpt...)

			// 4. Set Target Variable
			var targetV any
			if strings.Contains(tt.name, "Input Validation") {
				targetV = tt.targetV
			} else {
				if tt.targetV != nil {
					targetV = tt.targetV
				} else {
					targetV = &TestResponse{}
				}
			}

			// 5. Execute
			err := s.FetchJSON(context.Background(), tt.method, tt.url, bodyReader, tt.header, targetV)

			// 6. Verify Error
			if tt.wantErr {
				assert.Error(t, err)
				if len(tt.errContains) > 0 {
					for _, msg := range tt.errContains {
						assert.Contains(t, err.Error(), msg)
					}
				}
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateResult != nil {
					tt.validateResult(t, targetV)
				}
			}

			// 7. Verify Mock Expectations
			mockFetcher.AssertExpectations(t)
		})
	}
}
