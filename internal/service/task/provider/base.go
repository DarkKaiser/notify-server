package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

const (
	// notifySendTimeout 알림 발송 한 건의 전체 처리 흐름에 허용되는 최대 시간입니다.
	//
	// 알림 발송은 두 단계로 이루어집니다:
	//   1. Enqueue: Notify()가 내부 채널에 요청을 넣는 단계 (빠름)
	//   2. Send: 백그라운드 워커가 채널에서 꺼내 텔레그램 API를 실제로 호출하는 단계
	//
	// 이 타임아웃은 두 단계를 합산한 전체 시간을 기준으로 합니다.
	// 텔레그램 HTTP Client 타임아웃(70초)보다 짧게 설정하여, 발송이 완료된 직후
	// 컨텍스트가 자연스럽게 해제될 수 있도록 합니다.
	notifySendTimeout = 60 * time.Second
)

const (
	// notifyTaskExecutionFailed 작업 실행 중 발생한 모든 에러 상황에 대해 사용자에게 전송되는 알림 메시지의 공통 헤더입니다.
	//
	// 이 메시지 뒤에는 항상 구체적인 에러 원인(reason)이 결합되어 전송되므로,
	// 사용자가 어떤 문제로 실패했는지 쉽게 파악할 수 있도록 돕습니다.
	notifyTaskExecutionFailed = "작업 실행 중 오류가 발생하였습니다.😱"

	// notifySnapshotSaveFailedFormat 작업 실행 완료 후, 새로운 작업 결과(Snapshot)를 Storage에 저장하는 과정에서
	// 오류가 발생하였을 때 사용되는 알림 메시지 포맷입니다.
	//
	// 이 메시지는 비즈니스 로직은 성공했으나 데이터 영속화에 실패했음을 사용자에게 알립니다.
	notifySnapshotSaveFailedFormat = "작업 실행은 성공하였으나, 결과 데이터 저장에 실패하였습니다.😱\n\n☑ %s"

	// notifySnapshotLoadFailedFormat 이전 작업 실행 결과(Snapshot)를 Storage에서 불러오는 과정에서
	// 오류가 발생했을 때 사용자에게 전달되는 알림 메시지 포맷입니다.
	//
	// 작업 실행을 위한 초기 상태 복원에 실패했음을 의미하며, 주로 Storage 연결 문제나 데이터 손상이 원인입니다.
	notifySnapshotLoadFailedFormat = "이전 작업 결과 데이터를 불러오는 과정에서 오류가 발생하였습니다.😱\n\n☑ %s"

	// errMsgExecuteFuncNotInitialized Task의 핵심 비즈니스 로직(ExecuteFunc)이
	// 주입되지 않았을 때 발생하는 개발자 대상 에러 메시지입니다.
	//
	// ExecuteFunc는 Task가 수행해야 할 구체적인 작업(스크래핑, 데이터 가공 등)을 정의하며,
	// 이 에러는 Task 생성 시점의 의존성 주입 누락(개발자 실수)을 의미합니다.
	errMsgExecuteFuncNotInitialized = "Execute()가 초기화되지 않았습니다"

	// errMsgScraperNotInitialized 웹 스크래핑 기능이 필요한 Task(RequireScraper=true)임에도 불구하고
	// Scraper 의존성이 주입되지 않았을 때 발생하는 개발자 대상 에러 메시지입니다.
	//
	// 이 에러는 Task 생성 시점의 의존성 주입 누락(개발자 실수)을 의미합니다.
	errMsgScraperNotInitialized = "Scraper가 초기화되지 않았습니다"

	// errMsgStorageNotInitialized 작업 결과(Snapshot)를 저장하거나 불러오기 위해 필요한
	// Storage 의존성이 주입되지 않았을 때 발생하는 개발자 대상 에러 메시지입니다.
	//
	// Snapshot을 통해 상태를 관리하는 Task는 반드시 Storage 구현체가 필요하며,
	// 이 에러는 Task 생성 시점의 의존성 주입 누락(개발자 실수)을 의미합니다.
	errMsgStorageNotInitialized = "Storage가 초기화되지 않았습니다"

	// errMsgSnapshotCreationFailed 작업 결과를 담을 Snapshot 객체 생성(NewSnapshot)이
	// 실패했을 때 발생하는 개발자 대상 에러 메시지입니다.
	//
	// NewSnapshot 팩토리 함수가 nil을 반환하는 경우에 사용되며, 이는 팩토리 함수의 구현 오류(버그)를 의미합니다.
	errMsgSnapshotCreationFailed = "작업 결과 객체(Snapshot) 생성에 실패했습니다 (nil 반환)"
)

