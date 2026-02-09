package scraper

import (
	"errors"
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// @@@@@
	// ErrNetwork 네트워크 통신 중 오류가 발생했습니다.
	ErrNetwork = errors.New("network error")

	// @@@@@
	// ErrValidation 응답 유효성 검증(Content-Type, Status Code 등)에 실패했습니다.
	ErrValidation = errors.New("validation failed")

	// @@@@@
	// ErrTooLarge 응답 본문이 허용된 크기를 초과했습니다.
	ErrTooLarge = errors.New("response body too large")

	// ErrDecodeTargetNil JSON 디코딩 결과를 저장할 변수로 nil이 전달되었을 때 반환하는 에러입니다.
	ErrDecodeTargetNil = apperrors.New(apperrors.Internal, "JSON 디코딩 실패: 결과를 저장할 대상(v)이 nil입니다. 유효한 포인터를 전달해 주세요.")
)

// newErrResponseBodyTooLarge 응답 본문의 크기가 설정된 제한을 초과하여 처리를 중단해야 할 때 반환하는 에러를 생성합니다.
func newErrResponseBodyTooLarge(limit int64, url string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("HTML 처리 중단: 응답 본문의 크기가 허용된 제한(%d 바이트)을 초과하여 데이터 무결성을 보장할 수 없습니다. (대상 URL: %s)", limit, url))
}

// newErrHTMLParseFailed HTML 문서를 파싱하는 과정에서 구조 분석에 실패했을 때 반환하는 에러를 생성합니다.
func newErrHTMLParseFailed(url string, err error) error {
	return apperrors.Wrap(err, apperrors.ParsingFailed, fmt.Sprintf("HTML 파싱 실패: 대상 페이지(%s)의 구조를 분석하는 과정에서 오류가 발생했습니다", url))
}

// newErrDecodeTargetInvalidType JSON 디코딩 결과를 저장할 변수가 올바르지 않은 타입(nil 또는 비포인터)일 때 반환하는 에러를 생성합니다.
func newErrDecodeTargetInvalidType(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("JSON 디코딩 실패: 결과를 저장할 대상(v)은 반드시 nil이 아닌 포인터여야 합니다. (입력된 타입: %T)", v))
}

// @@@@@
// ResponseError HTTP 응답 에러를 구조화하여 상세 정보를 제공합니다.
// HTTP 상태 코드, 헤더, 응답 본문 등을 포함하여 호출자가 에러 상황을 더 정확히 파악할 수 있게 합니다.
type ResponseError struct {
	StatusCode int
	Header     http.Header
	Body       []byte
	URL        string
	Err        error
}

// @@@@@
func (e *ResponseError) Error() string {
	msg := fmt.Sprintf("HTTP 요청 실패 (URL: %s, Status: %d)", e.URL, e.StatusCode)
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if len(e.Body) > 0 {
		// Body 내용이 있으면 메시지에 포함 (기존 에러 메시지 호환성 및 디버깅 용도)
		// Body는 이미 validateResponse에서 길이 제한(4KB)되어 저장됨
		msg += fmt.Sprintf(", Body: %s", string(e.Body))
	}
	return msg
}

// @@@@@
func (e *ResponseError) Unwrap() error {
	return e.Err
}

// @@@@@
// Is ResponseError가 특정 에러(ErrValidation 등)와 매칭되는지 확인합니다.
func (e *ResponseError) Is(target error) bool {
	// validateResponse에서 반환되는 에러는 기본적으로 ErrValidation의 성격을 가집니다.
	// 따라서 기존 테스트 코드(errors.Is(err, scraper.ErrValidation))와의 호환성을 위해 true를 반환합니다.
	return target == ErrValidation
}

// @@@@@
// NewErrHTMLStructureChanged HTML 구조 변경으로 인한 파싱 실패 에러를 생성합니다.
func NewErrHTMLStructureChanged(url, message string) error {
	if url != "" {
		return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. (URL: %s) %s", url, message))
	}
	return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. %s", message))
}
