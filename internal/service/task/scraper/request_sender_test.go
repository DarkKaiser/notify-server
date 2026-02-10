package scraper

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockFetcher is a mock implementation of the Fetcher interface
type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockFetcher) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestScraper_prepareBody(t *testing.T) {
	s := &scraper{maxRequestBodySize: 1024}

	tests := []struct {
		name        string
		body        interface{}
		expected    string
		expectError bool
	}{
		{
			name:     "NilBody",
			body:     nil,
			expected: "",
		},
		{
			name:     "StringBody",
			body:     "test string",
			expected: "test string",
		},
		{
			name:     "BytesBody",
			body:     []byte("test bytes"),
			expected: "test bytes",
		},
		{
			name:     "ReaderBody",
			body:     strings.NewReader("test reader"),
			expected: "test reader",
		},
		{
			name:     "JSONBody",
			body:     map[string]string{"key": "value"},
			expected: `{"key":"value"}`,
		},
		{
			name:        "JSONError",
			body:        make(chan int), // Channels cannot be marshaled
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			reader, err := s.prepareBody(ctx, tt.body)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, reader)
			} else {
				require.NoError(t, err)
				if tt.expected == "" {
					assert.Nil(t, reader)
				} else {
					require.NotNil(t, reader)
					content, err := io.ReadAll(reader)
					require.NoError(t, err)
					assert.Equal(t, tt.expected, string(content))
				}
			}
		})
	}
}

func TestScraper_prepareBody_Limit(t *testing.T) {
	// Set limit to 5 bytes
	s := &scraper{maxRequestBodySize: 5}
	ctx := context.Background()

	// Create body with 6 bytes
	body := "123456"

	// prepareBody reads the whole body to check limit if it's string/bytes/json
	// Or if it's a Reader, it wraps it.
	// Wait, prepareBody implementation for string/bytes checks length immediately?
	// Let's check implementation.
	// Case string: return strings.NewReader(v), nil. It does NOT check limit there.
	// It checks limit when reading from the returned reader?
	// No, prepareBody returns an io.Reader. The check is done inside the reader?
	// Let's look at prepareBody implementation again.
	// Ah, line 78 in request_sender.go: limitReader := io.LimitReader(v, s.maxRequestBodySize+1)
	// Then it does io.ReadAll(limitReader).
	// So it DOES check limit inside prepareBody for io.Reader types.
	// For string/bytes, it falls through to default case (JSON marshaling) if not handled explicitly?
	// No, string is handled in switch v := body.(type).
	// Wait, string case is NOT handled explicitly in switch!
	// Only io.Reader, []byte, nil are handled.
	// String falls into default case -> JSON Marshal -> "123456" (JSON string).
	// Oh, wait. If I pass "123456" as body, it becomes "\"123456\"" in JSON?
	// Let's check request_sender.go code again.

	reader, err := s.prepareBody(ctx, body)

	// If body is string, it goes to default case: JSON Marshal.
	// JSON Marshal of "123456" is "\"123456\"" (8 bytes).
	// Then it goes to prepareBody recursive call with []byte.
	// []byte case: checks length. 8 > 5 returns error.

	require.Error(t, err)
	assert.Contains(t, err.Error(), "요청 본문 크기 초과")
	assert.Nil(t, reader)
}

func TestScraper_createAndSendRequest(t *testing.T) {
	tests := []struct {
		name          string
		params        requestParams
		mockResponse  *http.Response
		mockError     error
		expectedError string
	}{
		{
			name: "Success",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockResponse: &http.Response{StatusCode: 200},
		},
		{
			name: "RequestCreationError_InvalidURL",
			params: requestParams{
				Method: "GET",
				URL:    "://invalid-url", // This causes new request failure
			},
			expectedError: "HTTP 요청 생성 실패",
		},
		// ContextCanceled check is tricky because NewRequestWithContext doesn't return error on cancelled context immediately
		{
			name: "NetworkError",
			params: requestParams{
				Method: "GET",
				URL:    "http://example.com",
			},
			mockError:     errors.New("connection refused"),
			expectedError: "네트워크 오류",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := new(MockFetcher)
			s := &scraper{fetcher: mockFetcher}
			ctx := context.Background()

			if tt.expectedError == "" || tt.name == "NetworkError" {
				mockFetcher.On("Do", mock.AnythingOfType("*http.Request")).Return(tt.mockResponse, tt.mockError)
			}

			resp, err := s.createAndSendRequest(ctx, tt.params)

			if tt.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				assert.Equal(t, tt.mockResponse, resp)
			}
		})
	}
}

func TestScraper_createAndSendRequest_Headers(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{fetcher: mockFetcher}
	ctx := context.Background()

	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
		Header: http.Header{
			"X-Custom-Header": []string{"CustomValue"},
		},
		DefaultAccept: "application/json",
	}

	mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
		return req.Header.Get("X-Custom-Header") == "CustomValue" &&
			req.Header.Get("Accept") == "application/json"
	})).Return(&http.Response{StatusCode: 200}, nil)

	_, err := s.createAndSendRequest(ctx, params)
	require.NoError(t, err)
	mockFetcher.AssertExpectations(t)
}

func TestScraper_createAndSendRequest_ContextCanceled(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{fetcher: mockFetcher}

	// Create a context that is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	// When context is canceled, fetcher.Do might behave differently depending on implementation
	// But our wrapper createAndSendRequest checks ctx.Err() AFTER Do returns.
	// We simulate Do returning an error due to context cancel
	mockFetcher.On("Do", mock.Anything).Return(nil, context.Canceled)

	_, err := s.createAndSendRequest(ctx, params)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "요청 중단")
}
