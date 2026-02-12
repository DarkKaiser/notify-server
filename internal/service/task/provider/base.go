package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

const (
	msgTaskExecutionFailed        = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"
	msgStorageNotInitialized      = "Storage가 초기화되지 않았습니다."
	msgExecuteFuncNotInitialized  = "Execute()가 초기화되지 않았습니다."
	msgSnapshotCreationFailed     = "작업결과데이터 생성이 실패하였습니다."
	msgNewSnapshotSaveFailed      = "작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s"
	msgPreviousSnapshotLoadFailed = "이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s\n\n빈 작업결과데이터를 이용하여 작업을 계속 진행합니다."
)

// Base 개별 작업의 실행 단위이자 상태를 관리하는 핵심 구조체입니다.
//
// Base는 불변 상태(id, commandID 등)와 가변 상태(canceled, storage 상태 등)를 모두 포함하며,
// Service에 의해 생성되고 생명주기가 관리됩니다. 이 구조체는 '작업의 정의'와 '실행 상태'를 모두 캡슐화합니다.
//
// 주요 특징:
//   - 상태 보존 (Stateful): storage를 통해 실행 결과를 영속화하여, 스크래핑 작업 간의 데이터 연속성을 보장합니다.
//   - 실행 제어 (Control): Cancel() 메서드를 통해 실행 중인 작업을 안전하게 중단할 수 있습니다.
//   - 의존성 주입 (DI): storage, fetcher 등의 외부 의존성을 필드로 주입받아 테스트 용이성을 높입니다.
type Base struct {
	id         contract.TaskID         // 실행할 작업의 고유 식별자입니다. (예: "NAVER", "KURLY")
	commandID  contract.TaskCommandID  // 작업 내에서 수행할 구체적인 명령어 식별자입니다. (예: "CheckPrice")
	instanceID contract.TaskInstanceID // 이번 작업 실행 인스턴스에 할당된 유일한 식별자(UUID 등)입니다.

	// 알림을 전송할 대상 채널 또는 수단(Notifier)의 식별자입니다.
	notifierID contract.NotifierID

	// 작업 취소 여부 플래그 (0: false, 1: true) - 원자적 접근 필요
	canceled int32

	// 컨텍스트 취소를 위한 함수 (Run 실행 중에만 유효)
	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex

	// 해당 작업을 누가/무엇이 실행 요청했는지를 나타냅니다.
	// (예: RunByUser - 사용자 수동 실행, RunByScheduler - 스케줄러 자동 실행)
	runBy contract.TaskRunBy
	// 작업 실행 시작 시각
	runTime time.Time

	// execute는 실제 비즈니스 로직(스크래핑, 가격 비교 등)을 수행하는 함수입니다.
	execute ExecuteFunc

	// scraper는 웹 요청(HTTP) 및 파싱을 수행하는 컴포넌트입니다.
	scraper scraper.Scraper

	// storage는 작업의 상태를 저장하고 불러오는 인터페이스입니다.
	storage contract.TaskResultStore

	// logger 고정 필드가 바인딩된 로거 인스턴스입니다.
	// 로깅 시 매번 맵을 복사하는 오버헤드를 줄이기 위해 생성 시점에 초기화하여 재사용합니다.
	logger *applog.Entry

	// newSnapshot은 작업 결과 데이터(Snapshot)의 새 인스턴스를 생성하는 팩토리 함수입니다.
	newSnapshot NewSnapshotFunc
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ Task = (*Base)(nil)

// BaseParams Base 구조체 초기화에 필요한 매개변수들을 정의하는 구조체입니다.
// 인자가 많아짐에 따른 가독성 저하를 방지하고, 향후 공통 필드 추가 시 확장성을 보장합니다.
type BaseParams struct {
	ID          contract.TaskID
	CommandID   contract.TaskCommandID
	InstanceID  contract.TaskInstanceID
	NotifierID  contract.NotifierID
	RunBy       contract.TaskRunBy
	Storage     contract.TaskResultStore
	Scraper     scraper.Scraper
	NewSnapshot NewSnapshotFunc
}

// NewBase Base 구조체의 필수 불변 필드들을 초기화하여 반환하는 생성자입니다.
// 하위 Task 구현체는 이 함수를 사용하여 기본 Base 필드를 초기화해야 합니다.
func NewBase(p BaseParams) *Base {
	return &Base{
		id:         p.ID,
		commandID:  p.CommandID,
		instanceID: p.InstanceID,
		notifierID: p.NotifierID,
		canceled:   0,
		runBy:      p.RunBy,

		storage: p.Storage,
		scraper: p.Scraper,

		logger: applog.WithComponentAndFields("task.executor", applog.Fields{
			"task_id":     p.ID,
			"command_id":  p.CommandID,
			"instance_id": p.InstanceID,
			"notifier_id": p.NotifierID,
		}),

		newSnapshot: p.NewSnapshot,
	}
}

// NewBaseFromParams NewTaskParams를 기반으로 Base 인스턴스를 생성하는 헬퍼 함수입니다.
// 개별 프로바이더 구현체에서 반복적으로 나타나는 Base 초기화 코드를 간소화합니다.
func NewBaseFromParams(p NewTaskParams) *Base {
	var s scraper.Scraper
	if p.Fetcher != nil {
		s = scraper.New(p.Fetcher)
	}

	return NewBase(BaseParams{
		ID:          p.Request.TaskID,
		CommandID:   p.Request.CommandID,
		InstanceID:  p.InstanceID,
		NotifierID:  p.Request.NotifierID,
		RunBy:       p.Request.RunBy,
		Storage:     p.Storage,
		Scraper:     s,
		NewSnapshot: p.NewSnapshot,
	})
}

func (t *Base) GetID() contract.TaskID {
	return t.id
}

func (t *Base) GetCommandID() contract.TaskCommandID {
	return t.commandID
}

func (t *Base) GetInstanceID() contract.TaskInstanceID {
	return t.instanceID
}

func (t *Base) GetNotifierID() contract.NotifierID {
	return t.notifierID
}

func (t *Base) Cancel() {
	atomic.StoreInt32(&t.canceled, 1)

	// Run 실행 중이라면 컨텍스트도 취소합니다.
	t.cancelMu.Lock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.cancelMu.Unlock()
}

func (t *Base) IsCanceled() bool {
	return atomic.LoadInt32(&t.canceled) == 1
}

func (t *Base) SetRunBy(runBy contract.TaskRunBy) {
	t.runBy = runBy
}

func (t *Base) GetRunBy() contract.TaskRunBy {
	return t.runBy
}

func (t *Base) Elapsed() time.Duration {
	if t.runTime.IsZero() {
		return 0
	}

	return time.Since(t.runTime)
}

func (t *Base) SetExecute(fn ExecuteFunc) {
	t.execute = fn
}

func (t *Base) GetScraper() scraper.Scraper {
	return t.scraper
}

// Run Task의 실행 수명 주기를 관리하는 메인 진입점입니다.
func (t *Base) Run(ctx context.Context, notificationSender contract.NotificationSender) {
	// 상위 컨텍스트를 래핑하여 Cancel() 호출 시 즉시 취소 신호를 전파할 수 있도록 합니다.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// cancelFunc 등록 (Cancel 메서드에서 사용)
	t.cancelMu.Lock()
	t.cancelFunc = cancel
	t.cancelMu.Unlock()

	// Run 종료 시 cancelFunc 정리
	defer func() {
		t.cancelMu.Lock()
		t.cancelFunc = nil
		t.cancelMu.Unlock()
	}()

	defer func() {
		if r := recover(); r != nil {
			err := apperrors.New(apperrors.Internal, fmt.Sprintf("Task 실행 도중 Panic 발생: %v", r))
			t.LogWithContext("task.executor", applog.ErrorLevel, "Critical: Task 내부 Panic 발생 (Recovered)", applog.Fields{"panic_value": r}, err)

			// Panic 발생 시에도 결과 처리 로직을 태워 "작업 실패"로 기록하고 알림을 보냅니다.
			t.handleExecutionResult(ctx, notificationSender, "", nil, err)
		}
	}()

	t.runTime = time.Now()

	// 1. 사전 검증 및 데이터 준비
	previousSnapshot, err := t.prepareExecution(ctx, notificationSender)
	if err != nil {
		return
	}

	// 사전 준비 완료 후 실행 직전 취소 확인
	// Storage Load 등의 준비 작업 중에 취소 요청이 들어온 경우,
	// 무거운 비즈니스 로직(execute)을 실행하지 않고 조기 종료합니다.
	if t.IsCanceled() {
		t.LogWithContext("task.executor", applog.InfoLevel, "작업이 실행 직전에 취소되었습니다", nil, nil)
		return
	}

	// 2. 작업 실행
	message, newSnapshot, err := t.execute(ctx, previousSnapshot, notificationSender.SupportsHTML(t.notifierID))

	if t.IsCanceled() {
		return
	}

	// 3. 결과 처리
	t.handleExecutionResult(ctx, notificationSender, message, newSnapshot, err)
}

// prepareExecution 실행 전 필요한 조건을 검증하고 데이터를 준비합니다.
func (t *Base) prepareExecution(ctx context.Context, notificationSender contract.NotificationSender) (any, error) {
	if t.execute == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgExecuteFuncNotInitialized)
		t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, nil)
		t.notifyError(ctx, notificationSender, message)
		return nil, apperrors.New(apperrors.Internal, msgExecuteFuncNotInitialized)
	}

	var snapshot interface{}
	if t.newSnapshot != nil {
		snapshot = t.newSnapshot()
	}

	if snapshot == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgSnapshotCreationFailed)
		t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, nil)
		t.notifyError(ctx, notificationSender, message)
		return nil, apperrors.New(apperrors.Internal, msgSnapshotCreationFailed)
	}

	if t.storage == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgStorageNotInitialized)
		t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, nil)
		t.notifyError(ctx, notificationSender, message)
		return nil, apperrors.New(apperrors.Internal, msgStorageNotInitialized)
	}

	err := t.storage.Load(t.GetID(), t.GetCommandID(), snapshot)
	if err != nil {
		if errors.Is(err, contract.ErrTaskResultNotFound) {
			// 최초 실행 시에는 데이터가 없는 것이 정상입니다.
			// 경고 로그 대신 Info 로그를 남기고 빈 스냅샷으로 시작합니다.
			t.LogWithContext("task.executor", applog.InfoLevel, "이전 작업 결과가 없습니다 (최초 실행)", nil, nil)
		} else {
			// [Policy: Fail-Fast]
			// 스토리지 장애, 네트워크 에러 등으로 로딩에 실패한 경우
			// 불완전한 상태로 작업을 강행하지 않고 즉시 실패 처리합니다.
			// 이는 데이터 정합성(최저가 이력 등)을 보장하고 오탐지 알림을 방지하기 위함입니다.
			message := fmt.Sprintf(msgPreviousSnapshotLoadFailed, err)
			t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, err)
			t.notifyError(ctx, notificationSender, message)
			return nil, apperrors.Wrap(err, apperrors.Internal, "이전 작업 결과 로딩 실패")
		}
	}

	return snapshot, nil
}

