package scraper

import (
	"fmt"
	"net/http"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// JSON 응답 처리 에러 (JSON Response Handling)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@ 주석만 삭제하고 다시
// newErrJSONBodySizeLimitExceeded JSON 응답 본문의 크기가 설정된 제한을 초과하여 파싱을 사전에 차단할 때 발생하는 에러를 생성합니다.
//
// 보안 및 무결성 정책:
//   - DoS 공격 방어: 악의적인 대용량 응답으로 인한 메모리 고갈을 사전에 차단하는 핵심 보안 메커니즘입니다.
//   - 데이터 무결성: JSON은 구조적 완결성이 필수이므로, 크기 제한으로 잘린 불완전한 데이터는 파싱을 시도하지 않고 즉시 거부합니다.
func newErrJSONBodySizeLimitExceeded(url string, limit int64) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("JSON 파싱 중단: 응답 본문 크기가 허용된 제한(%d 바이트)을 초과하여 데이터 무결성을 보장할 수 없습니다 (URL: %s)", limit, url))
}

// @@@@@ 주석만 삭제하고 다시
// newErrJSONParseFailed JSON 바이트 스트림을 Go 구조체로 역직렬화(Unmarshal)하는 과정에서 구문 오류가 발생했을 때 에러를 생성합니다.
//
// 디버깅 지원:
//   - 에러 발생 위치(Offset)와 주변 문맥(Snippet)을 포함하여, API 응답 형식 변경이나 문자 인코딩 문제를 즉시 파악할 수 있습니다.
//   - 서버 응답에 예상치 못한 문자가 포함되었는지, 또는 Go 구조체 정의가 실제 JSON 스키마와 불일치하는지 신속하게 진단 가능합니다.
func newErrJSONParseFailed(cause error, url string, offset int, snippet string) error {
	m := fmt.Sprintf("JSON 파싱 실패: 구문 오류로 인해 응답 데이터를 변환할 수 없습니다 (URL: %s)", url)

	if len(snippet) > 0 {
		m += fmt.Sprintf(" - 오류 위치: %d, 주변 문맥: ...%s...", offset, snippet)
	}

	return apperrors.Wrap(cause, apperrors.ParsingFailed, m)
}

// @@@@@ 주석만 삭제하고 다시
// newErrJSONUnexpectedToken 유효한 JSON 객체 파싱 완료 후에도 데이터 스트림에 처리되지 않은 잔여 데이터(Extra Data)가 존재할 때 발생하는 에러를 생성합니다.
//
// Strict Mode 검증:
//   - 표준 JSON 디코더는 첫 번째 유효한 객체 파싱 후 작업을 멈추지만, 본 스크래퍼는 데이터 무결성 보장을 위해 스트림 끝(EOF)까지 완전히 비어있는지 검증합니다.
//   - 이는 오염된 응답 데이터로 인한 비즈니스 로직 오작동을 사전에 차단하는 핵심 안전장치입니다.
//
// 부분적으로 유효한 데이터라도 잔여물이 있으면 전체를 거부하여 하위 시스템의 안정성을 보장합니다.
func newErrJSONUnexpectedToken(url string) error {
	return apperrors.New(apperrors.ParsingFailed, fmt.Sprintf("JSON 파싱 실패: 응답 데이터에 유효한 JSON 이후 불필요한 토큰이 포함되어 있습니다 (URL: %s)", url))
}

