package scraper

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestScraper_executeRequest_Success(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{
		fetcher:             mockFetcher,
		maxRequestBodySize:  1024,
		maxResponseBodySize: 1024,
	}

	ctx := context.Background()
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("test body")),
		Header:     make(http.Header),
	}

	mockFetcher.On("Do", mock.Anything).Return(mockResponse, nil)

	result, logger, err := s.executeRequest(ctx, params)

	require.NoError(t, err)
	require.NotNil(t, logger)
	assert.Equal(t, mockResponse, result.Response)
	assert.Equal(t, []byte("test body"), result.Body)
	assert.False(t, result.IsTruncated)

	// Verify duration field is present (indirectly via logger type check, tough to check field value without hook)
	// But we can verify execution flow was correct.
	mockFetcher.AssertExpectations(t)
}

func TestScraper_executeRequest_SendFailure(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{fetcher: mockFetcher}
	ctx := context.Background()
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	mockFetcher.On("Do", mock.Anything).Return(nil, errors.New("network error"))

	result, _, err := s.executeRequest(ctx, params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "네트워크 오류") // newErrNetworkError
	assert.Nil(t, result.Response)
}

func TestScraper_executeRequest_ValidationFailure(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{fetcher: mockFetcher}
	ctx := context.Background()
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	mockResponse := &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(bytes.NewBufferString("not found")),
		Header:     make(http.Header),
	}

	mockFetcher.On("Do", mock.Anything).Return(mockResponse, nil)

	result, _, err := s.executeRequest(ctx, params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 요청 실패") // Validation failed for 404
	assert.Nil(t, result.Response)                // Result is empty on error
}

func TestScraper_executeRequest_ReadBodyFailure(t *testing.T) {
	// This is hard to test with standard bytes.Buffer as it doesn't fail read.
	// We need a custom reader that fails, but Do returns http.Response which takes ReadCloser.
	// Let's create a failing reader.
}

type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read failed")
}
func (f *failReader) Close() error { return nil }

func TestScraper_executeRequest_ReadBodyFailure_RealImplementation(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{
		fetcher:             mockFetcher,
		maxResponseBodySize: 1024,
	}
	ctx := context.Background()
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       &failReader{},
		Header:     make(http.Header),
	}

	mockFetcher.On("Do", mock.Anything).Return(mockResponse, nil)

	result, _, err := s.executeRequest(ctx, params)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "응답 본문 데이터 수신 실패") // newErrReadResponseBody
	assert.Nil(t, result.Response)
}

func TestScraper_executeRequest_TruncatedBody(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{
		fetcher:             mockFetcher,
		maxRequestBodySize:  1024,
		maxResponseBodySize: 5, // Small limit
	}

	ctx := context.Background()
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
	}

	longBody := "1234567890"
	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(longBody)),
		Header:     make(http.Header),
	}

	mockFetcher.On("Do", mock.Anything).Return(mockResponse, nil)

	result, logger, err := s.executeRequest(ctx, params)

	require.NoError(t, err)
	assert.True(t, result.IsTruncated)
	assert.Equal(t, 5, len(result.Body))
	assert.Equal(t, "12345", string(result.Body))

	// Check log output would require capturing logs, which is complex with current logger setup.
	// Assuming logic coverage is sufficient.
	require.NotNil(t, logger)
}

func TestScraper_executeRequest_CustomValidator(t *testing.T) {
	mockFetcher := new(MockFetcher)
	s := &scraper{fetcher: mockFetcher}
	ctx := context.Background()

	validatorCalled := false
	params := requestParams{
		Method: "GET",
		URL:    "http://example.com",
		Validator: func(resp *http.Response, logger *applog.Entry) error {
			validatorCalled = true
			return nil
		},
	}

	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString("ok")),
		Header:     make(http.Header),
	}

	mockFetcher.On("Do", mock.Anything).Return(mockResponse, nil)

	_, _, err := s.executeRequest(ctx, params)
	require.NoError(t, err)
	assert.True(t, validatorCalled)
}