// handleExecutionResult 작업 결과를 처리합니다.
func (t *Base) handleExecutionResult(ctx context.Context, notificationSender contract.NotificationSender, message string, newSnapshot interface{}, err error) {
	if err == nil {
		// 성공 알림 전송 여부를 추적합니다.
		successNotified := false
		if len(message) > 0 {
			notificationSender.Notify(ctx, contract.Notification{
				NotifierID:    t.GetNotifierID(),
				TaskID:        t.GetID(),
				CommandID:     t.GetCommandID(),
				InstanceID:    t.GetInstanceID(),
				Message:       message,
				ElapsedTime:   t.Elapsed(),
				ErrorOccurred: false,
				Cancelable:    false, // Completed -> Not cancelable
			})
			successNotified = true
		}

		if newSnapshot != nil {
			if err0 := t.storage.Save(t.GetID(), t.GetCommandID(), newSnapshot); err0 != nil {
				saveErrMsg := fmt.Sprintf(msgNewSnapshotSaveFailed, err0)
				// 스냅샷 저장 실패는 시스템 정합성을 깨뜨리는 심각한 문제이므로 Error 레벨로 기록합니다.
				t.LogWithContext("task.executor", applog.ErrorLevel, saveErrMsg, nil, err0)

				// 성공 알림을 보낸 경우, 다음 실행 시 중복 알림 가능성을 운영자에게 경고합니다.
				if successNotified {
					warningMsg := fmt.Sprintf("⚠️ 알림 전송은 성공했으나 상태 저장에 실패했습니다.\n다음 실행 시 중복 알림이 발생할 수 있습니다.\n\n☑ %s", err0)
					t.notifyError(ctx, notificationSender, warningMsg)
				} else {
					// 성공 알림을 보내지 않은 경우, 기존 에러 메시지를 그대로 전송합니다.
					t.notifyError(ctx, notificationSender, saveErrMsg)
				}
			}
		}
	} else {
		// execute 함수가 에러와 함께 메시지를 반환한 경우, 해당 메시지를 알림에 포함합니다.
		errorMsg := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, err)
		if len(message) > 0 {
			errorMsg = fmt.Sprintf("%s\n\n%s", errorMsg, message)
		}

		t.LogWithContext("task.executor", applog.ErrorLevel, errorMsg, nil, err)
		t.notifyError(ctx, notificationSender, errorMsg)
	}
}