// @@@@@ 주석만 삭제하고 다시
// newErrUnexpectedHTMLResponse JSON API 호출 시 서버가 HTML 응답(에러 페이지, 로그인 페이지 등)을 반환했을 때 발생하는 에러를 생성합니다.
//
// 주요 원인:
//   - API 엔드포인트 오류: URL 오타, 경로 변경, 잘못된 라우팅
//   - 인증 실패: 세션 만료로 인한 로그인 페이지 리다이렉트
//   - 서버 에러: 500/403 등의 상태 코드에 대한 HTML 에러 페이지 응답
//
// 이 에러 발생 시 API 엔드포인트 URL과 인증 토큰/쿠키 상태를 우선 점검해야 합니다.
func newErrUnexpectedHTMLResponse(url, contentType string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("응답 형식 오류: JSON 대신 HTML이 반환되었습니다 (URL: %s, Content-Type: %s)", url, contentType))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HTML 파싱 및 구조 에러 (HTML Parsing & Structure)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@ 주석만 삭제하고 다시
// newErrHTMLParseFailed HTML 바이트 스트림을 DOM 트리(goquery.Document)로 파싱하는 과정에서 치명적인 오류가 발생했을 때 에러를 생성합니다.
//
// 파싱 실패 원인:
//   - 마이너 문법 오류는 브라우저처럼 관대하게 처리되지만, 스트림 손상이나 메모리 할당 실패 등 심각한 문제 발생 시 이 에러가 반환됩니다.
//   - 데이터 원본의 무결성이 훼손되었음을 의미하며, 이후 CSS 셀렉터 기반 데이터 추출이 불가능합니다.
func newErrHTMLParseFailed(cause error, url string) error {
	return apperrors.Wrap(cause, apperrors.ParsingFailed, fmt.Sprintf("HTML 파싱 실패: DOM 트리 생성 중 오류가 발생했습니다 (URL: %s)", url))
}

// @@@@@ 여기에 둬야 하는지는 검토
// @@@@@ 주석만 삭제하고 다시
// NewErrHTMLStructureChanged 대상 페이지의 HTML 구조 변경으로 인해 예상 요소를 찾을 수 없을 때 발생하는 '논리적 파싱 실패' 에러를 생성합니다.
//
// 모니터링 신호:
//   - 이 에러는 네트워크 장애가 아닌, 사이트 개편(UI/UX 변경, 프론트엔드 프레임워크 교체, 데이터 노출 정책 변화)을 감지하는 핵심 관측 지표입니다.
//   - 운영 환경에서 이 에러 발생 시 CSS 셀렉터 업데이트가 필요한 유지보수 골든 타임임을 의미합니다.
func NewErrHTMLStructureChanged(url, message string) error {
	if url != "" {
		return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조 변경: %s (URL: %s)", message, url))
	}
	return apperrors.New(apperrors.ExecutionFailed, fmt.Sprintf("HTML 구조 변경: %s", message))
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// HTTP 요청 및 네트워크 에러 (HTTP Request & Network)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@ 주석만 삭제하고 다시
// newErrHTTPRequestFailed HTTP 응답 상태 코드(4xx, 5xx)에 따라 적절한 에러 타입을 자동 분류하여 재시도 정책을 지원합니다.
//
// 재시도 정책:
//   - Unavailable (재시도 가능): 5xx 서버 에러, 408 Timeout, 429 Rate Limit 등 일시적 장애
//   - ExecutionFailed (재시도 불필요): 400 Bad Request, 404 Not Found 등 영구적 클라이언트 오류
//
// 이를 통해 복구 불가능한 요청에 대한 불필요한 재시도를 차단하여 시스템 자원을 효율적으로 관리합니다.
func newErrHTTPRequestFailed(cause error, url string, statusCode int, body string) error {
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
	msg := fmt.Sprintf("HTTP 요청 실패: %d %s (URL: %s)", statusCode, http.StatusText(statusCode), url)
	if body != "" {
		msg += fmt.Sprintf(" - 응답: %s", body)
	}

	return apperrors.Wrap(cause, errType, msg)
}

// @@@@@ 주석만 삭제하고 다시
// newErrCreateHTTPRequest HTTP 요청 객체 생성 실패 시 에러를 생성합니다. 네트워크 통신 전 초기화 단계에서 발생하는 오류입니다.
// 주요 원인: URL 형식 오류, 잘못된 HTTP 메서드, 컨텍스트 주입 실패 등
func newErrCreateHTTPRequest(cause error, url string) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, fmt.Sprintf("HTTP 요청 생성 실패: 요청 객체 초기화 중 오류 발생 (URL: %s)", url))
}

