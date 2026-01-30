package fetcher

import (
	"errors"
	"io"
	"net/http"
)

const (
	// defaultMaxBytes 응답 본문의 기본 크기 제한값입니다 (10MB).
	defaultMaxBytes = 10 * 1024 * 1024

	// NoLimit 응답 본문에 대한 크기 제한을 적용하지 않음을 나타내는 특수 상수입니다.
	NoLimit = -1
)

// maxBytesReader http.MaxBytesReader를 래핑하여 apperrors 형식의 에러 메시지를 제공하는 내부 헬퍼 구조체입니다.
type maxBytesReader struct {
	rc io.ReadCloser

	// 바이트 수 제한값 (에러 메시지에 포함하기 위해 저장)
	limit int64
}

// Read 데이터를 읽으며, 크기 제한 초과 시 apperrors로 변환합니다.
func (r *maxBytesReader) Read(p []byte) (n int, err error) {
	n, err = r.rc.Read(p)
	if err != nil {
		// http.MaxBytesReader는 제한 초과 시 *http.MaxBytesError를 반환합니다.
		// 문자열 비교 대신 타입 검사를 사용하여 더 견고하게 처리합니다.
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return n, NewErrResponseBodyTooLarge(r.limit)
		}
	}

	return n, err
}

// Close 래핑된 ReadCloser를 닫습니다.
func (r *maxBytesReader) Close() error {
	return r.rc.Close()
}

// MaxBytesFetcher HTTP 응답 본문의 크기를 제한하는 미들웨어입니다.
//
// 주요 기능:
//   - Content-Length 헤더 기반 조기 차단 (네트워크 대역폭 절약)
//   - 실제 읽기 시점의 바이트 수 제한 (악의적인 Content-Length 조작 방어)
//   - OOM(Out Of Memory) 방지
type MaxBytesFetcher struct {
	delegate Fetcher

	// 응답 본문의 최대 허용 바이트 수
	limit int64
}

// NewMaxBytesFetcher 새로운 MaxBytesFetcher 인스턴스를 생성합니다.
func NewMaxBytesFetcher(delegate Fetcher, limit int64) Fetcher {
	if limit == NoLimit {
		return delegate
	}
	if limit <= 0 {
		limit = defaultMaxBytes
	}

	return &MaxBytesFetcher{
		delegate: delegate,
		limit:    limit,
	}
}

// Do HTTP 요청을 수행하고, 응답 본문에 크기 제한을 적용합니다.
//
// 매개변수:
//   - req: 처리할 HTTP 요청
//
// 반환값:
//   - HTTP 응답 객체 (성공 시)
//   - 에러 (요청 처리 중 발생한 에러)
//
// 주의사항:
//   - 반환된 응답의 Body는 반드시 호출자가 닫아야 합니다.
//   - Body를 읽는 도중 제한 초과 시 에러가 발생할 수 있습니다.
//   - Content-Length가 없는 응답도 2차 방어로 보호됩니다.
func (f *MaxBytesFetcher) Do(req *http.Request) (*http.Response, error) {
	resp, err := f.delegate.Do(req)
	if err != nil {
		// 에러가 발생했더라도 응답 객체가 있을 수 있음 (예: 상태 코드 에러, 리다이렉트 에러)
		if resp != nil {
			// 커넥션 재사용을 위해 응답 객체의 Body를 안전하게 비우고 닫음
			drainAndCloseBody(resp.Body)
		}

		return nil, err
	}

	// 1차 방어: Content-Length 헤더 기반 조기 차단
	// 장점: 실제 데이터를 다운로드하기 전에 차단하여 네트워크 대역폭 절약
	if resp.ContentLength > f.limit {
		if resp.Body != nil {
			// 커넥션 재사용을 위해 응답 객체의 Body를 안전하게 비우고 닫음
			drainAndCloseBody(resp.Body)
		}

		return nil, NewErrResponseBodyTooLargeByContentLength(resp.ContentLength, f.limit)
	}

	// 2차 방어: 실제 읽기 시점의 바이트 수 제한
	//
	// http.MaxBytesReader는 Content-Length 헤더를 신뢰하지 않고,
	// 실제 Read() 호출 시 읽은 바이트 수를 기준으로 제한합니다.
	// 따라서 다음과 같은 경우를 방어할 수 있습니다:
	//   - Content-Length 헤더가 없는 응답
	//   - Content-Length가 실제 크기보다 작게 조작된 악의적인 응답
	//
	// 주의사항:
	//   - MaxBytesReader는 제한 초과 시 에러를 반환하지만 Body를 자동으로 닫지 않습니다.
	//   - 호출자는 반드시 defer resp.Body.Close()를 사용해야 합니다.
	//   - 이는 일반적인 HTTP 응답 Body 처리 규칙과 동일합니다.
	resp.Body = &maxBytesReader{
		rc:    http.MaxBytesReader(nil, resp.Body, f.limit),
		limit: f.limit,
	}

	return resp, nil
}
