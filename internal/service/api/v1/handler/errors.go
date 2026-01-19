package handler

import (
	"fmt"

	"github.com/darkkaiser/notify-server/internal/service/api/httputil"
)

// NewErrAppIDMismatch 요청 본문(Body)의 Application ID와 인증 정보(Header/Query)가 불일치할 때 발생하는 보안 에러를 생성합니다.
func NewErrAppIDMismatch(reqAppID, authAppID string) error {
	return httputil.NewBadRequestError(fmt.Sprintf("요청 본문의 application_id와 인증된 애플리케이션이 일치하지 않습니다 (요청: %s, 인증: %s)", reqAppID, authAppID))
}

// NewErrInvalidBody 요청 본문(Body)의 데이터 형식이 올바르지 않거나(예: 잘못된 JSON), 파싱에 실패했을 때 발생하는 에러를 생성합니다.
func NewErrInvalidBody() error {
	return httputil.NewBadRequestError("요청 본문을 파싱할 수 없습니다. JSON 형식을 확인해주세요")
}

// NewErrValidationFailed 요청 데이터의 필수 값 누락, 형식 위반 등 유효성 검증(Validation)에 실패했을 때 발생하는 에러를 생성합니다.
func NewErrValidationFailed(msg string) error {
	return httputil.NewBadRequestError(msg)
}

// NewErrServiceStopped 서버 종료(Graceful Shutdown) 등으로 인해 서비스가 잠시 중지되었을 때 발생하는 에러를 생성합니다.
func NewErrServiceStopped() error {
	return httputil.NewServiceUnavailableError("서비스가 점검 중이거나 종료되었습니다. 관리자에게 문의해 주세요")
}

// NewErrServiceOverloaded 요청 대기열(Queue)이 가득 찼거나, 시스템 부하가 심해 요청을 처리할 수 없을 때 발생하는 에러를 생성합니다.
func NewErrServiceOverloaded() error {
	return httputil.NewServiceUnavailableError("일시적인 과부하로 알림을 처리할 수 없습니다. 잠시 후 다시 시도해주세요")
}

// NewErrServiceInterrupted 요청 처리 중 예기치 않은 시스템 오류나 인터럽트(Context Cancelled)가 발생했을 때 발생하는 에러를 생성합니다.
func NewErrServiceInterrupted() error {
	return httputil.NewInternalServerError("알림 서비스를 일시적으로 사용할 수 없습니다. 잠시 후 다시 시도해주세요")
}

// NewErrNotifierNotFound 지정된 알림 채널(Notifier)을 찾을 수 없거나, 존재하지 않을 때 발생하는 에러를 생성합니다.
func NewErrNotifierNotFound() error {
	return httputil.NewNotFoundError("등록되지 않은 알림 채널입니다. 설정을 확인해 주세요")
}