// @@@@@ 주석만 삭제하고 다시
// newErrNetworkError 네트워크 통신 장애(DNS 조회 실패, TCP 연결 타임아웃, 서버 거부 등) 발생 시 에러를 생성합니다.
// 일시적 네트워크 장애와 영구적 장애를 구분하기 위한 핵심 진단 지점입니다.
func newErrNetworkError(cause error, url string) error {
	return apperrors.Wrap(cause, apperrors.Unavailable, fmt.Sprintf("네트워크 오류: 연결 실패 (URL: %s)", url))
}

// @@@@@ 주석만 삭제하고 다시
// newErrHTTPRequestCanceled 컨텍스트 취소 또는 타임아웃으로 인해 HTTP 요청이 중단되었을 때 에러를 생성합니다.
// 불필요한 네트워크 I/O를 조기 종료하여 고루틴 누수를 방지하고 시스템 부하를 관리합니다.
func newErrHTTPRequestCanceled(cause error, url string) error {
	return apperrors.Wrap(cause, apperrors.Unavailable, fmt.Sprintf("요청 중단: 컨텍스트 취소 또는 타임아웃 (URL: %s)", url))
}

// ErrContextCanceled 스크래핑 프로세스에서 컨텍스트 취소 또는 타임아웃 발생 시 사용되는 공통 에러 인스턴스입니다.
var ErrContextCanceled = apperrors.New(apperrors.Unavailable, "작업 중단: 컨텍스트 취소 또는 타임아웃")

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 데이터 스트림 및 크기 제한 에러 (Body Processing & Limits)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// newErrPrepareRequestBody HTTP 요청 전송을 위해 본문 데이터(io.Reader)를 메모리로 읽어들이는 과정에서 I/O 오류가 발생했을 때 에러를 생성합니다.
//
// 발생 시점:
//   - prepareBody 함수에서 io.ReadAll 호출 시 실패한 경우
//   - 네트워크 전송 이전 단계로, 아직 HTTP 요청이 서버로 전송되지 않은 상태입니다.
//
// 주요 원인:
//   - 컨텍스트 취소/타임아웃: contextAwareReader가 감지한 컨텍스트 종료
//   - 스트림 읽기 실패: 제공된 io.Reader의 내부 오류 (파일 시스템, 네트워크 스트림 등)
//   - 메모리 부족: 대용량 데이터를 메모리로 읽는 중 할당 실패
func newErrPrepareRequestBody(cause error) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, "요청 본문 준비 실패: 데이터 스트림을 읽는 중 오류가 발생했습니다")
}

// newErrEncodeJSONBody HTTP 요청 본문으로 전송할 데이터를 JSON 형식으로 직렬화(json.Marshal)하는 과정에서 오류가 발생했을 때 에러를 생성합니다.
//
// 발생 시점:
//   - prepareBody 함수에서 임의의 타입(구조체, 맵 등)을 JSON으로 변환할 때 json.Marshal 실패
//   - 네트워크 전송 이전 단계로, 요청 본문 준비 과정에서 발생합니다.
//
// 주요 원인 (코드 레벨 버그):
//   - 순환 참조(Circular Reference): 구조체가 자기 자신을 참조하는 경우
//   - 직렬화 불가능한 타입: chan, func, unsafe.Pointer 등 JSON으로 변환할 수 없는 타입
//   - 잘못된 구조체 태그: json 태그 문법 오류
//
// 이 에러는 개발자가 제공한 데이터 구조의 문제를 의미하므로, 코드 수정이 필요합니다.
func newErrEncodeJSONBody(cause error) error {
	return apperrors.Wrap(cause, apperrors.Internal, "요청 본문 JSON 인코딩 실패: 데이터를 직렬화할 수 없습니다")
}