func (t *Base) notify(ctx context.Context, notificationSender contract.NotificationSender, message string) error {
	return notificationSender.Notify(ctx, contract.Notification{
		NotifierID:    t.GetNotifierID(),
		TaskID:        t.GetID(),
		CommandID:     t.GetCommandID(),
		InstanceID:    t.GetInstanceID(),
		Message:       message,
		ElapsedTime:   t.Elapsed(),
		ErrorOccurred: false,
		Cancelable:    t.GetRunBy() == contract.TaskRunByUser,
	})
}

func (t *Base) notifyError(ctx context.Context, notificationSender contract.NotificationSender, message string) error {
	return notificationSender.Notify(ctx, contract.Notification{
		NotifierID:    t.GetNotifierID(),
		TaskID:        t.GetID(),
		CommandID:     t.GetCommandID(),
		InstanceID:    t.GetInstanceID(),
		Message:       message,
		ElapsedTime:   t.Elapsed(),
		ErrorOccurred: true,
		Cancelable:    false, // Error means termination, so not cancelable
	})
}

// LogWithContext 컴포넌트 이름과 추가 필드를 포함하여 로깅을 수행하는 메서드입니다.
func (t *Base) LogWithContext(component string, level applog.Level, message string, fields applog.Fields, err error) {
	entry := t.logger.WithField("component", component).WithField("run_by", t.GetRunBy())

	if len(fields) > 0 {
		entry = entry.WithFields(fields)
	}

	if err != nil {
		entry = entry.WithError(err)
	}

	entry.Log(level, message)
}
