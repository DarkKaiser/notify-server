package contract

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/pkg/strutil"
)

// NotificationSender 알림 발송을 위한 핵심 인터페이스입니다.
// 클라이언트는 이 인터페이스를 통해 알림 발송을 요청하며, 실제 전송은 구현체가 담당합니다.
type NotificationSender interface {
	// Notify 알림 메시지 발송을 요청합니다.
	//
	// 이 메서드는 일반적으로 비동기적으로 동작할 수 있으며(구현체에 따라 다름),
	// 전송 요청이 성공적으로 큐에 적재되거나 시스템에 수락되었을 때 nil을 반환합니다.
	// 즉, nil 반환이 반드시 "최종 사용자 도달"을 보장하는 것은 아닙니다.
	//
	// 파라미터:
	//   - ctx: 요청의 컨텍스트 (Timeout, Cancellation 전파 용도)
	//   - notification: 전송할 알림의 상세 내용 (메시지, 수신처, 메타데이터 등)
	//
	// 반환값:
	//   - error: 요청 검증 실패, 큐 포화 상태, 또는 일시적 시스템 장애 시 에러를 반환합니다.
	Notify(ctx context.Context, notification Notification) error

	// SupportsHTML 지정된 Notifier가 HTML 형식의 메시지 본문을 지원하는지의 여부를 반환합니다.
	SupportsHTML(notifierID NotifierID) bool
}

// NotificationHealthChecker Notification 서비스의 상태를 확인하는 인터페이스입니다.
type NotificationHealthChecker interface {
	// Health 시스템이 정상적으로 동작 중인지 검사합니다.
	Health() error
}

// Notification 알림 전송을 위한 표준 데이터 구조(DTO)입니다.
//
// 단순한 메시지 전달을 넘어, 알림이 발생한 작업(Task)의 맥락(Context)과 상태 정보를 포함합니다.
// 이를 통해 수신 측에서는 "무엇을", "어디로", "왜" 보내는지 명확히 파악할 수 있으며,
// 시스템 전반에서 통일된 형식으로 알림을 처리할 수 있게 합니다.
type Notification struct {
	// NotifierID 알림을 전송할 대상 알림 채널(Notifier)의 식별자입니다. (Optional)
	// 이 값이 비어있다면(""), 애플리케이션 또는 시스템의 기본 알림 채널(Notifier)로 전송됩니다.
	NotifierID NotifierID

	// TaskID 이 알림을 생성한 작업(Task)의 종류를 나타내는 식별자입니다. (Optional)
	// 예: "NaverShopping", "KurlyCheck" 등 비즈니스 로직의 큰 카테고리를 의미합니다.
	TaskID TaskID

	// CommandID 해당 작업 내에서 실행된 구체적인 명령어 식별자입니다. (Optional)
	// 예: "CheckPrice", "MonitorStock" 등 작업 내부의 세부 동작을 구분합니다.
	CommandID TaskCommandID

	// InstanceID 작업이 실제로 실행될 때 부여되는 고유한 인스턴스 ID입니다. (Optional)
	// 스케줄러에 의해 주기적으로 반복 실행되는 작업의 경우, 각 회차를 구분하고 로그를 추적(Trace)하는 데 사용됩니다.
	InstanceID TaskInstanceID

	// Title 알림의 제목입니다. (Optional)
	Title string

	// Message 전달하고자 하는 알림의 본문 내용입니다. (Required)
	Message string

	// Elapsed 작업 실행에 소요된 시간입니다. (Optional)
	Elapsed time.Duration

	// ErrorOccurred 이 알림이 에러 상황을 알리는 것인지의 여부입니다.
	ErrorOccurred bool

	// Cancelable 해당 작업이 취소 가능한지의 여부입니다.
	Cancelable bool
}

// Validate 알림 데이터의 유효성을 검증합니다.
func (n *Notification) Validate() error {
	if strings.TrimSpace(n.Message) == "" {
		return ErrMessageRequired
	}
	return nil
}

// String 로그 기록이나 디버깅 시 알림 객체를 식별하기 위한 문자열 표현을 반환합니다.
func (n *Notification) String() string {
	fields := make([]string, 0, 6)

	if n.NotifierID != "" {
		fields = append(fields, fmt.Sprintf("notifier=%s", n.NotifierID))
	} else {
		fields = append(fields, "notifier=default")
	}

	if n.TaskID != "" {
		fields = append(fields, fmt.Sprintf("task=%s/%s", n.TaskID, n.CommandID))
	}

	if n.InstanceID != "" {
		fields = append(fields, fmt.Sprintf("instance=%s", n.InstanceID))
	}

	if n.ErrorOccurred {
		fields = append(fields, "error=true")
	}

	if n.Cancelable {
		fields = append(fields, "cancelable=true")
	}

	// 메시지가 길 경우 로그 가독성을 위해 앞부분만 잘라서 표시
	fields = append(fields, fmt.Sprintf("msg=%q", strutil.Truncate(n.Message, 47)))

	return fmt.Sprintf("Notification{%s}", strings.Join(fields, ", "))
}

// NewNotification 기본 알림 채널(Notifier)로 일반 메시지 전송을 위한 상세 알림 객체를 생성하여 반환하는 팩토리 함수입니다.
func NewNotification(message string) Notification {
	return Notification{
		Message:       message,
		ErrorOccurred: false,
	}
}

// NewErrorNotification 기본 알림 채널(Notifier)로 에러 메시지 전송을 위한 상세 알림 객체를 생성하여 반환하는 팩토리 함수입니다.
func NewErrorNotification(message string) Notification {
	return Notification{
		Message:       message,
		ErrorOccurred: true,
	}
}