// Base 개별 Task의 실행 단위이자 상태를 관리하는 핵심 구조체입니다.
//
// 이 구조체는 Task의 정의(무엇을, 어떻게)와 실행 상태(언제, 얼마나)를 모두 캡슐화하며,
// Service 레이어에 의해 생성되고 생명주기가 관리됩니다.
type Base struct {
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 식별자
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// id Task의 고유 식별자입니다.
	id contract.TaskID

	// commandID Task 내에서 실행할 Command의 고유 식별자입니다.
	commandID contract.TaskCommandID

	// instanceID 실행된 Task 인스턴스의 고유 식별자입니다.
	instanceID contract.TaskInstanceID

	// notifierID 작업 완료 시 알림을 전송할 대상 채널의 고유 식별자입니다.
	notifierID contract.NotifierID

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 실행 제어
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// canceled 작업 취소 여부를 나타내는 플래그입니다.
	// atomic.Bool을 사용하여 여러 고루틴에서 안전하게 접근 가능합니다.
	canceled atomic.Bool

	// ctxCancel Run() 실행 중 컨텍스트를 취소하기 위한 함수입니다.
	// Run() 시작 시 설정되고 종료 시 nil로 초기화됩니다.
	ctxCancel context.CancelFunc

	// ctxCancelMu ctxCancel 필드에 대한 동시 접근을 보호하는 뮤텍스입니다.
	ctxCancelMu sync.Mutex

	// requireScraper 이 Task가 웹 스크래핑을 필요로 하는지 여부를 나타냅니다.
	// true인 경우 scraper 필드가 반드시 초기화되어야 합니다.
	requireScraper bool

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 실행 메타데이터
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// runBy Task의 실행 주체를 나타냅니다.
	// (예: RunByUser - 사용자 수동 실행, RunByScheduler - 스케줄러 자동 실행)
	runBy contract.TaskRunBy

	// startedAt Task 실행 시작 시각입니다.
	// startedAtMu에 의해 보호되며, Elapsed() 메서드에서 경과 시간 계산에 사용됩니다.
	startedAt time.Time

	// startedAtMu startedAt 필드에 대한 동시 접근을 보호하는 읽기/쓰기 뮤텍스입니다.
	startedAtMu sync.RWMutex

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 비즈니스 로직 및 의존성
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// execute 실제 비즈니스 로직(스크래핑, 가격 비교 등)을 수행하는 함수입니다.
	// SetExecute() 메서드를 통해 개별 Task 구현체에서 주입됩니다.
	execute ExecuteFunc

	// scraper 웹 요청(HTTP) 및 HTML/JSON 파싱을 수행하는 컴포넌트입니다.
	// requireScraper가 true인 경우에만 초기화됩니다.
	scraper scraper.Scraper

	// storage 작업 결과 데이터(Snapshot)를 영구 저장하고 불러오는 인터페이스입니다.
	// 이전 실행 결과 조회(Load)와 새로운 결과 저장(Save)에 사용됩니다.
	storage contract.TaskResultStore

	// newSnapshot 작업 결과 데이터의 빈 인스턴스를 생성하는 팩토리 함수입니다.
	// Storage.Load() 호출 시 데이터를 담을 구조체를 생성하는 데 사용됩니다.
	newSnapshot NewSnapshotFunc

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 유틸리티
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// logger 고정 필드(task_id, command_id 등)가 바인딩된 로거 인스턴스입니다.
	// 생성 시점에 초기화하여 로깅 시 매번 필드를 복사하는 오버헤드를 방지합니다.
	logger *applog.Entry
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ Task = (*Base)(nil)

// baseParams Base 구조체 초기화에 필요한 매개변수들을 그룹화한 구조체입니다.
//
// 설계 목적:
//   - Base 구조체 초기화에 필요한 매개변수들을 하나의 구조체로 묶어 함수 시그니처를 간결하게 유지합니다.
//   - 향후 Base 구조체 필드 추가 시 기존 호출 코드를 수정하지 않아도 되는 확장성을 제공합니다.
//   - 필드명을 통해 각 매개변수의 의미를 명확히 전달하여 가독성을 높입니다.
type baseParams struct {
	ID         contract.TaskID
	CommandID  contract.TaskCommandID
	InstanceID contract.TaskInstanceID
	NotifierID contract.NotifierID

	RequireScraper bool

	RunBy contract.TaskRunBy

	Scraper     scraper.Scraper
	Storage     contract.TaskResultStore
	NewSnapshot NewSnapshotFunc
}

// newBase baseParams를 받아 Base 인스턴스를 생성하는 내부 팩토리 함수입니다.
//
// 이 함수는 패키지 내부에서만 사용되며, 외부에서는 NewBase() 함수를 통해 간접적으로 호출됩니다.
// Base 구조체의 모든 필드를 초기화하며, 특히 logger는 생성 시점에 고정 필드를 바인딩하여
// 이후 로깅 시 매번 필드를 복사하는 오버헤드를 방지합니다.
//
// 매개변수:
//   - p: Base 초기화에 필요한 모든 매개변수를 담은 구조체
//
// 반환값: 완전히 초기화된 Base 인스턴스 포인터
func newBase(p baseParams) *Base {
	return &Base{
		id:         p.ID,
		commandID:  p.CommandID,
		instanceID: p.InstanceID,
		notifierID: p.NotifierID,

		requireScraper: p.RequireScraper,

		runBy: p.RunBy,

		scraper:     p.Scraper,
		storage:     p.Storage,
		newSnapshot: p.NewSnapshot,

		logger: applog.WithFields(applog.Fields{
			"task_id":         p.ID,
			"command_id":      p.CommandID,
			"instance_id":     p.InstanceID,
			"notifier_id":     p.NotifierID,
			"require_scraper": p.RequireScraper,
			"run_by":          p.RunBy,
		}),
	}
}

// NewBase NewTaskParams를 기반으로 Base 인스턴스를 생성하는 공개 팩토리 함수입니다.
//
// 이 함수는 개별 Task 구현체(kurly, naver, lotto 등)의 NewTask 메서드에서 호출되며,
// 반복적으로 나타나는 Base 초기화 코드를 간소화하여 코드 중복을 방지합니다.
// NewTaskParams를 내부 baseParams로 변환하고, 필요 시 Scraper를 초기화한 후
// newBase() 내부 팩토리 함수를 호출하여 완전히 초기화된 Base 인스턴스를 반환합니다.
//
// 매개변수:
//   - p: Task 생성에 필요한 모든 매개변수를 담은 구조체
//   - requireScraper: 웹 스크래핑 기능 활성화 여부
//     (true로 설정 시 p.Fetcher가 필수이며, nil일 경우 패닉 발생)
//
// 반환값: 완전히 초기화된 Base 인스턴스 포인터
func NewBase(p NewTaskParams, requireScraper bool) *Base {
	if p.Request == nil {
		panic("NewBase: params.Request는 필수입니다")
	}

	// 웹 스크래핑이 필요한 Task인 경우, Fetcher 의존성을 검증하고 Scraper를 생성합니다.
	var scr scraper.Scraper
	if requireScraper {
		if p.Fetcher == nil {
			panic(fmt.Sprintf("NewBase: 스크래핑 작업에는 Fetcher 주입이 필수입니다 (TaskID=%s)", p.Request.TaskID))
		}

		scr = scraper.New(p.Fetcher)
	}

	return newBase(baseParams{
		ID:         p.Request.TaskID,
		CommandID:  p.Request.CommandID,
		InstanceID: p.InstanceID,
		NotifierID: p.Request.NotifierID,

		RequireScraper: requireScraper,

		RunBy: p.Request.RunBy,

		Scraper:     scr,
		Storage:     p.Storage,
		NewSnapshot: p.NewSnapshot,
	})
}

func (b *Base) ID() contract.TaskID {
	return b.id
}

func (b *Base) CommandID() contract.TaskCommandID {
	return b.commandID
}

func (b *Base) InstanceID() contract.TaskInstanceID {
	return b.instanceID
}

func (b *Base) NotifierID() contract.NotifierID {
	return b.notifierID
}

func (b *Base) Cancel() {
	b.canceled.Store(true)

	// 현재 실행 중(Run 메서드가 호출된 상태)이라면, 할당된 컨텍스트를 즉시 취소하여
	// 진행 중인 비즈니스 로직(네트워크 요청, 데이터 처리 등)이 조속히 중단되도록 유도합니다.
	b.ctxCancelMu.Lock()
	if b.ctxCancel != nil {
		b.ctxCancel()
	}
	b.ctxCancelMu.Unlock()
}

func (b *Base) IsCanceled() bool {
	return b.canceled.Load()
}

func (b *Base) RunBy() contract.TaskRunBy {
	return b.runBy
}

func (b *Base) Elapsed() time.Duration {
	b.startedAtMu.RLock()
	defer b.startedAtMu.RUnlock()

	// 작업이 실행된 적이 없거나 시작 시각이 기록되지 않은 경우 0을 반환합니다.
	if b.startedAt.IsZero() {
		return 0
	}

	// 현재 시각과 시작 시각의 차이를 계산하여 경과 시간을 반환합니다.
	return time.Since(b.startedAt)
}

func (b *Base) SetExecute(execute ExecuteFunc) {
	b.execute = execute
}

func (b *Base) Scraper() scraper.Scraper {
	return b.scraper
}

// Run Task의 전체 생명주기를 관리하는 메인 진입점입니다.
//
// 이 메서드는 Task 인터페이스의 핵심 메서드로서, Service 레이어에서 호출되어
// Task의 생성부터 종료까지의 전체 생명주기를 제어합니다.
//
// 매개변수:
//   - ctx: Task 실행의 생명주기를 제어하는 컨텍스트 (타임아웃, 취소 신호 전파)
//   - notificationSender: 작업 결과를 사용자에게 알리기 위한 인터페이스 구현체 (필수)
func (b *Base) Run(ctx context.Context, notificationSender contract.NotificationSender) {
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 사전 검증: 필수 의존성 확인 및 조기 취소 감지
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// notificationSender는 Task 작업 결과를 사용자에게 알리기 위한 필수 의존성입니다.
	// nil인 경우 작업을 수행할 수 없으므로 즉시 종료합니다.
	if notificationSender == nil {
		b.Log(component, applog.ErrorLevel, "작업 실행 중단: NotificationSender 의존성 누락", nil, nil)
		return
	}

	// Run() 호출 전에 이미 Cancel()이 호출된 경우를 감지합니다.
	// 이는 스케줄러가 Task를 큐에 넣었지만, 실행 전에 사용자가 취소한 경우 등에 해당합니다.
	// 조기 종료(Early Exit)를 통해 불필요한 리소스 사용을 방지합니다.
	if b.IsCanceled() {
		b.Log(component, applog.InfoLevel, "작업 실행 중단: 컨텍스트 취소 (시작 전)", nil, nil)
		return
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 컨텍스트 설정: 취소 가능한 컨텍스트 생성 및 생명주기 관리
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// 상위 컨텍스트를 래핑하여 이 Task 전용 취소 가능한 컨텍스트를 생성합니다.
	// 이를 통해 Cancel() 메서드 호출 시 진행 중인 모든 하위 작업(네트워크 요청, DB 쿼리 등)에
	// 즉시 취소 신호를 전파할 수 있습니다.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// cancel 함수를 Base 구조체에 저장하여 외부(Cancel 메서드)에서 접근 가능하도록 합니다.
	b.ctxCancelMu.Lock()
	b.ctxCancel = cancel
	b.ctxCancelMu.Unlock()

	// Run() 종료 시 cancel 함수 참조를 정리하여 메모리 누수를 방지합니다.
	// 이후 Cancel() 호출은 이미 종료된 Task에 영향을 주지 않습니다.
	defer func() {
		b.ctxCancelMu.Lock()
		defer b.ctxCancelMu.Unlock()

		b.ctxCancel = nil
	}()

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 패닉 복구: 예상치 못한 패닉 발생 시 시스템 안정성 유지
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// Task 실행 중 발생할 수 있는 모든 패닉을 복구하여 전체 서비스가 중단되지 않도록 보호합니다.
	// 패닉이 발생하더라도 다음 두 가지 핵심 작업을 수행합니다:
	//   1. 상세한 패닉 정보를 로그에 기록하여 디버깅 가능하도록 함
	//   2. 사용자에게 에러 알림을 전송하여 작업 실패를 인지할 수 있도록 함
	defer func() {
		if r := recover(); r != nil {
			// 1단계: 패닉 정보를 즉시 로그에 기록합니다.
			// 로깅은 가장 안전한 작업이므로 최우선으로 수행하여 패닉 원인을 보존합니다.
			b.Log(component, applog.ErrorLevel, "작업 실행 중단: 런타임 패닉 발생", newErrRuntimePanic(r, b.id, b.commandID), applog.Fields{
				"panic": r,
			})

			// 2단계: 사용자에게 에러 알림을 전송합니다.
			// 알림 전송 자체가 패닉을 일으킬 수 있으므로, 별도의 익명 함수로 격리하고
			// 내부에 또 다른 recover를 설치하여 2차 패닉을 방지합니다.
			func() {
				defer func() {
					if r2 := recover(); r2 != nil {
						// 2차 패닉이 발생한 경우, 로그만 기록하고 더 이상 알림을 시도하지 않습니다.
						// 이는 무한 재귀를 방지하고 시스템 안정성을 최우선으로 합니다.
						b.Log(component, applog.ErrorLevel, "알림 처리 중단: 패닉 복구 중 2차 패닉 발생", nil, applog.Fields{
							"secondary_panic": r2,
						})
					}
				}()

				if notificationSender != nil && b != nil {
					// [컨텍스트 설계 의도]
					// Notify()는 비동기 함수로, 실제 발송은 백그라운드 워커가 담당합니다.
					//
					// 워커는 이 함수가 반환된 이후에도 notifyCtx를 계속 사용하므로,
					// defer cancel()을 호출하면 워커가 발송을 시도하기도 전에 컨텍스트가 취소되어 버립니다.
					//
					// 따라서 cancel()을 명시적으로 호출하지 않고, notifySendTimeout(60초) 경과 시
					// 컨텍스트가 자동으로 해제되도록 합니다.
					notifyCtx, _ := context.WithTimeout(context.WithoutCancel(ctx), notifySendTimeout)

					// 사용자에게 전달할 패닉 메시지를 생성합니다.
					message := b.formatTaskErrorMessage(fmt.Sprintf("시스템 내부 오류(Panic)가 발생하였습니다.\n\n[오류 상세 내용]\n%v", r))

					_ = notificationSender.Notify(notifyCtx, b.newNotification(message, true))
				}
			}()
		}
	}()

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 실행 시작 시각 기록
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// Task 실행 시작 시각을 기록하여 Elapsed() 메서드에서 경과 시간을 계산할 수 있도록 합니다.
	b.startedAtMu.Lock()
	b.startedAt = time.Now()
	b.startedAtMu.Unlock()

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 1단계: 실행 준비 - 의존성 검증 및 이전 작업 결과 로딩
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// prepareExecution()은 다음 작업을 수행합니다:
	//   - execute, scraper, storage 등 필수 의존성이 초기화되었는지 검증
	//   - Storage에서 이전 작업 결과(Snapshot)를 로딩하여 비교 기반 작업 지원
	// 검증 실패 시 에러를 반환하며, 이 경우 비즈니스 로직을 실행하지 않고 종료합니다.
	previousSnapshot, err := b.prepareExecution(ctx, notificationSender)
	if err != nil {
		return
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 취소 확인 #2: 준비 작업 완료 후, 실행 직전
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// prepareExecution() 중에 취소 요청이 들어온 경우를 감지합니다.
	// Storage Load 등의 준비 작업은 완료되었지만, 무거운 비즈니스 로직(execute)을
	// 실행하기 전에 취소 상태를 확인하여 불필요한 CPU/네트워크 리소스 사용을 방지합니다.
	if b.IsCanceled() {
		b.Log(component, applog.InfoLevel, "작업 실행 중단: 컨텍스트 취소 (준비 완료 후)", nil, nil)
		return
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 2단계: 비즈니스 로직 실행
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// execute 함수를 호출하여 실제 작업(웹 스크래핑, 데이터 가공, 비교 등)을 수행합니다.
	//
	// 반환값:
	//   - message: 사용자에게 전송할 알림 메시지 (성공 시)
	//   - newSnapshot: 저장할 새로운 작업 결과 데이터 (nil이면 저장 생략)
	//   - err: 실행 중 발생한 에러 (nil이면 성공)
	message, newSnapshot, err := b.execute(ctx, previousSnapshot, notificationSender.SupportsHTML(b.notifierID))

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 취소 확인 #3: 비즈니스 로직 실행 완료 후, 결과 처리 직전
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// execute 실행 중에 취소 요청이 들어온 경우를 감지합니다.
	// 이미 작업은 완료되었지만, Snapshot 저장이나 알림 전송을 수행하지 않고 종료하여
	// 취소된 작업의 결과가 사용자에게 전달되지 않도록 합니다.
	if b.IsCanceled() {
		b.Log(component, applog.InfoLevel, "결과 처리 중단: 컨텍스트 취소 (실행 완료 후)", nil, nil)
		return
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 3단계: 결과 처리 - Snapshot 저장 및 사용자 알림 전송
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// finalizeExecution()는 execute의 반환값을 기반으로 다음 작업을 수행합니다:
	//   - 에러 발생 시: 에러 로깅 및 사용자에게 에러 알림 전송
	//   - 성공 시: Snapshot 저장 → 사용자에게 성공 알림 전송
	b.finalizeExecution(ctx, notificationSender, message, newSnapshot, err)
}

// prepareExecution Task 실행 전 필수 조건을 검증하고 이전 작업 결과 Snapshot을 준비하는 사전 검증 단계입니다.
//
// 이 메서드는 Run() 메서드에서 비즈니스 로직(execute)을 실행하기 전에 호출되며,
// 다음 두 가지 핵심 역할을 수행합니다:
//
//  1. 필수 의존성 검증
//     - execute 함수가 초기화되었는지 확인
//     - 웹 스크래핑이 필요한 Task인 경우 Scraper 인스턴스가 준비되었는지 확인
//
//  2. 이전 작업 결과(Snapshot) 로딩
//     - Snapshot 생성 팩토리 함수(newSnapshot)가 등록된 경우, Storage에서 이전 작업 결과를 로드
//
// 검증 실패 시 에러 로그를 기록하고 사용자에게 알림을 전송한 후 즉시 에러를 반환하여
// 불완전한 상태에서의 실행을 방지합니다.
//
// 매개변수:
//   - ctx: 작업 실행 컨텍스트 (취소 신호 전파 용도)
//   - notificationSender: 검증 실패 시 에러 알림 전송을 담당하는 인터페이스 구현체
//
// 반환값:
//   - any: 이전 작업 결과 Snapshot (최초 실행이거나 newSnapshot이 nil인 경우 nil)
//   - error: 검증 실패 또는 Snapshot 로딩 실패 시 에러
func (b *Base) prepareExecution(ctx context.Context, notificationSender contract.NotificationSender) (any, error) {
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 1단계: 필수 의존성 검증
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	// 비즈니스 로직 실행 함수(execute)가 초기화되었는지 검증합니다.
	// execute는 개별 Task 구현체에서 SetExecute()를 통해 주입되어야 하며, 이 함수가 없으면 Task는 실질적인 작업을 수행할 수 없습니다.
	if b.execute == nil {
		message := b.formatTaskErrorMessage(errMsgExecuteFuncNotInitialized)

		b.Log(component, applog.ErrorLevel, "작업 준비 실패: ExecuteFunc 의존성 누락", nil, applog.Fields{
			"notification_message": message,
		})

		b.sendErrorNotification(ctx, notificationSender, message)

		return nil, newErrExecuteFuncNotInitialized(b.id, b.commandID)
	}

	// 웹 스크래핑이 필요한 Task인 경우, Scraper가 초기화되었는지 검증합니다.
	if b.requireScraper && b.scraper == nil {
		message := b.formatTaskErrorMessage(errMsgScraperNotInitialized)

		b.Log(component, applog.ErrorLevel, "작업 준비 실패: Scraper 의존성 누락", nil, applog.Fields{
			"notification_message": message,
		})

		b.sendErrorNotification(ctx, notificationSender, message)

		return nil, newErrScraperNotInitialized(b.id, b.commandID)
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 2단계: 이전 작업 결과(Snapshot) 로딩
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

	var snapshot any

	// Snapshot 생성 팩토리 함수(newSnapshot)가 등록된 경우, 이전 작업 결과를 로딩합니다.
	if b.newSnapshot != nil {
		// 비어있는 Snapshot 인스턴스를 생성합니다.
		snapshot = b.newSnapshot()

		// Snapshot 생성 실패는 팩토리 함수의 구현 오류를 의미하므로 즉시 실패 처리합니다.
		if snapshot == nil {
			message := b.formatTaskErrorMessage(errMsgSnapshotCreationFailed)

			b.Log(component, applog.ErrorLevel, "작업 준비 실패: Snapshot 생성 실패", nil, applog.Fields{
				"notification_message": message,
			})

			b.sendErrorNotification(ctx, notificationSender, message)

			return nil, newErrSnapshotCreationFailed(b.id, b.commandID)
		}

		// Snapshot 인스턴스가 생성되었으므로, 데이터를 로드하기 위한 Storage가 반드시 초기화되어야 합니다.
		if b.storage == nil {
			message := b.formatTaskErrorMessage(errMsgStorageNotInitialized)

			b.Log(component, applog.ErrorLevel, "작업 준비 실패: Storage 의존성 누락", nil, applog.Fields{
				"notification_message": message,
			})

			b.sendErrorNotification(ctx, notificationSender, message)

			return nil, newErrStorageNotInitialized(b.id, b.commandID)
		}

		// Storage에서 이전 작업 결과를 로드합니다.
		if err := b.storage.Load(b.ID(), b.CommandID(), snapshot); err != nil {
			// 최초 실행 시에는 이전 작업 결과가 존재하지 않으므로, ErrTaskResultNotFound 에러는 정상적인 상황으로 간주합니다.
			// 따라서 에러로 처리하지 않고 로그만 남긴 후 작업을 계속 진행합니다.
			if errors.Is(err, contract.ErrTaskResultNotFound) {
				b.Log(component, applog.InfoLevel, "스냅샷 로딩 생략: 저장된 데이터 없음 (최초 실행)", nil, nil)
			} else {
				// 그 외의 에러(파일 시스템 오류, 역직렬화 실패 등)는 실제 문제이므로 에러 로그를 기록하고
				// 사용자에게 에러 알림을 전송한 후 실행을 중단합니다.
				message := fmt.Sprintf(notifySnapshotLoadFailedFormat, err)

				b.Log(component, applog.ErrorLevel, "작업 준비 실패: 이전 스냅샷 로딩 에러", err, applog.Fields{
					"notification_message": message,
				})

				// 사용자가 명시적으로 작업을 취소하거나(Canceled), 작업이 타임아웃된 경우(DeadlineExceeded)에는
				// 불필요한 알림 소음을 방지하기 위해 에러 알림 전송을 생략합니다.
				if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
					b.sendErrorNotification(ctx, notificationSender, message)
				}

				return nil, newErrSnapshotLoadingFailed(err, b.id, b.commandID)
			}
		}
	}

	return snapshot, nil
}

// finalizeExecution 비즈니스 로직(execute) 실행 후 반환된 결과를 처리하고 사용자에게 알림을 전송합니다.
//
// 이 메서드는 Task 실행의 마지막 단계로서, execute 함수가 반환한 결과(성공/실패)에 따라
// 다음 세 가지 핵심 작업을 순차적으로 수행합니다:
//
//  1. 에러 처리: execute 실행 중 발생한 에러를 로깅하고 사용자에게 에러 알림 전송
//  2. 상태 저장: 새로운 작업 결과(Snapshot)를 Storage에 영구 저장
//  3. 성공 알림: 모든 과정이 성공한 경우에만 사용자에게 성공 메시지 전송
//
// 매개변수:
//   - ctx: 알림 전송 및 저장 작업의 컨텍스트 (취소 신호 전파 용도)
//   - notificationSender: 사용자에게 알림 전송을 담당하는 인터페이스 구현체
//   - message: execute가 생성한 사용자 대상 알림 메시지 (성공 시 전송될 내용)
//   - newSnapshot: execute가 생성한 새로운 작업 결과 데이터 (nil이면 저장 생략)
//   - err: execute 실행 중 발생한 에러 (nil이면 성공으로 간주)
func (b *Base) finalizeExecution(ctx context.Context, notificationSender contract.NotificationSender, message string, newSnapshot any, err error) {
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 1단계: 비즈니스 로직(execute) 실행 에러 처리
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// execute 함수가 에러를 반환한 경우, 작업 실행이 실패했음을 의미합니다.
	// 이 경우 Snapshot 저장이나 성공 알림을 시도하지 않고 즉시 에러 처리 후 종료합니다.
	if err != nil {
		// 사용자에게 전송할 에러 알림 메시지를 생성합니다.
		notifyMsg := b.formatTaskErrorMessage(err)

		// execute가 에러와 함께 부가 정보(message)를 반환한 경우, 이를 에러 알림 메시지에 추가하여 사용자에게 더 많은 컨텍스트를 제공합니다.
		if len(message) > 0 {
			notifyMsg = fmt.Sprintf("%s\n\n%s", notifyMsg, message)
		}

		b.Log(component, applog.ErrorLevel, "작업 실행 실패: 비즈니스 로직(execute) 에러", err, applog.Fields{
			"notification_message": notifyMsg,
		})

		// 사용자가 명시적으로 작업을 취소하거나(Canceled), 작업이 타임아웃된 경우(DeadlineExceeded)에는
		// 불필요한 알림 소음을 방지하기 위해 에러 알림 전송을 생략합니다.
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			b.sendErrorNotification(ctx, notificationSender, notifyMsg)
		}

		return
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 2단계: 작업 결과(Snapshot) 저장
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// execute가 성공적으로 실행되었고 새로운 Snapshot을 반환한 경우,
	// 이를 Storage에 저장하여 다음 실행 시 참조할 수 있도록 영속화합니다.
	if newSnapshot != nil {
		if b.storage != nil {
			if saveErr := b.storage.Save(b.ID(), b.CommandID(), newSnapshot); saveErr != nil {
				// 비즈니스 로직(execute)은 성공했으므로, Snapshot 저장 실패와 무관하게 중요 정보(message)는
				// 사용자에게 반드시 전달되어야 합니다. 저장 실패로 인해 사용자가 중요한 비즈니스 결과를
				// 놓치는 일을 방지하기 위해 에러 알림에 실행 결과를 포함하여 전송합니다.
				notifyMsg := fmt.Sprintf(notifySnapshotSaveFailedFormat, saveErr)
				if len(message) > 0 {
					notifyMsg = fmt.Sprintf("%s\n\n[작업 실행 결과 상세]\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n%s", notifyMsg, message)
				}

				b.Log(component, applog.ErrorLevel, "스냅샷 저장 실패: Storage 저장 에러", saveErr, applog.Fields{
					"notification_message": notifyMsg,
				})

				b.sendErrorNotification(ctx, notificationSender, notifyMsg)

				return
			}
		} else {
			// Task가 작업 결과(Snapshot)를 생성했으나 저장할 Storage가 없는 것은 명백한 설정 오류(버그)입니다.
			// 개발 단계에서 검출되어야 할 문제이나, 만약 프로덕션 환경에서 발생할 경우를 대비하여
			// 사용자에게 에러를 알리고 운영자가 신속히 인지할 수 있도록 로그를 남깁니다.
			notifyMsg := b.formatTaskErrorMessage(errMsgStorageNotInitialized)
			if len(message) > 0 {
				notifyMsg = fmt.Sprintf("%s\n\n[작업 실행 결과 상세]\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n%s", notifyMsg, message)
			}

			b.Log(component, applog.ErrorLevel, "스냅샷 저장 실패: Storage 의존성 누락", nil, applog.Fields{
				"notification_message": notifyMsg,
			})

			b.sendErrorNotification(ctx, notificationSender, notifyMsg)

			return
		}
	}

	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 3단계: 성공 알림 전송
	// ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
	// 모든 과정(execute 실행, Snapshot 저장)이 성공적으로 완료된 경우에만 사용자에게 성공 메시지를 전송합니다.
	//
	// message가 비어있는 경우는 execute가 의도적으로 알림을 생략한 것으로 간주하여
	// 알림을 전송하지 않습니다. (예: 변경 사항이 없는 경우)
	if len(message) > 0 {
		// [컨텍스트 설계 의도]
		// Notify()는 비동기 함수로, 실제 발송은 백그라운드 워커가 담당합니다.
		//
		// 워커는 이 함수가 반환된 이후에도 notifyCtx를 계속 사용하므로,
		// defer cancel()을 호출하면 워커가 발송을 시도하기도 전에 컨텍스트가 취소되어 버립니다.
		//
		// 따라서 cancel()을 명시적으로 호출하지 않고, notifySendTimeout(60초) 경과 시
		// 컨텍스트가 자동으로 해제되도록 합니다.
		notifyCtx, _ := context.WithTimeout(context.WithoutCancel(ctx), notifySendTimeout)

		// 알림 전송 실패는 로그로만 기록하고 Task 실행 자체는 성공으로 간주합니다.
		// 이는 "알림 실패가 비즈니스 로직 실패로 전파되지 않도록" 하는 설계 원칙을 따릅니다.
		if notifyErr := notificationSender.Notify(notifyCtx, b.newNotification(message, false)); notifyErr != nil {
			b.Log(component, applog.ErrorLevel, "성공 알림 발송 실패: 전송 에러", notifyErr, nil)
		}
	}
}

// sendErrorNotification 작업 실행 중 발생한 에러를 사용자에게 알림으로 전송합니다.
//
// 이 메서드는 Task 실행 과정에서 발생한 다양한 에러 상황(초기화 실패, 스크래핑 실패, 저장 실패 등)을
// 사용자에게 알리기 위해 내부적으로 사용됩니다. Base에 바인딩된 메타데이터(TaskID, CommandID, Elapsed 등)를
// 포함한 에러 알림 객체를 생성하고, NotificationSender를 통해 전송합니다.
//
// 알림 전송 자체가 실패하더라도 Task 실행 흐름을 중단하지 않으며, 대신 에러 로그를 기록하여
// 시스템 안정성을 유지합니다. 이는 "알림 실패가 비즈니스 로직 실패로 전파되지 않도록" 하는 설계 원칙을 따릅니다.
//
// 매개변수:
//   - ctx: 알림 전송 요청의 컨텍스트 (타임아웃, 취소 신호 전파 용도)
//   - notificationSender: 알림 전송을 담당하는 인터페이스 구현체
//   - message: 사용자에게 전달할 에러 메시지 본문
func (b *Base) sendErrorNotification(ctx context.Context, notificationSender contract.NotificationSender, message string) {
	// [컨텍스트 설계 의도]
	// Notify()는 비동기 함수로, 실제 발송은 백그라운드 워커가 담당합니다.
	//
	// 워커는 이 함수가 반환된 이후에도 notifyCtx를 계속 사용하므로,
	// defer cancel()을 호출하면 워커가 발송을 시도하기도 전에 컨텍스트가 취소되어 버립니다.
	//
	// 따라서 cancel()을 명시적으로 호출하지 않고, notifySendTimeout(60초) 경과 시
	// 컨텍스트가 자동으로 해제되도록 합니다.
	notifyCtx, _ := context.WithTimeout(context.WithoutCancel(ctx), notifySendTimeout)

	if err := notificationSender.Notify(notifyCtx, b.newNotification(message, true)); err != nil {
		b.Log(component, applog.ErrorLevel, "에러 알림 발송 실패: 전송 에러", err, nil)
	}
}

// newNotification Base의 상태 정보를 기반으로 새로운 Notification 객체를 생성합니다.
//
// 이 메서드는 Task 실행 중 발생한 이벤트(성공, 실패, 패닉 등)를 사용자에게 알리기 위한
// Notification 구조체를 초기화합니다. Base에 바인딩된 식별자(TaskID, CommandID 등)와
// 실행 메타데이터(Elapsed)를 자동으로 포함하여 일관된 알림 형식을 보장합니다.
//
// 매개변수:
//   - message: 사용자에게 전달할 알림 메시지 본문
//   - errorOccurred: 에러 발생 여부 (true: 실패 알림, false: 성공 알림)
func (b *Base) newNotification(message string, errorOccurred bool) contract.Notification {
	return contract.Notification{
		NotifierID: b.NotifierID(),

		TaskID:     b.ID(),
		CommandID:  b.CommandID(),
		InstanceID: b.InstanceID(),

		Message: message,
		Elapsed: b.Elapsed(),

		ErrorOccurred: errorOccurred,
		Cancelable:    false,
	}
}

// formatTaskErrorMessage 작업 실패 시 사용자에게 전송할 에러 메시지를 생성합니다.
//
// 이 메서드는 모든 Task 실행 에러에 대해 일관된 형식의 메시지를 생성하기 위해 사용됩니다.
// 공통 에러 헤더(notifyTaskExecutionFailed)에 구체적인 실패 원인(reason)을 결합하여,
// 사용자가 문제를 쉽게 파악할 수 있도록 합니다.
//
// 매개변수:
//   - reason: 작업 실패의 구체적인 원인 (문자열 또는 error 객체)
//
// 반환값:
//   - string: 공통 헤더와 실패 원인이 결합된 최종 에러 메시지
func (b *Base) formatTaskErrorMessage(reason any) string {
	return fmt.Sprintf("%s\n\n☑ %s", notifyTaskExecutionFailed, reason)
}

// Log 컴포넌트 이름, 에러, 추가 필드를 포함한 구조적 로깅을 수행합니다.
//
// 이 메서드는 Base 생성 시 바인딩된 기본 필드(task_id, command_id 등)에
// 추가적인 컨텍스트 정보를 덧붙여 로그를 기록합니다.
func (b *Base) Log(component string, level applog.Level, message string, err error, fields applog.Fields) {
	entry := b.logger.WithField("component", component)
	if err != nil {
		entry = entry.WithError(err)
	}
	if len(fields) > 0 {
		entry = entry.WithFields(fields)
	}
	entry.Log(level, message)
}
