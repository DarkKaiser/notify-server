package task

import (
	"sync"
)

// Handler 개별 Task 인스턴스를 제어하고 상태를 조회하기 위한 인터페이스입니다.
//
// Handler는 Service 레이어와 개별 Task 구현체(Task 구조체) 사이의 계약(Contract)을 정의합니다.
// Service는 이 인터페이스를 통해 Task의 구체적인 구현을 알 필요 없이,
// 표준화된 방식으로 실행(Run), 취소(Cancel), 상태 확인 등을 수행할 수 있습니다.
type Handler interface {
	GetID() ID
	GetCommandID() CommandID
	GetInstanceID() InstanceID

	// GetNotifierID 알림을 발송할 대상 Notifier의 ID를 반환합니다.
	GetNotifierID() string

	// Cancel 작업을 취소 요청합니다.
	// 호출 즉시 IsCanceled()가 true를 반환해야 하며, 실행 중인 로직은 이를 감지하여 조기 종료해야 합니다.
	Cancel()

	// IsCanceled 작업이 취소되었는지 여부를 반환합니다.
	// Run 루프 내에서 주기적으로 확인하여, 취소 시 작업을 중단하는 용도로 사용됩니다.
	IsCanceled() bool

	// ElapsedTimeAfterRun 작업이 시작된 후 경과된 시간(초)을 반환합니다.
	// 작업 모니터링이나 타임아웃 감지에 활용될 수 있습니다.
	ElapsedTimeAfterRun() int64

	// SetStorage 작업 결과를 저장할 스토리지를 주입합니다.
	// 테스트 시 Mock 스토리지를 주입하거나, 런타임에 동적으로 스토리지를 변경할 때 사용됩니다.
	SetStorage(storage TaskResultStorage)

	// Run 작업을 실행하는 메인 메서드입니다.
	Run(taskCtx TaskContext, notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID)
}

// Submitter 작업을 제출하는 인터페이스입니다.
type Submitter interface {
	// SubmitTask 작업을 제출합니다. 제출 성공 여부(error)를 반환합니다.
	SubmitTask(req *SubmitRequest) error
}

// Canceler 실행 중인 작업을 취소하는 인터페이스입니다.
type Canceler interface {
	// CancelTask 특정 작업 인스턴스를 취소합니다. 취소 성공 여부(error)를 반환합니다.
	CancelTask(instanceID InstanceID) error
}

// Executor 작업을 실행하고 취소할 수 있는 Combined 인터페이스입니다.
type Executor interface {
	Submitter
	Canceler
}

// NotificationSender Task 실행 중 발생하는 다양한 이벤트(시작, 성공, 실패 등)를 외부로 알리기 위한 인터페이스입니다.
// Task 로직은 이 인터페이스를 통해 구체적인 알림 수단(Telegram, Email, Slack 등)의 구현 상세에 의존하지 않고
// 추상화된 방식으로 메시지를 전달합니다. 이를 통해 알림 채널의 유연한 교체와 확장이 가능해집니다.
type NotificationSender interface {
	// Notify 지정된 NotifierID를 통해 알림 메시지를 전송합니다.
	// Task의 실행 컨텍스트(TaskContext)를 함께 전달하여, 알림 수신자가 작업의 메타데이터(TaskID, Title, 실행 시간 등)를
	// 확인할 수 있도록 지원합니다. 메시지 형식은 Notifier 구현체에 따라 달라질 수 있습니다.
	//
	// 파라미터:
	//   - taskCtx: 작업 실행 컨텍스트 정보 (필수)
	//   - notifierID: 메시지를 발송할 대상 Notifier의 고유 ID
	//   - message: 전송할 알림 메시지 본문
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 처리되었는지 여부
	Notify(taskCtx TaskContext, notifierID string, message string) bool

	// NotifyDefault 시스템 기본 알림 채널로 일반 메시지를 발송합니다.
	// 특정 Notifier를 지정하지 않고, 시스템 설정에 정의된 기본 채널(예: 운영자 공통 채널)로
	// 알림을 보내야 할 때 사용합니다.
	//
	// 파라미터:
	//   - message: 전송할 메시지 내용
	//
	// 반환값:
	//   - bool: 발송 요청이 성공적으로 처리되었는지 여부
	NotifyDefault(message string) bool

	// SupportsHTML 지정된 Notifier가 HTML 포맷의 메시지 본문을 지원하는지 확인합니다.
	// 마크다운이나 텍스트 스타일링(굵게, 기울임, 링크 등)이 포함된 메시지를 전송하기 전에,
	// 해당 Notifier가 이를 올바르게 렌더링할 수 있는지 검사하는 용도로 사용됩니다.
	//
	// 파라미터:
	//   - notifierID: 지원 여부를 확인할 Notifier의 ID
	//
	// 반환값:
	//   - bool: HTML 포맷 지원 여부 (true: 지원함, false: 텍스트로만 처리됨)
	SupportsHTML(notifierID string) bool
}
