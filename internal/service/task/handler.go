package task

import (
	"context"
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/storage"
)

// Handler 개별 Task 인스턴스를 제어하고 상태를 조회하기 위한 인터페이스입니다.
//
// Handler는 Service 레이어와 개별 Task 구현체(Task 구조체) 사이의 계약(Contract)을 정의합니다.
// Service는 이 인터페이스를 통해 Task의 구체적인 구현을 알 필요 없이,
// 표준화된 방식으로 실행(Run), 취소(Cancel), 상태 확인 등을 수행할 수 있습니다.
type Handler interface {
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

	// ElapsedTimeAfterRun 작업이 시작된 후 경과된 시간(초)을 반환합니다.
	// 작업 모니터링이나 타임아웃 감지에 활용될 수 있습니다.
	ElapsedTimeAfterRun() int64

	// SetStorage 작업 결과를 저장할 스토리지를 주입합니다.
	// 테스트 시 Mock 스토리지를 주입하거나, 런타임에 동적으로 스토리지를 변경할 때 사용됩니다.
	SetStorage(storage storage.TaskResultStorage)

	// Run 작업을 실행하는 메인 메서드입니다.
	Run(ctx context.Context, notificationSender contract.NotificationSender, taskStopWG *sync.WaitGroup, taskDoneC chan<- contract.TaskInstanceID)
}