// newErrRequestBodyTooLarge HTTP 요청으로 전송하려는 본문 데이터의 크기가 설정된 최대 허용 범위를 초과했을 때 발생하는 에러를 생성합니다.
//
// 발생 시점:
//   - prepareBody 함수에서 요청 본문 크기를 검증할 때
//   - 네트워크 전송 이전 단계로, 데이터 준비 과정에서 사전 차단됩니다.
//
// 검증 단계:
//   - io.Reader 타입: Len() 메서드로 조기 검증 또는 io.ReadAll 후 최종 검증
//   - string/[]byte 타입: len() 함수로 직접 검증
//   - 기타 타입: JSON 직렬화 후 바이트 크기 검증
//
// 보안 및 성능 목적:
//   - DoS 공격 방지: 악의적인 대용량 요청으로부터 서버 자원을 보호
//   - 네트워크 효율: 불필요한 대역폭 낭비를 사전에 차단
func newErrRequestBodyTooLarge(maxSize int64) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("요청 본문 크기 초과: 전송 데이터가 허용 제한(%d 바이트)을 초과했습니다", maxSize))
}

// newErrResponseBodyTooLarge 서버로부터 수신된 응답 본문의 크기가 허용된 최대 크기를 초과하여 파싱을 중단해야 할 때 발생하는 에러를 생성합니다.
//
// 발생 시점:
//   - FetchHTML 함수에서 executeRequest 실행 후 result.IsTruncated가 true일 때
//   - executeRequest가 maxResponseBodySize를 초과하는 응답을 자동으로 잘라낸(Truncated) 경우
//
// 보안 및 안정성:
//   - DoS 공격 방지: 악의적인 대용량 응답으로부터 메모리를 보호
//   - 자원 관리: 시스템 전체의 안정성을 위해 개별 요청의 메모리 사용량을 제한
func newErrResponseBodyTooLarge(limit int64, url string) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("응답 본문 크기 초과: 수신 데이터가 허용 제한(%d 바이트)을 초과했습니다 (URL: %s)", limit, url))
}

// newErrReadResponseBody 서버로부터 수신된 HTTP 응답 본문을 메모리로 읽어들이는 과정에서 I/O 오류가 발생했을 때 에러를 생성합니다.
//
// 발생 시점:
//   - executeRequest 함수에서 readResponseBodyWithLimit 호출 시 io.ReadAll 실패
//   - HTTP 응답 헤더는 정상적으로 수신되었으나, 본문 데이터를 읽는 중 오류 발생
//
// 주요 원인:
//   - 네트워크 연결 중단: 응답 본문을 읽는 도중 서버와의 연결이 끊긴 경우
//   - 타임아웃: 응답 본문 읽기 중 네트워크 지연으로 인한 타임아웃
//   - 서버 측 연결 종료: 서버가 응답 본문 전송 중 연결을 강제로 종료한 경우
//
// 재시도 정책:
//   - 일시적 네트워크 장애일 가능성이 높으므로 재시도를 통해 복구 가능합니다.
func newErrReadResponseBody(cause error) error {
	return apperrors.Wrap(cause, apperrors.Unavailable, "응답 본문 데이터 수신 실패: 데이터 스트림을 읽는 중 I/O 오류가 발생했습니다")
}

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 입력값 및 파라미터 검증 에러 (Internal Validation)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// ErrDecodeTargetNil JSON 응답을 디코딩할 대상 변수가 nil일 때 반환되는 에러입니다.
//
// 발생 시점:
//   - FetchJSON 함수 호출 시 디코딩 대상 변수 v가 nil인 경우
//   - 네트워크 요청 전 초기 검증 단계에서 즉시 반환됩니다.
//
// 원인 (코드 레벨 버그):
//   - 호출자가 json.Unmarshal의 규칙을 위반하여 nil 포인터를 전달한 경우
//   - 예: scraper.FetchJSON(ctx, "GET", url, nil, nil, nil) // 마지막 인자가 nil
//
// 방어적 프로그래밍:
//   - json.Decoder.Decode(nil) 호출 시 발생하는 런타임 패닉을 사전에 차단합니다.
//   - 명확한 에러 메시지를 통해 개발자가 문제를 즉시 파악할 수 있도록 합니다.
var ErrDecodeTargetNil = apperrors.New(apperrors.Internal, "JSON 디코딩 실패: 결과를 저장할 변수가 nil입니다")

