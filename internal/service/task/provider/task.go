package provider

import (
	"context"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
)

// component Task 서비스의 Provider 로깅용 컴포넌트 이름
const component = "task.provider"

// Task 개별 Task 인스턴스를 제어하고 상태를 조회하기 위한 인터페이스입니다.
//
// Task는 Service 레이어와 개별 Task 구현체(Base 구조체) 사이의 계약(Contract)을 정의합니다.
// Service는 이 인터페이스를 통해 Task의 구체적인 구현을 알 필요 없이,
// 표준화된 방식으로 실행(Run), 취소(Cancel), 상태 확인 등을 수행할 수 있습니다.
type Task interface {
	GetID() contract.TaskID
	GetCommandID() contract.TaskCommandID
	GetInstanceID() contract.TaskInstanceID

	// GetNotifierID 알림을 발송할 대상 Notifier의 ID를 반환합니다.
	GetNotifierID() contract.NotifierID

	// Cancel 작업을 취소 요청합니다.
	// 호출 즉시 IsCanceled()가 true를 반환해야 하며, 실행 중인 로직은 이를 감지하여 조기 종료해야 합니다.
	Cancel()

	// IsCanceled 작업이 취소되었는지 여부를 반환합니다.
	// Run 루프 내에서 주기적으로 확인하여, 취소 시 작업을 중단하는 용도로 사용됩니다.
	IsCanceled() bool

	// Elapsed 작업이 시작된 후 현재까지의 경과 시간을 반환합니다.
	Elapsed() time.Duration

	// Run Task를 수행합니다.
	// 순차적으로 실행되며, 작업이 완료되면 리턴합니다. 동기화(goroutine/waitgroup) 처리는 호출자의 책임입니다.
	Run(ctx context.Context, notificationSender contract.NotificationSender)
}

// ExecuteFunc 작업 실행 로직을 정의하는 함수 타입입니다.
//
// 이 함수는 순수 함수(Pure Function)에 가깝게 구현되어야 하며,
// 작업에 필요한 데이터(Snapshot)를 받아 처리한 후 결과 메시지와 변경된 데이터를 반환합니다.
//
// 매개변수:
//   - ctx: 작업 실행 컨텍스트 (취소 및 타임아웃 처리용)
//   - previousSnapshot: 이전 실행 시 저장된 데이터 (상태 복원용). 최초 실행 시에는 nil 또는 초기값이 전달됩니다.
//   - supportsHTML: 알림 채널(Notifier)이 HTML 포맷을 지원하는지 여부.
//
// 반환값:
//   - string: 사용자에게 알림으로 전송할 메시지 본문. 빈 문자열일 경우 알림을 보내지 않습니다.
//   - interface{}: 실행 완료 후 저장할 새로운 데이터. 다음 실행 시 data 인자로 전달됩니다.
//   - error: 실행 중 발생한 에러. nil이 아니면 작업 실패로 처리됩니다.
type ExecuteFunc func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error)

// NewSnapshotFunc Task 결과 데이터 구조체를 생성하는 팩토리 함수입니다.
type NewSnapshotFunc func() any

// NewTaskParams 새로운 Task 인스턴스 생성에 필요한 매개변수들을 정의하는 구조체입니다.
// 인자가 많아짐에 따른 가독성 저하를 방지하고, 향후 공통 필드 추가 시 하위 호환성을 보장합니다.
type NewTaskParams struct {
	InstanceID  contract.TaskInstanceID
	Request     *contract.TaskSubmitRequest
	AppConfig   *config.AppConfig
	Storage     contract.TaskResultStore
	Fetcher     fetcher.Fetcher
	NewSnapshot NewSnapshotFunc
}

// NewTaskFunc 새로운 Task 인스턴스를 생성하는 팩토리 함수 타입입니다.
type NewTaskFunc func(p NewTaskParams) (Task, error)
