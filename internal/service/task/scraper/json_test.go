package scraper_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestFetchJSON_Table(t *testing.T) {
	type TestData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	tests := []struct {
		name        string
		method      string
		url         string
		header      http.Header
		body        interface{} // Object to serialize to JSON for body
		setupMock   func(*mocks.MockFetcher)
		wantErr     bool
		errType     apperrors.ErrorType
		errContains string
		validateRes func(*testing.T, TestData)
	}{
		{
			name:   "Success - POST with Body and Header",
			method: "POST",
			url:    "http://example.com",
			header: http.Header{"X-Custom": []string{"HeaderVal"}},
			body:   map[string]string{"input": "data"},
			setupMock: func(m *mocks.MockFetcher) {
				jsonContent := `{"name": "test", "value": 123}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				m.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					// Verify Request Properties
					if req.Method != "POST" || req.URL.String() != "http://example.com" {
						return false
					}
					if req.Header.Get("X-Custom") != "HeaderVal" {
						return false
					}

					// Verify Body Read
					if req.Body == nil {
						return false
					}
					bodyBytes, _ := io.ReadAll(req.Body)
					req.Body.Close() // Important to close/reset for further use if needed, but here it's consumed
					return strings.Contains(string(bodyBytes), `"input":"data"`)
				})).Return(resp, nil)
			},
			validateRes: func(t *testing.T, res TestData) {
				assert.Equal(t, "test", res.Name)
				assert.Equal(t, 123, res.Value)
			},
		},
		{
			name:   "Error - JSON Parsing (Invalid Type)",
			method: "GET",
			url:    "http://example.com",
			setupMock: func(m *mocks.MockFetcher) {
				jsonContent := `{"name": "test", "value": "invalid"}`
				resp := mocks.NewMockResponse(jsonContent, 200)
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ParsingFailed, // Parsing error
			errContains: "JSON 변환이 실패하였습니다",
		},
		{
			name:   "Error - HTTP 404 Status",
			method: "GET",
			url:    "http://example.com/404",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`{"error": "not found"}`, 404)
				resp.Status = "404 Not Found"
				// Request is needed for URL redaction in error message
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/404", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "{\"error\": \"not found\"}",
		},
		{
			name:   "Error - HTTP 500 Status (Unavailable)",
			method: "GET",
			url:    "http://example.com/500",
			setupMock: func(m *mocks.MockFetcher) {
				resp := mocks.NewMockResponse(`error`, 500)
				resp.Status = "500 Internal Server Error"
				// Request is needed for URL redaction in error message
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/500", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errContains: "error",
		},
		{
			name:   "Error - HTTP 5xx with HTML Content (Should return Unavailable, not ExecutionFailed)",
			method: "GET",
			url:    "http://example.com/502",
			setupMock: func(m *mocks.MockFetcher) {
				htmlError := `<html><body>502 Bad Gateway</body></html>`
				resp := mocks.NewMockResponse(htmlError, 502)
				resp.Status = "502 Bad Gateway"
				resp.Header.Set("Content-Type", "text/html") // HTML Content-Type

				req, _ := http.NewRequest(http.MethodGet, "http://example.com/502", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable, // 중요: 파싱 에러가 아니라 502 에러여야 함
			errContains: "502 Bad Gateway",
		},
		{
			name:   "Error - HTML Content Type in JSON Request",
			method: "GET",
			url:    "http://example.com/html-error",
			setupMock: func(m *mocks.MockFetcher) {
				htmlContent := `<html><body>Error</body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "text/html; charset=utf-8")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/html-error", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: "유효하지 않은 응답 형식: JSON을 기대했으나 HTML 응답이 수신되었습니다",
		},
		{
			name:   "Error - HTML Content Type Case Insensitive",
			method: "GET",
			url:    "http://example.com/html-case",
			setupMock: func(m *mocks.MockFetcher) {
				htmlContent := `<html><body>Error</body></html>`
				resp := mocks.NewMockResponse(htmlContent, 200)
				resp.Header.Set("Content-Type", "TEXT/HTML")
				req, _ := http.NewRequest(http.MethodGet, "http://example.com/html-case", nil)
				resp.Request = req
				m.On("Do", mock.Anything).Return(resp, nil)
			},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: "유효하지 않은 응답 형식: JSON을 기대했으나 HTML 응답이 수신되었습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mocks.MockFetcher{}
			if tt.setupMock != nil {
				tt.setupMock(mockFetcher)
			}

			// Prepare Body Reader
			var bodyReader io.Reader
			if tt.body != nil {
				jsonBytes, _ := json.Marshal(tt.body)
				bodyReader = bytes.NewReader(jsonBytes)
			}

			var result TestData
			s := scraper.New(mockFetcher)
			err := s.FetchJSON(context.Background(), tt.method, tt.url, bodyReader, tt.header, &result)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				assert.NoError(t, err)
				if tt.validateRes != nil {
					tt.validateRes(t, result)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}

func TestFetchJSON_NoContent(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse("", 204)
	resp.Status = "204 No Content"
	// No Content-Type needed for 204
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/204", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]interface{}
	err := s.FetchJSON(context.Background(), "GET", "http://example.com/204", nil, nil, &result)

	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestFetchJSON_SyntaxError_Context(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// Invalid JSON: unquoted string value triggers SyntaxError (invalid character 'v' looking for beginning of value)
	jsonContent := `{"name": "test", "values": [1, 2, 3], "key": value}`
	resp := mocks.NewMockResponse(jsonContent, 200)
	resp.Header.Set("Content-Type", "application/json")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/syntax", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]interface{}
	err := s.FetchJSON(context.Background(), "GET", "http://example.com/syntax", nil, nil, &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SyntaxError at offset")
	// The snippet should show where it failed (around "value")
	assert.Contains(t, err.Error(), "value")
}

func TestFetchJSON_SyntaxError_Encoding(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}

	// EUC-KR로 인코딩된 JSON 데이터 생성 (구문 에러 포함)
	// {"key": "값"} -> "값"은 EUC-KR에서 2바이트 (B0 AA), UTF-8에서 3바이트 (EA B0 80)
	// {"key": "한글", "err": bad}  <-- bad는 따옴표 없는 문자열로 유효하지 않음

	// '한글' (EUC-KR)
	// 한: 0xC7, 0xD1
	// 글: 0xB1, 0xDB
	eucKrData := []byte{
		'{', '"', 'k', 'e', 'y', '"', ':', ' ', '"',
		0xC7, 0xD1, 0xB1, 0xDB, // 한글 (EUC-KR)
		'"', ',', ' ', '"', 'e', 'r', 'r', '"', ':', ' ', 'b', 'a', 'd', // "err": bad
		'}',
	}

	resp := mocks.NewMockResponse(string(eucKrData), 200)
	resp.Header.Set("Content-Type", "application/json; charset=euc-kr")
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/syntax-encoding", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]interface{}
	err := s.FetchJSON(context.Background(), "GET", "http://example.com/syntax-encoding", nil, nil, &result)

	assert.Error(t, err)
	// 에러 메시지에 "한글"이 깨지지 않고 올바르게 포함되어야 함
	assert.Contains(t, err.Error(), "한글")
}

func TestFetchJSON_RawBody(t *testing.T) {
	t.Run("String Body", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		resp := mocks.NewMockResponse(`{}`, 200)
		resp.Header.Set("Content-Type", "application/json")

		rawBody := `{"key": "value"}`

		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			if req.Body == nil {
				return false
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(req.Body)
			// Reset body for potential future reads (mocks usually consume it once)
			req.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
			return buf.String() == rawBody
		})).Return(resp, nil)

		s := scraper.New(mockFetcher)
		var result map[string]interface{}

		// Pass string directly
		err := s.FetchJSON(context.Background(), "POST", "http://example.com/raw-string", rawBody, nil, &result)

		assert.NoError(t, err)
		mockFetcher.AssertExpectations(t)
	})

	t.Run("Byte Slice Body", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		resp := mocks.NewMockResponse(`{}`, 200)
		resp.Header.Set("Content-Type", "application/json")

		rawBody := []byte(`{"key": "byte"}`)

		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			if req.Body == nil {
				return false
			}
			buf := new(bytes.Buffer)
			buf.ReadFrom(req.Body)
			req.Body = io.NopCloser(bytes.NewReader(buf.Bytes()))
			return buf.String() == string(rawBody)
		})).Return(resp, nil)

		s := scraper.New(mockFetcher)
		var result map[string]interface{}

		// Pass []byte directly
		err := s.FetchJSON(context.Background(), "POST", "http://example.com/raw-byte", rawBody, nil, &result)

		assert.NoError(t, err)
		mockFetcher.AssertExpectations(t)
	})
}

