package scraper

import (
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// JSON 응답 처리 에러 (JSON Response Handling)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@
// newErrJSONBodyTruncated JSON 파싱 중 응답 본문 크기가 설정된 제한을 초과하여 데이터가 잘린(Truncated) 경우 발생하는 에러를 생성합니다.
// JSON은 구조적 무결성이 필수적이므로, 일부 데이터가 누락되면 파싱을 진행할 수 없습니다.
func newErrJSONBodyTruncated(limit int64, url string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("[JSON 파싱 불가]: 응답 본문 크기가 설정된 제한(%d bytes)을 초과하여 데이터 무결성을 보장할 수 없습니다. (대상 URL: %s)", limit, url))
}

// @@@@@
// newErrJSONParseFailed 바이트 스트림을 JSON 구조체로 변환(Unmarshal)하는 도중 구문 오류가 발생했을 때 에러를 생성합니다.
// 에러 발생 위치(Offset)와 주변 문맥 데이터를 포함하여 문제 원인을 파악할 수 있도록 돕습니다.
func newErrJSONParseFailed(url string, offset int, snippet string, err error) error {
	m := fmt.Sprintf("불러온 페이지(%s) 데이터의 JSON 변환이 실패하였습니다.", url)
	if len(snippet) > 0 {
		m += fmt.Sprintf(" (구문 오류 발생 - 위치: %d, 주변 문맥: ...%s...)", offset, snippet)
	}

	return apperrors.Wrap(err, apperrors.ParsingFailed, m)
}

// @@@@@
// newErrJSONUnexpectedToken 유효한 JSON 객체 뒤에 불필요한 잔여 데이터(Extra Data)가 감지되었을 때 에러를 생성합니다.
// 이는 서버 응답에 디버그 메시지가 섞여 있거나 응답 데이터가 오염되었음을 의미합니다.
func newErrJSONUnexpectedToken(url string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("불러온 페이지(%s) 데이터에 불필요한 데이터가 포함되어 있습니다. (Unexpected Token)", url))
}

// newErrUnexpectedHTMLResponse JSON 응답을 기대했으나 서버에서 HTML 에러 페이지 등이 반환되었을 때 에러를 생성합니다.
// 주로 API 엔드포인트 오타, 인증 만료로 인한 로그인 페이지 리다이렉트 발생 시 활용됩니다.
func newErrUnexpectedHTMLResponse(url, contentType string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("유효하지 않은 응답 형식: JSON을 기대했으나 HTML 응답이 수신되었습니다. API 엔드포인트 또는 인증 상태를 점검하십시오 (대상 URL: %s, Content-Type: %s)", url, contentType))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HTML 파싱 및 구조 에러 (HTML Parsing & Structure)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrHTMLParseFailed 수신된 데이터 스트림을 HTML DOM 트리로 변환(Parsing)하는 도중 오류가 발생했을 때 에러를 생성합니다.
func newErrHTMLParseFailed(url string, err error) error {
	return apperrors.Wrap(err, apperrors.ParsingFailed, fmt.Sprintf("HTML 파싱 실패: 불러온 페이지(%s)를 처리하는 도중 오류가 발생하였습니다", url))
}

// @@@@@
// NewErrHTMLStructureChanged 대상 페이지의 HTML 구조가 변경되어 예상했던 요소나 데이터를 추출할 수 없을 때 발생하는 에러를 생성합니다.
func NewErrHTMLStructureChanged(url, message string) error {
	if url != "" {
		return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. (URL: %s) %s", url, message))
	}
	return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조가 변경되었습니다. %s", message))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HTTP 요청 및 네트워크 에러 (HTTP Request & Network)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@
// newErrHTTPRequestFailed HTTP 응답 상태 코드가 비정상(4xx, 5xx 등)일 때 에러를 생성하며, 코드의 성격에 따라 재시도 가능 여부(ErrorType)를 자동으로 분류합니다.
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

// newErrCreateHTTPRequest URL 형식이 잘못되었거나 컨텍스트 설정 문제 등으로 HTTP 요청 객체를 초기화하지 못했을 때 발생하는 에러를 생성합니다.
func newErrCreateHTTPRequest(url string, err error) error {
	return apperrors.Wrap(err, apperrors.ExecutionFailed, fmt.Sprintf("HTTP 요청 생성 실패: 요청을 초기화하는 도중 오류가 발생했습니다 (대상 URL: %s)", url))
}

// newErrNetworkError DNS 조회 실패, 커넥션 타임아웃, 원격 서버 거부 등 하위 단계의 네트워크 통신 오류가 발생했을 때 에러를 생성합니다.
func newErrNetworkError(url string, err error) error {
	return apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("네트워크 오류: 페이지(%s)에 접속할 수 없습니다. 서버 상태나 네트워크 연결을 확인해 주세요", url))
}

