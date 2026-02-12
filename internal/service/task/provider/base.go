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
	msgScraperNotInitialized      = "Scraper가 초기화되지 않았습니다."
	msgSnapshotCreationFailed     = "작업결과데이터 생성이 실패하였습니다."
	msgNewSnapshotSaveFailed      = "작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s"
	msgPreviousSnapshotLoadFailed = "이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s"
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

	// 작업 취소 여부 플래그 - 원자적 접근 필요
	canceled atomic.Bool

	// 컨텍스트 취소를 위한 함수 (Run 실행 중에만 유효)
	cancelFunc context.CancelFunc
	cancelMu   sync.Mutex

	// 해당 작업을 누가/무엇이 실행 요청했는지를 나타냅니다.
	// (예: RunByUser - 사용자 수동 실행, RunByScheduler - 스케줄러 자동 실행)
	runBy contract.TaskRunBy
	// 작업 실행 시작 시각 - runTimeMu에 의해 보호됨
	runTime   time.Time
	runTimeMu sync.RWMutex

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
		runBy:      p.RunBy,

		storage: p.Storage,
		scraper: p.Scraper,

		logger: applog.WithComponentAndFields(component, applog.Fields{
			"task_id":     p.ID,
			"command_id":  p.CommandID,
			"instance_id": p.InstanceID,
			"notifier_id": p.NotifierID,
			"run_by":      p.RunBy,
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
	t.canceled.Store(true)

	// Run 실행 중이라면 컨텍스트도 취소합니다.
	t.cancelMu.Lock()
	if t.cancelFunc != nil {
		t.cancelFunc()
	}
	t.cancelMu.Unlock()
}

func (t *Base) IsCanceled() bool {
	return t.canceled.Load()
}

func (t *Base) GetRunBy() contract.TaskRunBy {
	return t.runBy
}

func (t *Base) Elapsed() time.Duration {
	t.runTimeMu.RLock()
	defer t.runTimeMu.RUnlock()

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
	// 실행 전 시스템에 의해 이미 취소된 상태라면 즉시 종료합니다 (Early Exit).
	if t.IsCanceled() {
		t.LogWithContext(component, applog.InfoLevel, "작업이 시작 전에 취소되었습니다", nil, nil)
		return
	}

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
			t.LogWithContext(component, applog.ErrorLevel, "Critical: Task 내부 Panic 발생 (Recovered)", applog.Fields{"panic_value": r}, err)

			// Panic 발생 시에도 결과 처리 로직을 태워 "작업 실패"로 기록하고 알림을 보냅니다.
			t.handleExecutionResult(ctx, notificationSender, "", nil, err)
		}
	}()

	t.runTimeMu.Lock()
	t.runTime = time.Now()
	t.runTimeMu.Unlock()

	// 1. 사전 검증 및 데이터 준비
	previousSnapshot, err := t.prepareExecution(ctx, notificationSender)
	if err != nil {
		return
	}

	// 사전 준비 완료 후 실행 직전 취소 확인
	// Storage Load 등의 준비 작업 중에 취소 요청이 들어온 경우,
	// 무거운 비즈니스 로직(execute)을 실행하지 않고 조기 종료합니다.
	if t.IsCanceled() {
		t.LogWithContext(component, applog.InfoLevel, "작업이 실행 직전에 취소되었습니다", nil, nil)
		return
	}

	// 2. 작업 실행
	message, newSnapshot, err := t.execute(ctx, previousSnapshot, notificationSender.SupportsHTML(t.notifierID))

	if t.IsCanceled() {
		t.LogWithContext(component, applog.InfoLevel, "작업 실행 중 취소가 감지되어 결과 처리를 중단합니다", nil, nil)
		return
	}

	// 3. 결과 처리
	t.handleExecutionResult(ctx, notificationSender, message, newSnapshot, err)
}

// prepareExecution 실행 전 필요한 조건을 검증하고 데이터를 준비합니다.
func (t *Base) prepareExecution(ctx context.Context, notificationSender contract.NotificationSender) (any, error) {
	if t.execute == nil {
		message := t.formatTaskErrorMessage(msgExecuteFuncNotInitialized)
		t.LogWithContext(component, applog.ErrorLevel, "작업 실행 중 에러가 발생하였습니다 (ExecuteFunc 미초기화)", applog.Fields{"detail": message}, nil)
		t.notifyError(ctx, notificationSender, message)
		return nil, apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", msgExecuteFuncNotInitialized, t.id, t.commandID)
	}

	var snapshot interface{}
	// Snapshot 생성 팩토리가 등록된 경우, Storage를 필수로 간주합니다.
	if t.newSnapshot != nil {
		snapshot = t.newSnapshot()

		if snapshot == nil {
			message := t.formatTaskErrorMessage(msgSnapshotCreationFailed)
			t.LogWithContext(component, applog.ErrorLevel, "작업 실행 중 에러가 발생하였습니다 (Snapshot 생성 실패)", applog.Fields{"detail": message}, nil)
			t.notifyError(ctx, notificationSender, message)
			return nil, apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", msgSnapshotCreationFailed, t.id, t.commandID)
		}

		if t.storage == nil {
			message := t.formatTaskErrorMessage(msgStorageNotInitialized)
			t.LogWithContext(component, applog.ErrorLevel, "작업 실행 중 에러가 발생하였습니다 (Storage 미초기화)", applog.Fields{"detail": message}, nil)
			t.notifyError(ctx, notificationSender, message)
			return nil, apperrors.Newf(apperrors.Internal, "%s (task_id: %s, command_id: %s)", msgStorageNotInitialized, t.id, t.commandID)
		}

		// Storage에서 이전 결과를 로드합니다.
		err := t.storage.Load(t.GetID(), t.GetCommandID(), snapshot)
		if err != nil {
			if errors.Is(err, contract.ErrTaskResultNotFound) {
				t.LogWithContext(component, applog.InfoLevel, "이전 작업 결과가 없습니다 (최초 실행)", nil, nil)
			} else {
				message := fmt.Sprintf(msgPreviousSnapshotLoadFailed, err)
				t.LogWithContext(component, applog.ErrorLevel, "이전 작업 결과 로딩 중 에러가 발생하였습니다", applog.Fields{"detail": message}, err)

				if !errors.Is(err, context.Canceled) {
					t.notifyError(ctx, notificationSender, message)
				}
				return nil, apperrors.Wrap(err, apperrors.Internal, "이전 작업 결과 로딩 실패")
			}
		}
	}

	return snapshot, nil
}