func TestFetchJSON_TrailingData(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		// Valid JSON followed by garbage
		Body: io.NopCloser(strings.NewReader(`{"key": "value"} garbage`)),
	}
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]string
	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "(Unexpected Token)")
}

func TestFetchJSON_ValidationFailure_Visibility(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/html"}, // Wrong content type for JSON request
		},
		Body: io.NopCloser(strings.NewReader(`<html><body>Error</body></html>`)),
	}
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]string
	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.Error(t, err)
	// The error message should contain the body snippet
	assert.Contains(t, err.Error(), "<html><body>Error</body></html>")
}

func TestFetchJSON_RequestBodyLimitTests(t *testing.T) {
	// 5 bytes limit
	limit := int64(5)

	// Mock fetcher (not used for this test but required for New)
	mockFetcher := &mocks.MockFetcher{}

	// Scraper with limit
	s := scraper.New(mockFetcher, scraper.WithMaxRequestBodySize(limit))

	// Large body (larger than 5 bytes)
	largeBody := strings.NewReader("1234567890")

	err := s.FetchJSON(context.Background(), "POST", "http://example.com", largeBody, nil, &map[string]any{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "요청 본문 크기 초과")
}

func TestFetchJSON_DefaultContentType(t *testing.T) {
	t.Run("Raw String", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Header.Get("Content-Type") == "application/json"
		})).Return(mocks.NewMockResponse(`{}`, 200), nil)

		s := scraper.New(mockFetcher)
		err := s.FetchJSON(context.Background(), "POST", "http://example.com", `{"foo":"bar"}`, nil, &map[string]any{})

		assert.NoError(t, err)
		mockFetcher.AssertExpectations(t)
	})

	t.Run("Bytes", func(t *testing.T) {
		mockFetcher := &mocks.MockFetcher{}
		mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
			return req.Header.Get("Content-Type") == "application/json"
		})).Return(mocks.NewMockResponse(`{}`, 200), nil)

		s := scraper.New(mockFetcher)
		err := s.FetchJSON(context.Background(), "POST", "http://example.com", []byte(`{"foo":"bar"}`), nil, &map[string]any{})

		assert.NoError(t, err)
		mockFetcher.AssertExpectations(t)
	})
}

