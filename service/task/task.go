package task

import (
	"fmt"
	"sync"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
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
//   - previousSnapshot: 이전 실행 시 저장된 데이터 (상태 복원용). 최초 실행 시에는 nil 또는 초기값이 전달됩니다.
//   - supportsHTML: 알림 채널(Notifier)이 HTML 포맷을 지원하는지 여부.
//
// 반환값:
//   - string: 사용자에게 알림으로 전송할 메시지 본문. 빈 문자열일 경우 알림을 보내지 않습니다.
//   - interface{}: 실행 완료 후 저장할 새로운 데이터(data). 다음 실행 시 data 인자로 전달됩니다.
//   - error: 실행 중 발생한 에러. nil이 아니면 작업 실패로 처리됩니다.
type ExecuteFunc func(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error)

// Task 개별 작업의 실행 단위이자 상태를 관리하는 핵심 구조체입니다.
//
// Task는 불변 상태(ID, Config 등)와 가변 상태(Canceled, Storage 상태 등)를 모두 포함하며,
// Service에 의해 생성되고 생명주기가 관리됩니다. 이 구조체는 '작업의 정의'와 '실행 상태'를 모두 캡슐화합니다.
//
// 주요 특징:
//   - 상태 보존 (Stateful): Storage를 통해 실행 결과를 영속화하여, 스크래핑 작업 간의 데이터 연속성을 보장합니다.
//   - 실행 제어 (Control): Cancel() 메서드를 통해 실행 중인 작업을 안전하게 중단할 수 있습니다.
//   - 의존성 주입 (DI): Storage, Fetcher 등의 외부 의존성을 필드로 주입받아 테스트 용이성을 높입니다.
type Task struct {
	ID         ID         // 실행할 작업의 고유 식별자입니다. (예: "NAVER", "KURLY")
	CommandID  CommandID  // 작업 내에서 수행할 구체적인 명령어 식별자입니다. (예: "CheckPrice")
	InstanceID InstanceID // 이번 작업 실행 인스턴스에 할당된 유일한 식별자(UUID 등)입니다.

	// 알림을 전송할 대상 채널 또는 수단(Notifier)의 식별자입니다.
	NotifierID string

	// 작업 취소 여부 플래그
	Canceled bool

	// 해당 작업을 누가/무엇이 실행 요청했는지를 나타냅니다.
	// (예: RunByUser - 사용자 수동 실행, RunByScheduler - 스케줄러 자동 실행)
	RunBy RunBy
	// 작업 실행 시작 시각
	RunTime time.Time

	// Execute는 실제 비즈니스 로직(스크래핑, 가격 비교 등)을 수행하는 함수입니다.
	Execute ExecuteFunc

	// Storage는 작업의 상태를 저장하고 불러오는 인터페이스입니다.
	Storage TaskResultStorage

	// Fetcher는 웹 요청(HTTP)을 수행하는 클라이언트 추상화입니다.
	Fetcher Fetcher
}

func (t *Task) GetID() ID {
	return t.ID
}

func (t *Task) GetCommandID() CommandID {
	return t.CommandID
}

func (t *Task) GetInstanceID() InstanceID {
	return t.InstanceID
}

func (t *Task) GetNotifierID() string {
	return t.NotifierID
}

func (t *Task) Cancel() {
	t.Canceled = true
}

func (t *Task) IsCanceled() bool {
	return t.Canceled
}

func (t *Task) ElapsedTimeAfterRun() int64 {
	return int64(time.Since(t.RunTime).Seconds())
}

func (t *Task) SetStorage(storage TaskResultStorage) {
	t.Storage = storage
}

// Run Task의 실행 수명 주기를 관리하는 메인 진입점입니다.
func (t *Task) Run(taskCtx TaskContext, notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	// 1. 사전 검증 및 데이터 준비
	previousSnapshot, err := t.prepareExecution(taskCtx, notificationSender)
	if err != nil {
		return
	}

	// 2. 작업 실행
	message, newSnapshot, err := t.execute(previousSnapshot, notificationSender.SupportsHTML(t.NotifierID))

	if t.IsCanceled() {
		return
	}

	// 3. 결과 처리
	t.handleExecutionResult(taskCtx, notificationSender, message, newSnapshot, err)
}

// prepareExecution 실행 전 필요한 조건을 검증하고 데이터를 준비합니다.
func (t *Task) prepareExecution(taskCtx TaskContext, notificationSender NotificationSender) (interface{}, error) {
	if t.Execute == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgExecuteFuncNotInitialized)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgExecuteFuncNotInitialized)
	}

	var snapshot interface{}
	cfg, findErr := findConfig(t.GetID(), t.GetCommandID())
	if findErr == nil {
		snapshot = cfg.Command.NewSnapshot()
	}
	if snapshot == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgSnapshotCreationFailed)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgSnapshotCreationFailed)
	}

	if t.Storage == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgStorageNotInitialized)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgStorageNotInitialized)
	}

	err := t.Storage.Load(t.GetID(), t.GetCommandID(), snapshot)
	if err != nil {
		message := fmt.Sprintf(msgPreviousSnapshotLoadFailed, err)
		t.log(log.WarnLevel, message, err)
		t.notify(taskCtx, notificationSender, message)
	}

	return snapshot, nil
}

// execute 실제 비즈니스 로직(Execute)을 실행합니다.
func (t *Task) execute(previousSnapshot interface{}, supportsHTML bool) (string, interface{}, error) {
	return t.Execute(previousSnapshot, supportsHTML)
}

// handleExecutionResult 작업 실행 결과를 처리합니다.
func (t *Task) handleExecutionResult(taskCtx TaskContext, notificationSender NotificationSender, message string, newSnapshot interface{}, err error) {
	if err == nil {
		if len(message) > 0 {
			t.notify(taskCtx, notificationSender, message)
		}

		if newSnapshot != nil {
			if err0 := t.Storage.Save(t.GetID(), t.GetCommandID(), newSnapshot); err0 != nil {
				message := fmt.Sprintf(msgNewSnapshotSaveFailed, err0)
				t.log(log.WarnLevel, message, err0)
				t.notifyError(taskCtx, notificationSender, message)
			}
		}
	} else {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, err)
		t.log(log.ErrorLevel, message, err)
		t.notifyError(taskCtx, notificationSender, message)
	}
}

func (t *Task) notify(taskCtx TaskContext, notificationSender NotificationSender, message string) bool {
	return notificationSender.Notify(taskCtx, t.GetNotifierID(), message)
}

func (t *Task) notifyError(taskCtx TaskContext, notificationSender NotificationSender, message string) bool {
	return notificationSender.Notify(taskCtx.WithError(), t.GetNotifierID(), message)
}

// log 로깅을 수행하는 내부 Helper 함수입니다.
func (t *Task) log(level log.Level, message string, err error) {
	fields := log.Fields{
		"task_id":    t.GetID(),
		"command_id": t.GetCommandID(),
	}
	if err != nil {
		fields["error"] = err
	}

	applog.WithComponentAndFields("task.executor", fields).Log(level, message)
}
