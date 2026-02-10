package scraper

import (
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// @@@@@
// newErrJSONTruncated JSON 파싱 중 응답 본문 크기 초과로 인한 에러를 생성합니다.
func newErrJSONTruncated(limit int64, url string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("[JSON 파싱 불가]: 응답 본문 크기가 설정된 제한(%d bytes)을 초과하여 데이터 무결성을 보장할 수 없습니다. (대상 URL: %s)", limit, url))
}

// @@@@@
// newErrJSONParsingFailed JSON 파싱 실패 에러를 생성합니다.
func newErrJSONParsingFailed(message string, err error) error {
	return apperrors.Wrap(err, apperrors.ParsingFailed, message)
}

// @@@@@
// newErrJSONUnexpectedToken JSON Strict Parsing 실패 에러(불필요한 데이터 포함)를 생성합니다.
func newErrJSONUnexpectedToken(url string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("불러온 페이지(%s) 데이터에 불필요한 데이터가 포함되어 있습니다. (Unexpected Token)", url))
}

// newErrReadRequestBody 요청 본문을 읽어들이는 과정에서 I/O 오류가 발생했을 때 반환하는 에러를 생성합니다.
func newErrReadRequestBody(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "요청 본문 처리 실패: 데이터 스트림을 읽는 중 알 수 없는 입출력 오류가 발생했습니다")
}

// newErrEncodeJSONBody 요청 본문 데이터를 JSON 형식으로 변환(직렬화)하는 과정에서 오류가 발생했을 때 반환하는 에러를 생성합니다.
func newErrEncodeJSONBody(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "데이터 변환 실패: 요청 본문을 JSON 형식으로 인코딩할 수 없습니다. 데이터 구조를 확인해 주세요")
}

// newErrRequestBodyTooLarge 전송하려는 요청 본문의 크기가 설정된 제한을 초과하여 작업을 중단해야 할 때 반환하는 에러를 생성합니다.
func newErrRequestBodyTooLarge(maxSize int64) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("요청 본문 크기 초과: 전송하려는 데이터가 허용된 제한(%d 바이트)을 초과하여 처리를 진행할 수 없습니다", maxSize))
}

// newErrResponseBodyTooLarge 응답 본문의 크기가 설정된 제한을 초과하여 처리를 중단해야 할 때 반환하는 에러를 생성합니다.
func newErrResponseBodyTooLarge(limit int64, url string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("HTML 처리 중단: 응답 본문의 크기가 허용된 제한(%d 바이트)을 초과하여 데이터 무결성을 보장할 수 없습니다 (대상 URL: %s)", limit, url))
}

// newErrReadResponseBody 응답 본문 데이터를 읽는 과정에서 오류가 발생했을 때 반환하는 에러를 생성합니다.
func newErrReadResponseBody(err error) error {
	return apperrors.Wrap(err, apperrors.ExecutionFailed, "응답 본문 데이터 수신 실패: 데이터 스트림을 읽는 중 I/O 오류가 발생했습니다")
}

// @@@@@
// newErrHTTPRequestFailed HTTP 요청 실패 에러를 생성합니다.
// 상태 코드에 따라 적절한 에러 타입(Available/ExecutionFailed)을 결정합니다.
func newErrHTTPRequestFailed(url string, statusCode int, body string, cause error) error {
	// HTTP 상태 코드에 따라 에러 타입을 분류합니다:
	//
	// 4xx 클라이언트 에러:
	//   - 기본: ExecutionFailed (재시도 불필요)
	//   - 예외: 408 Request Timeout, 429 Too Many Requests
	//     → Unavailable (일시적일 수 있으므로 재시도 가능)
	//
	// 5xx 서버 에러:
	//   - Unavailable (서버 문제이므로 재시도 가능)
	//
	// 기타 (3xx, 1xx 등):
	//   - Unavailable (기본값)
	errType := apperrors.Unavailable
	if statusCode >= 400 && statusCode < 500 {
		if statusCode != http.StatusRequestTimeout && statusCode != http.StatusTooManyRequests {
			errType = apperrors.ExecutionFailed
		}
	}

	// 에러 메시지 생성
	msg := fmt.Sprintf("HTTP 요청 실패 (URL: %s, Status: %d)", url, statusCode)
	if body != "" {
		msg += fmt.Sprintf(", Body: %s", body)
	}

	return apperrors.Wrap(cause, errType, msg)
}

