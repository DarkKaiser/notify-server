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

// Task 개별 Task 인스턴스의 생명주기를 제어하고 상태를 조회하기 위한 인터페이스입니다.
//
// 이 인터페이스는 Service 레이어와 구체적인 Task 구현체(Base 기반) 사이의 계약을 정의합니다.
// Service는 구현 세부사항을 알 필요 없이 이 인터페이스만으로 Task를 실행, 취소, 모니터링할 수 있습니다.
type Task interface {
	ID() contract.TaskID
	CommandID() contract.TaskCommandID
	InstanceID() contract.TaskInstanceID

	// NotifierID 작업 완료 시 알림을 전송할 대상 채널의 식별자를 반환합니다.
	NotifierID() contract.NotifierID

	// Cancel 실행 중인 작업의 취소를 요청합니다.
	// 호출 즉시 IsCanceled()가 true를 반환하며, Run 내부 로직은 이를 감지하여 조기 종료해야 합니다.
	Cancel()

	// IsCanceled 작업이 취소되었는지 여부를 반환합니다.
	// Run 메서드 내부에서 주기적으로 확인하여 취소 시 작업을 중단하는 용도로 사용됩니다.
	IsCanceled() bool

	// Elapsed 작업 시작 시점부터 현재까지의 경과 시간을 반환합니다.
	// 작업 시작 전에는 0을 반환합니다.
	Elapsed() time.Duration

	// Run Task의 핵심 비즈니스 로직을 실행합니다.
	// 이 메서드는 동기적으로 실행되며, 작업이 완료되거나 취소될 때까지 블로킹됩니다.
	// 비동기 실행이 필요한 경우 호출자가 goroutine과 동기화를 직접 관리해야 합니다.
	Run(ctx context.Context, notificationSender contract.NotificationSender)
}

// ExecuteFunc Task의 핵심 비즈니스 로직을 수행하는 함수 타입입니다.
//
// 이 함수는 가능한 한 순수 함수(Pure Function)에 가깝게 구현되어야 합니다.
// 즉, 외부 상태를 직접 변경하지 않고, 입력(previousSnapshot)을 받아 처리한 후
// 결과(메시지, 새로운 Snapshot)를 반환하는 방식으로 동작해야 합니다.
//
// 매개변수:
//   - ctx: 작업 실행의 생명주기를 제어하는 컨텍스트 (취소, 타임아웃 처리)
//   - previousSnapshot: 이전 실행 시 저장된 작업 결과 데이터 (최초 실행 시 nil 또는 초기값)
//   - supportsHTML: 알림 대상 채널의 HTML 지원 여부
//
// 반환값:
//   - string: 사용자에게 전송할 알림 메시지 (빈 문자열일 경우 알림 전송 안 함)
//   - any: 다음 실행을 위해 저장할 새로운 작업 결과 데이터 (nil일 경우 저장 안 함)
//   - error: 실행 중 발생한 에러 (nil이 아니면 작업 실패로 처리되며 에러 알림 전송)
type ExecuteFunc func(ctx context.Context, previousSnapshot any, supportsHTML bool) (string, any, error)

// NewSnapshotFunc Task 작업 결과 데이터의 빈 인스턴스를 생성하는 팩토리 함수 타입입니다.
type NewSnapshotFunc func() any

// NewTaskParams 새로운 Task 생성에 필요한 매개변수들을 정의하는 구조체입니다.
type NewTaskParams struct {
	AppConfig *config.AppConfig

	// Request 사용자가 제출한 Task 실행 요청 정보입니다.
	Request *contract.TaskSubmitRequest

	// InstanceID 실행 중인 Task의 고유 식별자입니다.
	InstanceID contract.TaskInstanceID

	// Storage Task 작업 결과 데이터를 영구 저장하기 위한 저장소 인터페이스입니다.
	// 이전 작업 결과 조회 및 새로운 결과 저장에 사용됩니다.
	Storage contract.TaskResultStore

	// Fetcher HTTP 요청을 수행하는 인터페이스입니다.
	Fetcher fetcher.Fetcher

	// NewSnapshot Task 작업 결과 데이터의 빈 인스턴스를 생성하는 팩토리 함수입니다.
	NewSnapshot NewSnapshotFunc
}

// NewTaskFunc 새로운 Task 인스턴스를 생성하는 팩토리 함수 타입입니다.
type NewTaskFunc func(NewTaskParams) (Task, error)
