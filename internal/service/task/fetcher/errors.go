package fetcher

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// MimeTypeFetcher 관련 에러

// ErrMissingResponseContentType Content-Type 헤더가 누락된 경우 반환하는 에러입니다.
var ErrMissingResponseContentType = apperrors.New(apperrors.InvalidInput, "Content-Type 헤더가 누락되어 요청을 처리할 수 없습니다")

// NewErrUnsupportedMediaType 지원하지 않는 미디어 타입인 경우 반환하는 에러를 생성합니다.
func NewErrUnsupportedMediaType(mediaType string, allowedTypes []string) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("지원하지 않는 미디어 타입입니다: %s (허용된 타입: %v)", mediaType, allowedTypes))
}

// MaxBytesFetcher 관련 에러

// NewErrResponseBodyTooLarge 응답 본문 크기가 제한을 초과한 경우 반환하는 에러를 생성합니다.
func NewErrResponseBodyTooLarge(limit int64) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("응답 본문 크기가 설정된 제한을 초과했습니다 (제한값: %d 바이트)", limit))
}

// NewErrResponseBodyTooLargeByContentLength Content-Length 헤더에 명시된 응답 본문 크기가 제한을 초과한 경우 반환하는 에러를 생성합니다.
func NewErrResponseBodyTooLargeByContentLength(contentLength, limit int64) error {
	return apperrors.New(apperrors.InvalidInput,
		fmt.Sprintf("Content-Length 헤더에 명시된 응답 본문 크기가 설정된 제한을 초과했습니다 (값: %d 바이트, 제한값: %d 바이트)", contentLength, limit))
}
