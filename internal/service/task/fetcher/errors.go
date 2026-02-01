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

// RetryFetcher 관련 에러

// ErrMissingGetBody 재시도 시 요청 본문을 재생성할 GetBody 함수가 없는 경우 반환하는 에러입니다.
var ErrMissingGetBody = apperrors.New(apperrors.InvalidInput, "HTTP 요청에 본문(Body)이 포함되어 있으나 재구성을 위한 GetBody 함수가 정의되지 않았습니다. 데이터 유실 방지를 위해 재시도를 수행할 수 없습니다")

// newErrGetBodyFailed GetBody 함수 실행 실패 시 반환하는 에러를 생성합니다.
func newErrGetBodyFailed(err error) error {
	return apperrors.Wrap(err, apperrors.InvalidInput, "GetBody 함수 실행 중 오류가 발생하여 재시도를 위한 요청 본문 재생성에 실패했습니다")
}

// ErrMaxRetriesExceeded 최대 재시도 횟수 초과 시 반환하는 에러입니다.
var ErrMaxRetriesExceeded = apperrors.New(apperrors.Unavailable, "최대 재시도 횟수를 초과하여 요청 수행에 실패했습니다")

// newErrMaxRetriesExceeded 최대 재시도 횟수 초과 시 반환하는 에러를 생성합니다.
// 기존 에러(cause)가 있는 경우 래핑하여 원인 에러를 보존하고, 없는 경우 기본 에러를 반환합니다.
func newErrMaxRetriesExceeded(cause error) error {
	if cause == nil {
		return ErrMaxRetriesExceeded
	}
	return apperrors.Wrap(cause, apperrors.Unavailable, ErrMaxRetriesExceeded.Error())
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