// @@@@@
// newErrValidationFailed 응답 유효성 검증 실패 에러를 생성합니다.
// Body Snippet을 포함하여 디버깅을 돕습니다.
func newErrValidationFailed(preview string, cause error) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, fmt.Sprintf("응답 검증 실패. Body Snippet: %s", preview))
}

// newErrHTMLParseFailed HTML 문서를 파싱하여 처리하는 과정에서 오류가 발생했을 때 반환하는 에러를 생성합니다.
func newErrHTMLParseFailed(url string, err error) error {
	return apperrors.Wrap(err, apperrors.ParsingFailed, fmt.Sprintf("HTML 파싱 실패: 불러온 페이지(%s)를 처리하는 도중 오류가 발생하였습니다", url))
}

// ErrDecodeTargetNil JSON 디코딩 결과를 저장할 변수로 nil이 전달되었을 때 반환하는 에러입니다.
var ErrDecodeTargetNil = apperrors.New(apperrors.Internal, "JSON 디코딩 실패: 결과를 저장할 변수(v)가 nil입니다. 유효한 포인터를 전달해 주세요")

// newErrDecodeTargetInvalidType JSON 디코딩 결과를 저장할 변수가 올바르지 않은 타입(nil 또는 비포인터)일 때 반환하는 에러를 생성합니다.
func newErrDecodeTargetInvalidType(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("JSON 디코딩 실패: 결과를 저장할 변수(v)는 반드시 nil이 아닌 포인터여야 합니다 (입력된 타입: %T)", v))
}

// ErrInputReaderNil HTML 파싱을 위해 전달된 io.Reader가 nil일 때 반환하는 에러입니다.
var ErrInputReaderNil = apperrors.New(apperrors.Internal, "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 nil입니다. 유효한 io.Reader를 전달해 주세요")

// ErrInputReaderInvalidType HTML 파싱을 위해 전달된 io.Reader가 유효하지 않은 타입(Typed Nil)일 때 반환하는 에러입니다.
var ErrInputReaderInvalidType = apperrors.New(apperrors.Internal, "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 유효하지 않은 타입(Typed Nil)입니다. nil이 아닌 구체적인 io.Reader 구현체를 전달해 주세요")

// ErrContextCanceled 작업 처리 중 컨텍스트가 취소되었을 때 반환하는 에러입니다.
// 타임아웃, 호출자의 명시적 취소, 또는 상위 컨텍스트의 종료 등으로 인해 작업이 중단된 상황을 나타냅니다.
var ErrContextCanceled = apperrors.New(apperrors.ExecutionFailed, "작업 중단: 실행 중인 요청이 취소되었거나 타임아웃이 발생했습니다")

// newErrUnexpectedHTMLResponse JSON 응답을 기대했으나 HTML 응답이 수신되었을 때 반환하는 에러를 생성합니다.
func newErrUnexpectedHTMLResponse(url, contentType string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("유효하지 않은 응답 형식: JSON을 기대했으나 HTML 응답이 수신되었습니다. API 엔드포인트 또는 인증 상태를 점검하십시오 (대상 URL: %s, Content-Type: %s)", url, contentType))
}

// newErrCreateHTTPRequest HTTP 요청 객체(http.Request)를 생성하는 과정에서 오류가 발생했을 때 반환하는 에러를 생성합니다.
func newErrCreateHTTPRequest(url string, err error) error {
	return apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("HTTP 요청 생성 실패: 요청을 초기화하는 도중 오류가 발생했습니다 (대상 URL: %s)", url))
}

// newErrHTTPRequestCanceled HTTP 요청이 컨텍스트 취소 또는 타임아웃으로 인해 중단되었을 때 반환하는 에러를 생성합니다.
func newErrHTTPRequestCanceled(url string, err error) error {
	return apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("요청 중단: 작업 시간이 초과되었거나 사용자에 의해 취소되었습니다 (대상 URL: %s)", url))
}

// newErrNetworkError HTTP 요청 전송 중 네트워크 오류나 클라이언트 설정 문제로 실패했을 때 반환하는 에러를 생성합니다.
func newErrNetworkError(url string, err error) error {
	return apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("네트워크 오류: 페이지(%s)에 접속할 수 없습니다. 서버 상태나 네트워크 연결을 확인해 주세요", url))
}

// @@@@@
// NewErrHTMLStructureChanged HTML 구조 변경으로 인한 파싱 실패 에러를 생성합니다.
func NewErrHTMLStructureChanged(url, message string) error {
	if url != "" {
		return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. (URL: %s) %s", url, message))
	}
	return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. %s", message))
}
