package fetcher

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// HTTP 응답 검증 관련 에러

// newErrHTTPStatus HTTP 상태 코드 검증이 실패하였을 경우 반환하는 에러를 생성합니다.
func newErrHTTPStatus(errType apperrors.ErrorType, status, urlStr string) error {
	return apperrors.New(errType, fmt.Sprintf("HTTP 요청 처리 실패 (Status: %s, URL: %s)", status, urlStr))
}

// MimeTypeFetcher 관련 에러

// ErrMissingResponseContentType Content-Type 헤더가 누락된 경우 반환하는 에러입니다.
var ErrMissingResponseContentType = apperrors.New(apperrors.InvalidInput, "Content-Type 헤더가 누락되어 요청을 처리할 수 없습니다")

// newErrUnsupportedMediaType 지원하지 않는 미디어 타입인 경우 반환하는 에러를 생성합니다.
func newErrUnsupportedMediaType(mediaType string, allowedTypes []string) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("지원하지 않는 미디어 타입입니다: %s (허용된 타입: %v)", mediaType, allowedTypes))
}

// MaxBytesFetcher 관련 에러

// newErrResponseBodyTooLarge 응답 본문 크기가 제한을 초과한 경우 반환하는 에러를 생성합니다.
func newErrResponseBodyTooLarge(limit int64) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("응답 본문 크기가 설정된 제한을 초과했습니다 (제한값: %d 바이트)", limit))
}

// newErrResponseBodyTooLargeByContentLength Content-Length 헤더에 명시된 응답 본문 크기가 제한을 초과한 경우 반환하는 에러를 생성합니다.
func newErrResponseBodyTooLargeByContentLength(contentLength, limit int64) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("Content-Length 헤더에 명시된 응답 본문 크기가 설정된 제한을 초과했습니다 (값: %d 바이트, 제한값: %d 바이트)", contentLength, limit))
}