// handleExecutionResult 작업 결과를 처리합니다.
func (t *Base) handleExecutionResult(ctx context.Context, notificationSender contract.NotificationSender, message string, newSnapshot interface{}, err error) {
	// 1. 비즈니스 로직(execute) 실행 에러 처리
	if err != nil {
		errorMsg := t.formatTaskErrorMessage(err)
		if len(message) > 0 {
			errorMsg = fmt.Sprintf("%s\n\n%s", errorMsg, message)
		}
		t.LogWithContext(component, applog.ErrorLevel, "작업 실행 로직(execute) 중 에러가 발생하였습니다", applog.Fields{"detail": errorMsg}, err)

		// 사용자에 의한 취소인 경우 알림 소음을 방지하기 위해 에러 알림을 생략합니다.
		if !errors.Is(err, context.Canceled) {
			t.notifyError(ctx, notificationSender, errorMsg)
		}
		return
	}

	// 2. 상태 저장(Snapshot Save) 우선 수행
	if newSnapshot != nil && t.storage != nil {
		err := t.storage.Save(t.GetID(), t.GetCommandID(), newSnapshot)
		if err != nil {
			// [수정: Stability]
			// 저장이 실패하더라도 비즈니스 로직(execute)이 성공하여 생성된 중요한 알림 메시지(message)가 있다면,
			// 이를 에러 메시지와 함께 전송하여 사용자가 정보를 유실하지 않도록 합니다.
			errMsg := fmt.Sprintf(msgNewSnapshotSaveFailed, err)
			if message != "" {
				errMsg = fmt.Sprintf("%s\n\n---\n[비즈니스 실행 결과]\n%s", errMsg, message)
			}

			t.LogWithContext(component, applog.ErrorLevel, "작업 결과 저장 중 에러가 발생하였습니다", applog.Fields{"detail": errMsg}, err)
			t.notifyError(ctx, notificationSender, errMsg)
			return
		}
	}

	// 3. 모든 과정이 성공했을 때만 성공 알림 전송
	if len(message) > 0 {
		notifyErr := notificationSender.Notify(ctx, t.newNotification(message, false))

		if notifyErr != nil {
			t.LogWithContext(component, applog.ErrorLevel, "성공 알림 전송 중 에러가 발생하였습니다", nil, notifyErr)
		}
	}
}

func (t *Base) notifyError(ctx context.Context, notificationSender contract.NotificationSender, message string) {
	err := notificationSender.Notify(ctx, t.newNotification(message, true))

	if err != nil {
		t.LogWithContext(component, applog.ErrorLevel, "알림 전송 중 에러가 발생하였습니다", nil, err)
	}
}

// LogWithContext 컴포넌트 이름과 추가 필드를 포함하여 로깅을 수행하는 메서드입니다.
func (t *Base) LogWithContext(component string, level applog.Level, message string, fields applog.Fields, err error) {
	entry := t.logger.WithField("component", component)

	if len(fields) > 0 {
		entry = entry.WithFields(fields)
	}

	if err != nil {
		entry = entry.WithError(err)
	}

	entry.Log(level, message)
}

// formatTaskErrorMessage "작업 실패" 공통 문구와 세부 에러 내용을 조합합니다.
func (t *Base) formatTaskErrorMessage(detail any) string {
	return fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, detail)
}

// newNotification 새로운 Notification 객체를 생성합니다.
func (t *Base) newNotification(message string, isError bool) contract.Notification {
	return contract.Notification{
		NotifierID:    t.GetNotifierID(),
		TaskID:        t.GetID(),
		CommandID:     t.GetCommandID(),
		InstanceID:    t.GetInstanceID(),
		Message:       message,
		ElapsedTime:   t.Elapsed(),
		ErrorOccurred: isError,
		Cancelable:    false, // 통상적으로 결과 기반 알림은 취소 불가능
	}
}
