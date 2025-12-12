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
	msgTaskExecutionFailed          = "작업 진행중 오류가 발생하여 작업이 실패하였습니다.😱"
	msgRunFnNotInitialized          = "runFn()이 초기화되지 않았습니다."
	msgTaskResultDataCreationFailed = "작업결과데이터 생성이 실패하였습니다."
	msgStorageNotInitialized        = "Storage가 초기화되지 않았습니다."
	msgPreviousDataLoadFailed       = "이전 작업결과데이터 로딩이 실패하였습니다.😱\n\n☑ %s\n\n빈 작업결과데이터를 이용하여 작업을 계속 진행합니다."
	msgCurrentDataSaveFailed        = "작업이 끝난 작업결과데이터의 저장이 실패하였습니다.😱\n\n☑ %s"
)

// TaskRunFunc
type TaskRunFunc func(interface{}, bool) (string, interface{}, error)

// Task 개별 작업의 실행 단위이자 상태를 관리하는 핵심 구조체입니다.
type Task struct {
	ID         ID
	CommandID  CommandID
	InstanceID InstanceID

	// NotifierID는 알림을 발송할 대상 메신저의 ID입니다. (예: "telegram")
	NotifierID string

	// Canceled는 작업 취소 여부를 나타내는 플래그입니다.
	Canceled bool

	// RunBy는 작업이 실행된 트리거 주체(스케줄러, 수동 실행 등)를 나타냅니다.
	RunBy RunBy
	// RunTime은 작업이 실제 실행을 시작한 시각입니다.
	RunTime time.Time

	// RunFn은 실제 비즈니스 로직을 수행하는 함수입니다.
	// 순수 함수(Pure Function)에 가깝게 구현되어야 하며, 외부 의존성(Storage 등)은 인자로 주입받습니다.
	RunFn TaskRunFunc

	// Storage는 작업의 이전 실행 결과를 저장하고 불러오는 인터페이스입니다.
	Storage TaskResultStorage

	// Fetcher는 웹 스크래핑 등을 수행하는 HTTP 클라이언트 추상화입니다.
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

// Run 메서드는 Task의 실행 수명 주기를 관리하는 메인 진입점입니다.
//
// 실행 흐름:
// 1. [준비] prepareExecution: 실행 함수(RunFn) 확인, 데이터 초기화, 이전 상태 로드
// 2. [실행] execute: 비즈니스 로직 수행 및 결과 생성
// 3. [처리] handleExecutionResult: 결과 저장 및 알림 발송
//
// 동시성 관리:
// - 고루틴 내에서 실행되며, taskStopWaiter를 통해 종료 시점을 동기화합니다.
// - 실행 완료 후 taskDoneC 채널로 InstanceID를 전송하여 완료를 알립니다.
func (t *Task) Run(taskCtx TaskContext, notificationSender NotificationSender, taskStopWaiter *sync.WaitGroup, taskDoneC chan<- InstanceID) {
	defer taskStopWaiter.Done()
	defer func() {
		taskDoneC <- t.InstanceID
	}()

	t.RunTime = time.Now()

	// 1. 사전 검증 및 데이터 준비
	taskResultData, err := t.prepareExecution(taskCtx, notificationSender)
	if err != nil {
		return
	}

	// 2. 작업 실행
	message, changedTaskResultData, err := t.execute(taskResultData, notificationSender)

	if t.IsCanceled() {
		return
	}

	// 3. 결과 처리
	t.handleExecutionResult(taskCtx, notificationSender, message, changedTaskResultData, err)
}

// prepareExecution 실행 전 필요한 조건을 검증하고 데이터를 준비합니다.
//
// 주요 역할:
// - RunFn 및 Storage 초기화 여부 확인 (Fail Fast)
// - 작업 결과 데이터(TaskResultData) 객체 생성
// - Storage에서 이전 실행 결과 로드 (상태 복원)
//
// 에러 처리:
// - 필수 조건 불충족 시 Error 레벨 로그 및 알림을 발송하고 에러를 반환합니다.
// - 이전 데이터 로드 실패 시에는 Warn 레벨 로그를 남기지만, 빈 데이터로 실행을 계속합니다.
func (t *Task) prepareExecution(taskCtx TaskContext, notificationSender NotificationSender) (interface{}, error) {
	if t.RunFn == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgRunFnNotInitialized)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgRunFnNotInitialized)
	}

	// TaskResultData를 초기화하고 읽어들인다.
	var taskResultData interface{}
	searchResult, cfgErr := findConfig(t.GetID(), t.GetCommandID())
	if cfgErr == nil {
		taskResultData = searchResult.Command.NewTaskResultDataFn()
	}
	if taskResultData == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgTaskResultDataCreationFailed)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgTaskResultDataCreationFailed)
	}

	// Storage가 초기화되지 않았을 경우에 대한 방어 로직
	if t.Storage == nil {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, msgStorageNotInitialized)
		t.log(log.ErrorLevel, message, nil)
		t.notifyError(taskCtx, notificationSender, message)
		return nil, apperrors.New(apperrors.ErrInternal, msgStorageNotInitialized)
	}

	err := t.Storage.Load(t.GetID(), t.GetCommandID(), taskResultData)
	if err != nil {
		message := fmt.Sprintf(msgPreviousDataLoadFailed, err)
		t.log(log.WarnLevel, message, err)
		t.notify(taskCtx, notificationSender, message)
	}

	return taskResultData, nil
}

// execute 실제 비즈니스 로직(RunFn)을 실행합니다.
//
// 매개변수:
// - taskResultData: prepareExecution에서 로드된 이전 상태 데이터
//
// 반환값:
// - string: 알림으로 보낼 결과 메시지
// - interface{}: 변경된(새로운) 상태 데이터 (저장 대상)
// - error: 실행 중 발생한 에러
func (t *Task) execute(taskResultData interface{}, notificationSender NotificationSender) (string, interface{}, error) {
	return t.RunFn(taskResultData, notificationSender.SupportsHTML(t.NotifierID))
}

// handleExecutionResult 작업 실행 결과를 처리합니다.
//
// 성공 시 (runErr == nil):
// - 결과 메시지가 있으면 알림 발송
// - 변경된 데이터가 있으면 Storage에 저장
// - 저장 실패 시 Warn 로그 및 Error 알림 (데이터 유실 가능성 경고)
//
// 실패 시 (runErr != nil):
// - Error 레벨 로그 및 알림 발송
func (t *Task) handleExecutionResult(taskCtx TaskContext, notificationSender NotificationSender, message string, changedTaskResultData interface{}, runErr error) {
	if runErr == nil {
		if len(message) > 0 {
			t.notify(taskCtx, notificationSender, message)
		}

		if changedTaskResultData != nil {
			if err := t.Storage.Save(t.GetID(), t.GetCommandID(), changedTaskResultData); err != nil {
				message := fmt.Sprintf(msgCurrentDataSaveFailed, err)
				t.log(log.WarnLevel, message, err)
				t.notifyError(taskCtx, notificationSender, message)
			}
		}
	} else {
		message := fmt.Sprintf("%s\n\n☑ %s", msgTaskExecutionFailed, runErr)
		t.log(log.ErrorLevel, message, runErr)
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