func TestFetchJSON_SyntaxError_Logging(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse(`{"valid": "json" garbage`, 200)
	resp.Header.Set("Content-Type", "application/json")
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)

	var result map[string]any
	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "garbage")
	assert.Contains(t, err.Error(), "SyntaxError")
}

func TestFetchJSON_SyntaxError_Optimization(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	s := scraper.New(mockFetcher)

	// Invalid JSON: "{"key": @}" (invalid character)
	invalidJSON := `{"key": @}`
	resp := mocks.NewMockResponse(invalidJSON, 200)
	resp.Header.Set("Content-Type", "application/json")

	// Set Request for logging context
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp.Request = req

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	var result map[string]interface{}
	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.Error(t, err)
	// Check if Error message contains the context correctly (optimization shouldn't break this)
	assert.Contains(t, err.Error(), "invalid character") // SyntaxError's message
	assert.Contains(t, err.Error(), "@")
}

func TestFetchJSON_WithGenericReader_ShouldSupportRetry(t *testing.T) {
	// Arrange
	mockFetcher := &mocks.MockFetcher{}

	s := scraper.New(mockFetcher)
	ctx := context.Background()
	url := "http://example.com/api"
	requestBodyContent := `{"key": "value"}`
	bodyReader := io.NopCloser(strings.NewReader(requestBodyContent))

	mockFetcher.On("Do", mock.Anything).Run(func(args mock.Arguments) {
		req := args.Get(0).(*http.Request)
		// Crucial verification: GetBody should NOT be nil if our fix works.
		assert.NotNil(t, req.GetBody, "GetBody should be set for generic io.Reader to support retries")

		if req.GetBody != nil {
			// Verify content
			rc, err := req.GetBody()
			assert.NoError(t, err)
			content, _ := io.ReadAll(rc)
			assert.Equal(t, requestBodyContent, string(content))
			rc.Close()
		}
	}).Return(mocks.NewMockResponse(`{}`, 200), nil)

	// Act
	var result map[string]interface{}
	err := s.FetchJSON(ctx, http.MethodPost, url, bodyReader, nil, &result)

	// Assert
	assert.NoError(t, err)
	mockFetcher.AssertExpectations(t)
}

