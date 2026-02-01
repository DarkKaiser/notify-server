package fetcher

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockReadCloser는 테스트용 모의 ReadCloser입니다.
type mockReadCloser struct {
	io.Reader
	closed bool
	read   int64
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	n, err = m.Reader.Read(p)
	m.read += int64(n)
	return n, err
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

func TestDrainAndCloseBody(t *testing.T) {
	// 64KB보다 약간 더 큰 데이터 생성
	largeDataSize := maxDrainBytes + 1024
	largeData := make([]byte, largeDataSize)

	tests := []struct {
		name          string
		body          *mockReadCloser
		expectedRead  int64
		expectedClose bool
	}{
		{
			name:          "Nil Body",
			body:          nil,
			expectedRead:  0,
			expectedClose: false,
		},
		{
			name: "Small Body (< maxDrainBytes)",
			body: &mockReadCloser{
				Reader: bytes.NewReader([]byte("small data")),
			},
			expectedRead:  10, // len("small data")
			expectedClose: true,
		},
		{
			name: "Exact Boundary Body (= maxDrainBytes)",
			body: &mockReadCloser{
				Reader: bytes.NewReader(make([]byte, maxDrainBytes)),
			},
			expectedRead:  int64(maxDrainBytes),
			expectedClose: true,
		},
		{
			name: "Large Body (> maxDrainBytes)",
			body: &mockReadCloser{
				Reader: bytes.NewReader(largeData),
			},
			// maxDrainBytes까지만 읽어야 함
			expectedRead:  int64(maxDrainBytes),
			expectedClose: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mockReadCloser가 nil인 경우를 처리하기 위해 인터페이스로 캐스팅
			var rc io.ReadCloser
			if tt.body != nil {
				rc = tt.body
			}

			drainAndCloseBody(rc)

			if tt.body != nil {
				assert.Equal(t, tt.expectedClose, tt.body.closed, "Close() 호출 여부가 일치해야 합니다")
				assert.Equal(t, tt.expectedRead, tt.body.read, "읽은 바이트 수가 일치해야 합니다")
			}
		})
	}
}

func TestDrainAndCloseBody_ReaderError(t *testing.T) {
	// 읽기 도중 에러가 발생하는 경우 테스트
	// 에러가 발생하더라도 Close는 반드시 호출되어야 함
	expectedErr := errors.New("read error")
	mockBody := &mockReadCloser{
		Reader: &errorReader{err: expectedErr},
	}

	drainAndCloseBody(mockBody)

	assert.True(t, mockBody.closed, "에러가 발생해도 Body는 닫혀야 합니다")
}

// errorReader는 항상 에러를 반환하는 Reader입니다.
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}
