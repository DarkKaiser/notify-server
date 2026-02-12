package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestScraper_prepareBody는 prepareBody 메서드의 다양한 입력 타입과 에러 처리, 크기 제한을 검증합니다.
func TestScraper_prepareBody(t *testing.T) {
	tests := []struct {
		name string
		// Input
		body       any
		scraperOpt []Option

		// Verification
		wantContent string // Expected content if successful
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
	}{
		// -------------------------------------------------------------------------
		// [Category 1: Supported Types]
		// -------------------------------------------------------------------------
		{
			name:        "Success - Nil Body",
			body:        nil,
			wantContent: "", // Should return nil reader, handled as empty check
		},
		{
			name:        "Success - String Body",
			body:        "test string",
			wantContent: "test string",
		},
		{
			name:        "Success - Byte Slice Body",
			body:        []byte("test bytes"),
			wantContent: "test bytes",
		},
		{
			name:        "Success - io.Reader Body",
			body:        bytes.NewBufferString("test reader"),
			wantContent: "test reader",
		},
		{
			name:        "Success - Struct (JSON)",
			body:        struct{ Name string }{"json"},
			wantContent: `{"Name":"json"}`,
		},
		{
			name:        "Success - Map (JSON)",
			body:        map[string]int{"val": 1},
			wantContent: `{"val":1}`,
		},

		// -------------------------------------------------------------------------
		// [Category 2: Edge Cases]
		// -------------------------------------------------------------------------
		{
			name:        "Success - Empty String",
			body:        "",
			wantContent: "",
		},
		{
			name:        "Success - Empty Byte Slice",
			body:        []byte{},
			wantContent: "",
		},
		{
			name:        "Success - Typed Nil (io.Reader)",
			body:        (*bytes.Buffer)(nil),
			wantContent: "", // Should be handled as nil
		},

		// -------------------------------------------------------------------------
		// [Category 3: Size Limits]
		// -------------------------------------------------------------------------
		{
			name: "Success - Body At Limit",
			body: "12345",
			scraperOpt: []Option{
				WithMaxRequestBodySize(5),
			},
			wantContent: "12345",
		},
		{
			name: "Error - String Body Over Limit",
			body: "123456",
			scraperOpt: []Option{
				WithMaxRequestBodySize(5),
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"요청 본문 크기 초과"},
		},
		{
			name: "Error - Reader Body Over Limit",
			body: bytes.NewBufferString("123456"),
			scraperOpt: []Option{
				WithMaxRequestBodySize(5),
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"요청 본문 크기 초과"},
		},
		{
			name: "Error - JSON Body Over Limit",
			body: map[string]string{"key": "value_too_long"}, // {"key":"value_too_long"} -> 22 bytes
			scraperOpt: []Option{
				WithMaxRequestBodySize(5),
			},
			wantErr:     true,
			errType:     apperrors.InvalidInput,
			errContains: []string{"요청 본문 크기 초과"},
		},

		// -------------------------------------------------------------------------
		// [Category 4: Errors]
		// -------------------------------------------------------------------------
		{
			name:        "Error - JSON Marshal Failure (Cyclic/Unsupported)",
			body:        map[string]any{"chan": make(chan int)}, // Channel cannot be marshaled
			wantErr:     true,
			errType:     apperrors.Internal,
			errContains: []string{"요청 본문 JSON 인코딩 실패"},
		},
		{
			name:        "Error - Reader Read Failure",
			body:        &failReader{},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: []string{"요청 본문 준비 실패"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// 1. Initialize Scraper
			mockFetcher := new(mocks.MockFetcher) // Not used here, but needed for New
			s := New(mockFetcher, tt.scraperOpt...)

			// Cast to concrete type to access private method
			impl := s.(*scraper)

			// 2. Execute
			reader, err := impl.prepareBody(context.Background(), tt.body)

			// 3. Verify
			if tt.wantErr {
				require.Error(t, err)
				if len(tt.errContains) > 0 {
					for _, msg := range tt.errContains {
						assert.Contains(t, err.Error(), msg)
					}
				}
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				if tt.wantContent == "" {
					// Either nil reader or empty reader
					if reader == nil {
						return
					}
					// If not nil, read it
					content, err := io.ReadAll(reader)
					assert.NoError(t, err)
					assert.Empty(t, content)
				} else {
					require.NotNil(t, reader)
					content, err := io.ReadAll(reader)
					assert.NoError(t, err)
					assert.Equal(t, tt.wantContent, string(content))
				}
			}
		})
	}
}