func TestFetchJSON_TypedNilBody_Fix(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	resp := mocks.NewMockResponse(`{}`, 200)
	resp.Header.Set("Content-Type", "application/json")

	// We expect the request NOT to have Content-Type header if body is effectively nil
	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		ct := req.Header.Get("Content-Type")
		return ct == ""
	})).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]interface{}

	// Pass a typed nil pointer
	var nilBody *bytes.Buffer = nil
	err := s.FetchJSON(context.Background(), "POST", "http://example.com", nilBody, nil, &result)

	assert.NoError(t, err)
	mockFetcher.AssertExpectations(t)
}

func TestFetchJSON_UseNumber_Fix(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// JSON with a number that would be float64
	jsonContent := `{"int_val": 123, "float_val": 123.456, "large_val": 9999999999.99}`
	resp := mocks.NewMockResponse(jsonContent, 200)
	resp.Header.Set("Content-Type", "application/json")

	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]interface{}

	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.NoError(t, err)

	// Verify types
	// Without UseNumber(), all numbers in interface{} are float64
	assert.IsType(t, float64(0), result["int_val"], "Expected float64 for int_val")
	assert.IsType(t, float64(0), result["float_val"], "Expected float64 for float_val")
	assert.IsType(t, float64(0), result["large_val"], "Expected float64 for large_val")
}

func TestFetchJSON_InvalidDecodeTarget(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	s := scraper.New(mockFetcher)
	ctx := context.Background()
	url := "http://example.com"

	t.Run("Nil Target", func(t *testing.T) {
		err := s.FetchJSON(ctx, "GET", url, nil, nil, nil)
		assert.ErrorIs(t, err, scraper.ErrDecodeTargetNil)
	})

	t.Run("Non-Pointer Target", func(t *testing.T) {
		var result map[string]any
		err := s.FetchJSON(ctx, "GET", url, nil, nil, result)
		assert.Error(t, err)
		assert.True(t, apperrors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "JSON 디코딩 실패: 결과를 저장할 변수(v)는 반드시 nil이 아닌 포인터여야 합니다")
	})

	t.Run("Nil Pointer Target", func(t *testing.T) {
		var result *map[string]any = nil
		err := s.FetchJSON(ctx, "GET", url, nil, nil, result)
		assert.Error(t, err)
		assert.True(t, apperrors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "JSON 디코딩 실패: 결과를 저장할 변수(v)는 반드시 nil이 아닌 포인터여야 합니다")
	})
}

// Slow infinite reader for timeout test
type slowInfiniteReader struct {
	delay time.Duration
}

func (r *slowInfiniteReader) Read(p []byte) (n int, err error) {
	time.Sleep(r.delay)
	if len(p) > 0 {
		p[0] = 'a'
		return 1, nil
	}
	return 0, nil
}

func TestFetchJSON_ContextCancellation_DuringBodyRead(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	slowBody := &slowInfiniteReader{delay: 20 * time.Millisecond}
	s := scraper.New(&mocks.MockFetcher{})

	var result map[string]interface{}
	start := time.Now()
	err := s.FetchJSON(ctx, "POST", "http://example.com", slowBody, nil, &result)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "context deadline exceeded") || strings.Contains(err.Error(), "context canceled"))
	assert.Less(t, duration, 200*time.Millisecond)
}

func TestFetchJSON_ResponseBodySizeLimit(t *testing.T) {
	mockFetcher := &mocks.MockFetcher{}
	// Valid JSON > 10MB
	prefix := `{"key": "`
	suffix := `"}`
	fillerSize := 11*1024*1024 - len(prefix) - len(suffix)
	jsonContent := prefix + strings.Repeat("a", fillerSize) + suffix

	resp := mocks.NewMockResponse(jsonContent, 200)
	mockFetcher.On("Do", mock.Anything).Return(resp, nil)

	s := scraper.New(mockFetcher)
	var result map[string]string
	err := s.FetchJSON(context.Background(), "GET", "http://example.com", nil, nil, &result)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "[JSON 파싱 불가]")
}
