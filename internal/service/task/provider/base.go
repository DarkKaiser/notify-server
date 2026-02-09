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
type ExecuteFunc func(ctx context.Context, previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error)

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
}

// NewBase Base 구조체의 필수 불변 필드들을 초기화하여 반환하는 생성자입니다.
// 하위 Task 구현체는 이 함수를 사용하여 기본 Base 필드를 초기화해야 합니다.
func NewBase(id contract.TaskID, commandID contract.TaskCommandID, instanceID contract.TaskInstanceID, notifierID contract.NotifierID, runBy contract.TaskRunBy) *Base {
	return &Base{
		id:         id,
		commandID:  commandID,
		instanceID: instanceID,
		notifierID: notifierID,
		canceled:   0,
		runBy:      runBy,
	}
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

func (t *Base) ElapsedTimeAfterRun() int64 {
	return int64(time.Since(t.runTime).Seconds())
}

func (t *Base) SetExecute(fn ExecuteFunc) {
	t.execute = fn
}

func (t *Base) SetScraper(s scraper.Scraper) {
	t.scraper = s
}

func (t *Base) GetScraper() scraper.Scraper {
	return t.scraper
}

func (t *Base) SetStorage(storage contract.TaskResultStore) {
	t.storage = storage
}

// Run Task의 실행 수명 주기를 관리하는 메인 진입점입니다.
func (t *Base) Run(ctx context.Context, notificationSender contract.NotificationSender, taskStopWG *sync.WaitGroup, taskDoneC chan<- contract.TaskInstanceID) {
	defer taskStopWG.Done()

	// [Deep Panic Safety] defer는 역순(LIFO)으로 실행되므로, recover보다 늦게, taskStopWG.Done()보다 먼저 실행되도록 위치시킵니다.
	// 1. Recover (Panic 복구) -> 2. taskDoneC 전송 (완료 신호) -> 3. Done (WaitGroup 감소, 채널 닫힘 가능성)
	// 순서로 실행되어야 "닫힌 채널에 전송"하는 Panic을 방지할 수 있습니다.
	defer func() {
		taskDoneC <- t.instanceID
	}()

	defer func() {
		if r := recover(); r != nil {
			err := apperrors.New(apperrors.Internal, fmt.Sprintf("Task 실행 도중 Panic 발생: %v", r))
			t.LogWithContext("task.executor", applog.ErrorLevel, "Critical: Task 내부 Panic 발생 (Recovered)", nil, err)

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

	// 2. 작업 실행
	message, newSnapshot, err := t.execute(ctx, previousSnapshot, notificationSender.SupportsHTML(t.notifierID))

	if t.IsCanceled() {
		return
	}

	// 3. 결과 처리
	t.handleExecutionResult(ctx, notificationSender, message, newSnapshot, err)
}

// prepareExecution 실행 전 필요한 조건을 검증하고 데이터를 준비합니다.
func (t *Base) prepareExecution(ctx context.Context, notificationSender contract.NotificationSender) (interface{}, error) {
	if t.execute == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgExecuteFuncNotInitialized)
		t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, nil)
		t.notifyError(ctx, notificationSender, message)
		return nil, apperrors.New(apperrors.Internal, msgExecuteFuncNotInitialized)
	}

	var snapshot interface{}
	cfg, findErr := FindConfig(t.GetID(), t.GetCommandID())
	if findErr == nil {
		snapshot = cfg.Command.NewSnapshot()
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
			message := fmt.Sprintf(msgPreviousSnapshotLoadFailed, err)
			t.LogWithContext("task.executor", applog.WarnLevel, message, nil, err)
			t.notify(ctx, notificationSender, message)
		}
	}

	return snapshot, nil
}

// handleExecutionResult 작업 결과를 처리합니다.
func (t *Base) handleExecutionResult(ctx context.Context, notificationSender contract.NotificationSender, message string, newSnapshot interface{}, err error) {
	if err == nil {
		if len(message) > 0 {
			notificationSender.Notify(ctx, contract.Notification{
				NotifierID:    t.GetNotifierID(),
				TaskID:        t.GetID(),
				CommandID:     t.GetCommandID(),
				InstanceID:    t.GetInstanceID(),
				Message:       message,
				ElapsedTime:   time.Duration(t.ElapsedTimeAfterRun()) * time.Second,
				ErrorOccurred: false,
				Cancelable:    false, // Completed -> Not cancelable
			})
		}

		if newSnapshot != nil {
			if err0 := t.storage.Save(t.GetID(), t.GetCommandID(), newSnapshot); err0 != nil {
				message := fmt.Sprintf(msgNewSnapshotSaveFailed, err0)
				t.LogWithContext("task.executor", applog.WarnLevel, message, nil, err0)
				t.notifyError(ctx, notificationSender, message)
			}
		}
	} else {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, err)
		t.LogWithContext("task.executor", applog.ErrorLevel, message, nil, err)
		t.notifyError(ctx, notificationSender, message)
	}
}

func (t *Base) notify(ctx context.Context, notificationSender contract.NotificationSender, message string) error {
	return notificationSender.Notify(ctx, contract.Notification{
		NotifierID:    t.GetNotifierID(),
		TaskID:        t.GetID(),
		CommandID:     t.GetCommandID(),
		InstanceID:    t.GetInstanceID(),
		Message:       message,
		ElapsedTime:   time.Duration(t.ElapsedTimeAfterRun()) * time.Second,
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
		ElapsedTime:   time.Duration(t.ElapsedTimeAfterRun()) * time.Second,
		ErrorOccurred: true,
		Cancelable:    false, // Error means termination, so not cancelable
	})
}

// LogWithContext 컴포넌트 이름과 추가 필드를 포함하여 로깅을 수행하는 메서드입니다.
func (t *Base) LogWithContext(component string, level applog.Level, message string, fields applog.Fields, err error) {
	fieldsMap := applog.Fields{
		"task_id":     t.GetID(),
		"command_id":  t.GetCommandID(),
		"instance_id": t.GetInstanceID(),
		"notifier_id": t.GetNotifierID(),
		"run_by":      t.GetRunBy(),
	}
	for k, v := range fields {
		fieldsMap[k] = v
	}

	if err != nil {
		fieldsMap["error"] = err
	}

	applog.WithComponentAndFields(component, fieldsMap).Log(level, message)
}