// TestScraper_createAndSendRequest는 HTTP 요청 생성 및 전송 로직, 헤더 처리, 에러 래핑을 검증합니다.
func TestScraper_createAndSendRequest(t *testing.T) {
	tests := []struct {
		name string
		// Input
		params requestParams

		// Mock
		mockSetup func(*mocks.MockFetcher)

		// Context
		ctxSetup func() (context.Context, context.CancelFunc)

		// Verification
		wantErr     bool
		errType     apperrors.ErrorType
		errContains []string
		checkResp   func(*testing.T, *http.Response)
	}{
		// -------------------------------------------------------------------------
		// [Category 1: Success & Header Handling]
		// -------------------------------------------------------------------------
		{
			name: "Success - Basic Request",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "GET", req.Method)
					assert.Equal(t, "http://example.com", req.URL.String())
				}).Return(&http.Response{StatusCode: 200}, nil)
			},
			checkResp: func(t *testing.T, resp *http.Response) {
				assert.Equal(t, 200, resp.StatusCode)
			},
		},
		{
			name: "Success - Custom Headers",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
				Header: http.Header{"X-Custom": []string{"value"}},
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "value", req.Header.Get("X-Custom"))
				}).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name: "Success - Default Accept Header (Applied)",
			params: requestParams{
				Method:        "GET",
				URL:           "http://example.com",
				DefaultAccept: "application/json",
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "application/json", req.Header.Get("Accept"))
				}).Return(&http.Response{StatusCode: 200}, nil)
			},
		},
		{
			name: "Success - Default Accept Header (Ignored if Exists)",
			params: requestParams{
				Method:        "GET",
				URL:           "http://example.com",
				Header:        http.Header{"Accept": []string{"text/xml"}},
				DefaultAccept: "application/json",
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Run(func(args mock.Arguments) {
					req := args.Get(0).(*http.Request)
					assert.Equal(t, "text/xml", req.Header.Get("Accept"))
				}).Return(&http.Response{StatusCode: 200}, nil)
			},
		},

		// -------------------------------------------------------------------------
		// [Category 2: Context Cancellation]
		// -------------------------------------------------------------------------
		{
			name: "Error - Context Canceled Before Request",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			ctxSetup: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, context.Canceled)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"요청 중단"},
		},
		{
			name: "Error - Context Deadline Exceeded",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			ctxSetup: func() (context.Context, context.CancelFunc) {
				// Immediately expired context
				ctx, cancel := context.WithTimeout(context.Background(), 0)
				return ctx, cancel
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, context.DeadlineExceeded)
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"요청 중단"},
		},

		// -------------------------------------------------------------------------
		// [Category 3: Fetcher Errors]
		// -------------------------------------------------------------------------
		{
			name: "Error - Network Error",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockSetup: func(m *mocks.MockFetcher) {
				m.On("Do", mock.Anything).Return(nil, errors.New("dial tcp: i/o timeout"))
			},
			wantErr:     true,
			errType:     apperrors.Unavailable,
			errContains: []string{"네트워크 오류"},
		},

		// -------------------------------------------------------------------------
		// [Category 4: Validations]
		// -------------------------------------------------------------------------
		{
			name: "Error - Invalid Method",
			params: requestParams{
				Method: "INVALID METHOD", // Space not allowed in method
				URL:    "http://example.com",
			},
			// Mock should not be called because NewRequest fails
			mockSetup:   func(m *mocks.MockFetcher) {},
			wantErr:     true,
			errType:     apperrors.ExecutionFailed,
			errContains: []string{"HTTP 요청 생성 실패"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			// 1. Setup Mock
			mockFetcher := new(mocks.MockFetcher)
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			// 2. Initialize Scraper
			s := New(mockFetcher)
			impl := s.(*scraper)

			// 3. Setup Context
			var ctx context.Context
			var cancel context.CancelFunc
			if tt.ctxSetup != nil {
				ctx, cancel = tt.ctxSetup()
			} else {
				ctx, cancel = context.WithCancel(context.Background())
			}
			defer cancel()

			// 4. Execute
			resp, err := impl.createAndSendRequest(ctx, tt.params)

			// 5. Verify
			if tt.wantErr {
				require.Error(t, err)
				if len(tt.errContains) > 0 {
					for _, msg := range tt.errContains {
						assert.Contains(t, err.Error(), msg)
					}
				}
				if tt.errType != apperrors.Unknown {
					assert.True(t, apperrors.Is(err, tt.errType), "Expected error type %s, got %v", tt.errType, err)
				}
			} else {
				require.NoError(t, err)
				if tt.checkResp != nil {
					tt.checkResp(t, resp)
				}
			}
		})
	}
}

func init() {
	applog.SetLevel(applog.DebugLevel)
}