// newErrHTTPRequestCanceled 네트워크 요청 수행 도중 호출자에 의해 컨텍스트가 종료되거나 설정된 타임아웃에 도달하여 작업이 중단된 경우의 에러를 생성합니다.
func newErrHTTPRequestCanceled(url string, err error) error {
	return apperrors.Wrap(err, apperrors.Unavailable, fmt.Sprintf("요청 중단: 작업 시간이 초과되었거나 사용자에 의해 취소되었습니다 (대상 URL: %s)", url))
}

// ErrContextCanceled 스크래핑 프로세스 전반에서 컨텍스트 취소나 타임아웃이 감지되었을 때 사용하는 공통 에러 변수입니다.
var ErrContextCanceled = apperrors.New(apperrors.ExecutionFailed, "작업 중단: 실행 중인 요청이 취소되었거나 타임아웃이 발생했습니다")

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 데이터 스트림 및 크기 제한 에러 (Body Processing & Limits)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrReadRequestBody HTTP 요청 본문을 io.Reader에서 읽어들이는 과정에서 I/O 오류가 발생했을 때 에러를 생성합니다.
func newErrReadRequestBody(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "요청 본문 처리 실패: 데이터 스트림을 읽는 중 알 수 없는 입출력 오류가 발생했습니다")
}

// newErrEncodeJSONBody 구조체 데이터를 HTTP 요청을 위한 JSON 바이트 스트림으로 인코딩(Marshal)하는 도중 오류가 발생했을 때 에러를 생성합니다.
func newErrEncodeJSONBody(err error) error {
	return apperrors.Wrap(err, apperrors.Internal, "데이터 변환 실패: 요청 본문을 JSON 형식으로 인코딩할 수 없습니다. 데이터 구조를 확인해 주세요")
}

// newErrRequestBodyTooLarge 전송하려는 요청 본문의 크기가 미리 설정된 최대 허용 범위를 초과했을 때 발생하는 보안 및 성능 관련 에러를 생성합니다.
func newErrRequestBodyTooLarge(maxSize int64) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("요청 본문 크기 초과: 전송하려는 데이터가 허용된 제한(%d 바이트)을 초과하여 처리를 진행할 수 없습니다", maxSize))
}

// newErrResponseBodyTooLarge 서버로부터 수신된 응답 데이터가 허용된 최대 크기를 초과하여 파싱을 중단해야 할 때 발생하는 에러를 생성합니다. (DoS 공격 방지 및 메모리 보호)
func newErrResponseBodyTooLarge(limit int64, url string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("HTML 처리 중단: 응답 본문의 크기가 허용된 제한(%d 바이트)을 초과하여 데이터 무결성을 보장할 수 없습니다 (대상 URL: %s)", limit, url))
}

// newErrReadResponseBody 네트워크로부터 전달된 응답 바이트 스트림을 메모리 버퍼로 읽어들이는 과정에서 I/O 오류가 발생했을 때 에러를 생성합니다.
func newErrReadResponseBody(err error) error {
	return apperrors.Wrap(err, apperrors.ExecutionFailed, "응답 본문 데이터 수신 실패: 데이터 스트림을 읽는 중 I/O 오류가 발생했습니다")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 입력값 및 파라미터 검증 에러 (Internal Validation)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ErrDecodeTargetNil JSON 파싱 결과를 담을 변수(Interface)가 nil일 때 발생하는 내부 로직 에러입니다.
var ErrDecodeTargetNil = apperrors.New(apperrors.Internal, "JSON 디코딩 실패: 결과를 저장할 변수(v)가 nil입니다. 유효한 포인터를 전달해 주세요")

// newErrDecodeTargetInvalidType JSON 데이터를 역직렬화할 대상 변수가 포인터 타입이 아니거나 nil일 때 발생하는 에러를 생성합니다.
func newErrDecodeTargetInvalidType(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("JSON 디코딩 실패: 결과를 저장할 변수(v)는 반드시 nil이 아닌 포인터여야 합니다 (입력된 타입: %T)", v))
}

// ErrInputReaderNil HTML 파싱을 위해 제공된 데이터 소스(io.Reader)가 존재하지 않을 때 발생하는 에러입니다.
var ErrInputReaderNil = apperrors.New(apperrors.Internal, "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 nil입니다. 유효한 io.Reader를 전달해 주세요")

// ErrInputReaderInvalidType 제공된 io.Reader가 유효한 구현체를 참조하고 있지 않을 때(Typed Nil 등) 발생하는 에러입니다.
var ErrInputReaderInvalidType = apperrors.New(apperrors.Internal, "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 유효하지 않은 타입(Typed Nil)입니다. nil이 아닌 구체적인 io.Reader 구현체를 전달해 주세요")

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 응답 내용 검증 에러 (Content Validation)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@
// newErrValidationFailed 수신된 HTTP 응답에 대해 사용자가 정의한 추가 검증 조건(Validator)이 충족되지 않았을 때 에러를 생성합니다.
// 디버깅을 위해 응답 본문의 일부(Preview/Snippet)를 에러 메시지에 포함합니다.
func newErrValidationFailed(preview string, cause error) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, fmt.Sprintf("응답 검증 실패. Body Snippet: %s", preview))
}