// newErrDecodeTargetInvalidType JSON 응답을 디코딩할 대상 변수의 타입이 유효하지 않을 때 발생하는 에러를 생성합니다.
//
// 발생 시점:
//   - FetchJSON 함수 호출 시 디코딩 대상 변수 v의 타입 검증 실패
//   - reflect.ValueOf(v).Kind() != reflect.Ptr 또는 rv.IsNil()이 true인 경우
//
// 검증 조건:
//   - 포인터가 아닌 타입: 예) var result MyStruct (포인터가 아님)
//   - Typed Nil 포인터: 예) var result *MyStruct; result는 nil
//
// 타입 안전성:
//   - json.Unmarshal은 반드시 nil이 아닌 포인터를 요구합니다.
//   - 사전 검증을 통해 json.Decoder.Decode 호출 시 발생할 수 있는 런타임 패닉을 방지합니다.
func newErrDecodeTargetInvalidType(v any) error {
	return apperrors.New(apperrors.Internal, fmt.Sprintf("JSON 디코딩 실패: 결과를 저장할 변수는 nil이 아닌 포인터여야 합니다 (전달된 타입: %T)", v))
}

// ErrInputReaderNil HTML 파싱을 위한 입력 데이터 스트림이 제공되지 않았을 때 반환되는 에러입니다.
//
// 발생 시점:
//   - ParseHTML 함수 호출 시 io.Reader 파라미터 r이 nil인 경우
//   - 입력 검증 단계에서 즉시 반환됩니다.
//
// 원인 (코드 레벨 버그):
//   - 호출자가 필수 파라미터를 제공하지 않은 경우
//
// API 계약:
//   - ParseHTML은 유효한 io.Reader를 필수로 요구합니다.
//   - nil 전달은 API 계약 위반이므로 명확한 에러로 통보합니다.
var ErrInputReaderNil = apperrors.New(apperrors.Internal, "HTML 파싱 실패: 입력 데이터 스트림이 nil입니다")

// @@@@@
// ErrInputReaderInvalidType 제공된 io.Reader가 유효한 구현체를 참조하고 있지 않을 때(Typed Nil 등) 발생하는 에러입니다.
// Go의 인터페이스 특성상 nil 인터페이스 값과 nil 구체 타입을 가진 인터페이스는 다르므로, 이를 명시적으로 검증하여 런타임 오류를 방지합니다.
var ErrInputReaderInvalidType = apperrors.New(apperrors.Internal, "파싱 초기화 실패: 입력 데이터 스트림(Reader)이 유효하지 않은 타입(Typed Nil)입니다. nil이 아닌 구체적인 io.Reader 구현체를 전달해 주세요")

// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
// 응답 내용 검증 에러 (Content Validation)
// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

// @@@@@
// newErrValidationFailed 수신된 HTTP 응답에 대해 비즈니스 로직 차원에서 정의한 커스텀 검증 조건(Validator)이 충족되지 않았을 때 발생하는 '도메인 검증 에러'를 생성합니다.
//
// 전략적 가치:
//   - 단순한 프로토콜(HTTP, JSON) 수준의 오류를 넘어, 응답 본문에 포함된 특정 키워드 존재 여부나 데이터 값의 유효성 등 도메인 특화된 규칙을 강제합니다.
//   - 에러 메시지에 응답 본문의 일부(Preview/Snippet)를 포함하여, 어떤 실질적인 데이터가 검증을 통과하지 못했는지 운영 환경에서 즉각적으로 식별하고 재현할 수 있도록 지원합니다.
//   - 이는 스크래핑 프로세스의 신뢰성을 최종적으로 보장하는 '최후의 방어선' 역할을 합니다.
func newErrValidationFailed(cause error, preview string) error {
	return apperrors.Wrap(cause, apperrors.ExecutionFailed, fmt.Sprintf("응답 검증 실패. Body Snippet: %s", preview))
}
