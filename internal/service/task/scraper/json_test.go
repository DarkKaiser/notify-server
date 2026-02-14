package scraper

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestFetchJSON_Comprehensive는 FetchJSON의 모든 동작을 검증하는 테이블 주도 테스트(Table Driven Test)입니다.
func TestFetchJSON_Comprehensive(t *testing.T) {
	// Logrus Hook을 사용하여 로그 메시지를 검증합니다.
	hook := test.NewGlobal()

	// 테스트에 사용할 공통 데이터 구조체
	type TestResponse struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	type NestedResponse struct {
		Data []TestResponse `json:"data"`
	}

	tests := []struct {
		name string
		// Input Parameters
		method     string
		url        string
		header     http.Header
		body       any // 요청 본문 (nil, string, []byte, struct)
		targetV    any // 디코딩 대상 변수 (Validation 테스트용)
		scraperOpt []Option

		// Mock Setup
		setupMock func(*testing.T, *mocks.MockFetcher)
		ctxSetup  func() (context.Context, context.CancelFunc)

		// Verification
		wantErr        bool
		errType        apperrors.ErrorType
		errContains    []string
		checkLog       func(*testing.T, *test.Hook)
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
					// DefaultHeader check
					assert.Equal(t, "application/json", req.Header.Get("Accept"))
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
			// http.Header는 map[string][]string이며, Get 메서드는 Key를 Canonical Form으로 변환하여 찾습니다.
			// 따라서 map 리터럴로 초기화할 때는 Canonical Key("X-Test-Id")를 사용해야 Get("X-Test-ID")가 동작합니다.
			header: http.Header{"X-Test-Id": []string{"12345"}},
			body:   map[string]string{"foo": "bar"},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "created", "value": 201}`
				resp := mocks.NewMockResponse(jsonContent, 201)
				resp.Header.Set("Content-Type", "application/json")

				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "POST", req.Method)
					assert.Equal(t, "application/json", req.Header.Get("Content-Type")) // Auto-added header
					assert.Equal(t, "12345", req.Header.Get("X-Test-ID"))

					// Verify Body Content
					buf := new(bytes.Buffer)
					buf.ReadFrom(req.Body)
					assert.JSONEq(t, `{"foo":"bar"}`, buf.String())
				}).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "created", res.Name)
				assert.Equal(t, 201, res.Value)
			},
		},
		{
			name:    "Success - Nested JSON Structure",
			method:  "GET",
			url:     "http://example.com/nested",
			targetV: &NestedResponse{},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"data": [{"name": "A", "value": 1}, {"name": "B", "value": 2}]}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "application/json")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*NestedResponse)
				assert.Len(t, res.Data, 2)
				assert.Equal(t, "A", res.Data[0].Name)
				assert.Equal(t, 2, res.Data[1].Value)
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
			checkLog: nil, // charset.NewReader often handles invalid charsets gracefully without error
		},

		// =================================================================================
		// Group 4: 에러 상황 (Error Handling)
		// =================================================================================
		{
			name:   "Error - Request Body Marshal Failure",
			method: "POST",
			url:    "http://example.com/fail-marshal",
			body:   func() {}, // func is not marshallable
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				// Fetcher not called due to marshal error
			},
			wantErr:     true,
			errType:     apperrors.Internal,
			errContains: []string{"요청 본문 JSON 인코딩 실패"},
		},
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
			scraperOpt: []Option{
				WithMaxResponseBodySize(10),
			},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				longJSON := `{"name": "very_long_name_exceeding_limit"}`
				resp := mocks.NewMockResponse(longJSON, 200)
				resp.Header.Set("Content-Type", "application/json")
				// resp.IsTruncated = true (X) - Handled internally within scraper implementation or fetcher
				// Here we simulate truncated flag being set by fetcher if `MaxBodySize` was passed to it,
				// BUT: In `FetchJSON`, we rely on `s.executeRequest` which uses `fetcher` and reads into buffer.
				// The scraper's `executeRequest` logic handles truncation check if it implements limiting.
				// Let's assume standard behavior: if logic inside `executeRequest` sets Truncated.
				// Since we mock `Fetcher.Do`, we can't easily set `IsTruncated` inside `executeRequest` unless
				// we change how `executeRequest` works or mock it closer.
				// In `json.go`: `result, logger, err := s.executeRequest(...)`
				// `executeRequest` reads body.
				// If we want to test truncation logic in `decodeJSONResponse`, we can rely on `executeRequest` behavior.
				// However, `executeRequest` in `request_sender.go` does: `io.CopyN` or `LimitReader`.
				// To force truncation in this test without complex mocks, we can rely on the fact that
				// the real `executeRequest` will set `IsTruncated` if read hits limit.
				// Let's pass a response body larger than limit.

				// Re-verify `request_sender.go`: `executeRequest` implementation details.
				// Assuming `executeRequest` respects `s.maxResponseBodySize`.
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
			checkLog: func(t *testing.T, hook *test.Hook) {
				// Verify snippet logging
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.ErrorLevel && strings.Contains(entry.Message, "JSON 데이터 변환 실패") {
						if ctx, ok := entry.Data["syntax_error_context"]; ok {
							assert.Contains(t, ctx, "bad")
							found = true
						}
					}
				}
				assert.True(t, found, "Expected error log with syntax context")
			},
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
		{
			name:   "Error - Context Canceled",
			method: "GET",
			url:    "http://example.com/cancel",
			ctxSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				return ctx, cancel
			},
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				// Simulate context cancellation during fetch
				// We can return context error directly
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					// Cancel context inside call
					// But usually context is canceled before or during.
					// Here we simulate external cancellation.
				}).Return(nil, context.Canceled)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"요청 중단"},
		},

		// =================================================================================
		// Group 5: Warning Log Verification
		// =================================================================================
		{
			name:   "Warning - Non-Standard Content Type (text/plain)",
			method: "GET",
			url:    "http://example.com/text-plain",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "plain", "value": 1}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Set("Content-Type", "text/plain")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "plain", res.Name)
			},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "비표준 Content-Type") {
						found = true
					}
				}
				assert.True(t, found, "Expected warning log for text/plain content type")
			},
		},
		{
			name:   "Warning - Empty Content Type",
			method: "GET",
			url:    "http://example.com/empty-ct",
			setupMock: func(t *testing.T, m *mocks.MockFetcher) {
				jsonContent := `{"name": "empty", "value": 0}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				resp.Header.Del("Content-Type")
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			validateResult: func(t *testing.T, v any) {
				res := v.(*TestResponse)
				assert.Equal(t, "empty", res.Name)
			},
			checkLog: func(t *testing.T, hook *test.Hook) {
				found := false
				for _, entry := range hook.AllEntries() {
					if entry.Level == logrus.WarnLevel && strings.Contains(entry.Message, "비표준 Content-Type") {
						found = true
					}
				}
				assert.True(t, found, "Expected warning log for empty content type")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()

			// 1. Setup Mock
			mockFetcher := &mocks.MockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(t, mockFetcher)
			}

			// 2. Prepare Body
			var bodyArg any
			if tt.body != nil {
				bodyArg = tt.body
			}

			// 3. Setup Context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxSetup != nil {
				ctx, cancel = tt.ctxSetup()
			} else {
				// Default context with timeout to prevent hanging tests
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			}
			if cancel != nil {
				defer cancel()
			}

			// If we are testing cancellation, we might need to cancel before call
			if strings.Contains(tt.name, "Context Canceled") {
				cancel()
			}

			// 4. Initialize Scraper
			s := New(mockFetcher, tt.scraperOpt...)

			// 5. Set Target Variable
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

			// 6. Execute
			// Note: We pass `bodyArg` (interface{}) directly, not `bodyReader`.
			// `FetchJSON` expects `body any` and marshals it.
			err := s.FetchJSON(ctx, tt.method, tt.url, bodyArg, tt.header, targetV)

			// 7. Verify Error
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

			// 8. Verify Log
			if tt.checkLog != nil {
				tt.checkLog(t, hook)
			}

			// 9. Verify Mock Expectations
			mockFetcher.AssertExpectations(t)
		})
	}
}

func init() {
	// Set log level to Debug for verification
	applog.SetLevel(applog.DebugLevel)
}
