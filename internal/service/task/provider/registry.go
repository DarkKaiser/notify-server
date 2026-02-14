package provider

import (
	"sync"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// TaskConfig Task 생성 및 Command 구성을 위한 메타데이터를 정의하는 구조체입니다.
//
// 역할 및 목적:
//   - Task의 "청사진(Blueprint)" 역할을 수행합니다.
//   - 하나의 Task가 수행할 수 있는 모든 Command 목록과 Task 생성 방법을 정의합니다.
//   - Registry에 등록되어 Task 실행 요청 시 참조됩니다.
//
// 불변성 보장:
//   - Registry 등록 시 Clone()을 통해 깊은 복사되어 저장됩니다.
//   - 등록 후에는 외부에서 원본을 수정하더라도 Registry 내부 상태에 영향을 주지 않습니다.
//   - 조회 시에도 Clone()을 통해 복사본을 반환하여 동시성 안전성을 보장합니다.
type TaskConfig struct {
	// Commands Task가 수행할 수 있는 모든 Command의 목록입니다.
	// 최소 1개 이상의 TaskCommandConfig가 필요하며, 각 Command는 독립적인 실행 단위입니다.
	Commands []*TaskCommandConfig

	// NewTask 새로운 Task 인스턴스를 생성하는 팩토리 함수입니다.
	NewTask NewTaskFunc
}

// Clone Task 설정을 깊은 복사(Deep Copy)하여 반환합니다.
//
// Registry에 등록된 원본 설정을 외부 변경으로부터 완벽하게 격리하기 위해 사용됩니다.
// 단순히 구조체만 복사하면 내부의 Commands 슬라이스가 원본과 메모리를 공유하게 되어,
// 복사본을 수정했을 때 원본 데이터까지 함께 변경되는 부작용(Side Effect)이 발생할 수 있습니다.
//
// 복사 방식:
//  1. TaskConfig 구조체 자체를 얕은 복사 (기본 필드값 복제)
//  2. Commands 슬라이스를 새로 할당 (메모리 공간 분리)
//  3. 각 CommandConfig 요소를 재귀적으로 복제 (내부 객체까지 완전 분리)
func (c *TaskConfig) Clone() *TaskConfig {
	if c == nil {
		return nil
	}

	copy := *c
	if c.Commands != nil {
		copy.Commands = make([]*TaskCommandConfig, len(c.Commands))
		for i, commandConfig := range c.Commands {
			copy.Commands[i] = commandConfig.Clone()
		}
	}

	return &copy
}

// Validate Task 설정의 무결성을 검증합니다.
func (c *TaskConfig) Validate() error {
	if len(c.Commands) == 0 {
		return ErrCommandConfigsEmpty
	}
	if c.NewTask == nil {
		return ErrNewTaskNil
	}

	seenCommandIDs := make(map[contract.TaskCommandID]bool)
	for _, commandConfig := range c.Commands {
		if commandConfig == nil {
			return ErrCommandConfigNil
		}
		if err := commandConfig.ID.Validate(); err != nil {
			return newErrInvalidCommandID(err, commandConfig.ID)
		}

		// 동일한 Task 내에서 Command ID가 중복되지 않는지 확인합니다.
		if seenCommandIDs[commandConfig.ID] {
			return newErrDuplicateCommandID(commandConfig.ID)
		}

		if commandConfig.NewSnapshot == nil {
			return ErrNewSnapshotNil
		}

		// NewSnapshot이 nil을 반환하는지 사전 검증하여 런타임 오류를 차단합니다.
		if snapshot := commandConfig.NewSnapshot(); snapshot == nil {
			return newErrSnapshotFactoryReturnedNil(commandConfig.ID)
		}

		seenCommandIDs[commandConfig.ID] = true
	}

	return nil
}

// TaskCommandConfig 개별 Command의 실행 정책과 데이터 관리 방식을 정의하는 구조체입니다.
type TaskCommandConfig struct {
	// ID Command의 고유 식별자입니다.
	ID contract.TaskCommandID

	// AllowMultiple 동일 Command의 동시 실행 허용 여부입니다.
	//   - true: 동일 Command를 여러 인스턴스가 동시에 병렬 실행 가능
	//   - false: 이미 실행 중이면 새 요청 거부 (중복 실행 방지)
	AllowMultiple bool

	// NewSnapshot Task 작업 결과 데이터의 빈 인스턴스를 생성하는 팩토리 함수입니다.
	NewSnapshot NewSnapshotFunc
}

// Clone Command 설정을 복제하여 반환합니다.
//
// 복사 방식:
//   - 이 구조체는 슬라이스나 맵 같은 가변 참조 타입을 포함하지 않습니다.
//   - 따라서 얕은 복사(Shallow Copy)만으로도 원본과 완전히 독립된 객체가 됩니다.
//   - NewSnapshot 함수는 불변이므로 포인터가 공유되어도 안전합니다.
func (c *TaskCommandConfig) Clone() *TaskCommandConfig {
	if c == nil {
		return nil
	}

	copy := *c

	return &copy
}

// ResolvedConfig Registry에서 조회된 Task와 Command 설정의 조합입니다.
//
// Task 실행 요청 시 FindConfig를 통해 반환되며, Task 인스턴스 생성에 필요한
// 모든 설정을 포함합니다.
type ResolvedConfig struct {
	Task    *TaskConfig
	Command *TaskCommandConfig
}

// Registry 등록된 모든 Task와 Command 설정을 관리하는 중앙 저장소입니다.
//
// 이 구조체는 애플리케이션 시작 시점에 모든 Task 설정을 등록받아 저장하고,
// Task 실행 요청 시 해당 설정을 조회하여 제공하는 역할을 합니다.
// 동시성 제어를 위해 RWMutex를 사용합니다.
type Registry struct {
	// configs Task ID를 키로 하는 설정 맵입니다.
	// 모든 값은 Clone된 불변 객체로 저장되어 외부 수정으로부터 보호됩니다.
	configs map[contract.TaskID]*TaskConfig

	// mu 동시성 제어를 위한 읽기/쓰기 락입니다.
	// 등록 시에는 쓰기 락, 조회 시에는 읽기 락을 사용합니다.
	mu sync.RWMutex
}

// defaultRegistry 전역에서 사용하는 기본 Registry 인스턴스입니다.
var defaultRegistry = newRegistry()

// newRegistry 새로운 Registry 인스턴스를 생성합니다.
func newRegistry() *Registry {
	return &Registry{
		configs: make(map[contract.TaskID]*TaskConfig),
	}
}

// MustRegister Task 설정을 Registry에 등록하며, 실패 시 패닉을 발생시킵니다.
//
// "Fail Fast" 원칙에 따라 애플리케이션 초기화 단계에서 잘못된 설정을 즉시 감지합니다.
// 주로 init() 함수나 main() 함수에서 호출되어 설정 오류를 사전에 차단합니다.
//
// 매개변수:
//   - taskID: 등록할 Task의 고유 식별자
//   - cfg: Task 설정 객체 (nil 불가)
func (r *Registry) MustRegister(taskID contract.TaskID, cfg *TaskConfig) {
	if err := r.Register(taskID, cfg); err != nil {
		panic(err.Error())
	}
}

// Register Task 설정을 Registry에 등록합니다.
//
// 매개변수:
//   - taskID: 등록할 Task의 고유 식별자
//   - cfg: Task 설정 객체 (nil 불가)
//
// 반환값:
//   - error: 유효성 검증 실패 또는 중복 ID 시 에러 반환, 성공 시 nil
func (r *Registry) Register(taskID contract.TaskID, cfg *TaskConfig) error {
	if err := taskID.Validate(); err != nil {
		return newErrInvalidTaskID(err, taskID)
	}

	if cfg == nil {
		return ErrTaskConfigNil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 외부에서 원본 설정을 수정하더라도 Registry 내부 상태에 영향을 주지 않도록 복제합니다.
	// 락(Lock)을 획득한 상태에서 복제하여 객체 오염을 방지합니다.
	clonedCfg := cfg.Clone()
	if err := clonedCfg.Validate(); err != nil {
		return err
	}

	// 중복 등록 방지
	if _, exists := r.configs[taskID]; exists {
		return newErrDuplicateTaskID(taskID)
	}

	r.configs[taskID] = clonedCfg

	// 로그 기록을 위해 등록된 모든 Command ID 목록을 수집합니다.
	supportedCommandIDs := make([]contract.TaskCommandID, len(clonedCfg.Commands))
	for i, commandConfig := range clonedCfg.Commands {
		supportedCommandIDs[i] = commandConfig.ID
	}

	applog.WithComponentAndFields(component, applog.Fields{
		"task_id":       taskID,
		"commands":      supportedCommandIDs,
		"command_count": len(supportedCommandIDs),
	}).Info("Task 설정 등록 성공: 유효성 검증 및 Registry 등록 완료")

	return nil
}

// RegisterForTest 유효성 검증 절차를 우회하여 Task 설정을 강제 등록합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
func (r *Registry) RegisterForTest(taskID contract.TaskID, cfg *TaskConfig) {
	if cfg == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 테스트 환경에서도 불변성을 보장하기 위해 Clone()하여 등록합니다.
	// 락(Lock)을 획득한 상태에서 복제하여 객체 오염을 방지합니다.
	clonedCfg := cfg.Clone()
	r.configs[taskID] = clonedCfg
}

// ClearForTest Registry에 등록된 모든 Task 설정을 제거합니다.
//
// 테스트 간 격리를 위해 각 테스트 시작 시점에 호출하여 깨끗한 상태를 보장합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
// 실행 중인 서버의 모든 Task 설정이 삭제되어 서비스 장애로 이어질 수 있습니다.
func (r *Registry) ClearForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configs = make(map[contract.TaskID]*TaskConfig)
}

// findConfig 주어진 Task ID와 Command ID에 해당하는 설정을 검색하는 내부 메서드입니다.
//
// 매칭 전략:
//  1. 정확한 일치(Exact Match) 우선 시도
//  2. 정확한 일치 실패 시 와일드카드 매칭 시도
//  3. 모두 실패 시 지원 가능한 Command ID 목록과 함께 에러 반환
//
// 매개변수:
//   - taskID: 검색할 Task의 고유 식별자
//   - commandID: 검색할 Command의 고유 식별자
//
// 반환값:
//   - *ResolvedConfig: 매칭된 Task와 Command 설정 (깊은 복사본)
//   - error: Task 또는 Command를 찾을 수 없는 경우
func (r *Registry) findConfig(taskID contract.TaskID, commandID contract.TaskCommandID) (*ResolvedConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskConfig, exists := r.configs[taskID]
	if !exists || taskConfig == nil {
		return nil, NewErrTaskNotSupported(taskID)
	}

	// 1. 정확한 매칭(Exact Match) 우선 시도
	var matchedIdx = -1
	for i, commandConfig := range taskConfig.Commands {
		if commandConfig.ID == commandID {
			matchedIdx = i
			break
		}
	}

	// 2. 와일드카드 매칭 시도
	if matchedIdx == -1 {
		for i, commandConfig := range taskConfig.Commands {
			if commandConfig.ID.Match(commandID) {
				matchedIdx = i
				break
			}
		}
	}

	// 매칭된 결과가 있으면 복제하여 반환합니다.
	if matchedIdx != -1 {
		clonedTaskConfig := taskConfig.Clone()

		return &ResolvedConfig{
			Task:    clonedTaskConfig,
			Command: clonedTaskConfig.Commands[matchedIdx],
		}, nil
	}

	// 지원 가능한 모든 Command ID 목록 수집 (에러 메시지용)
	supportedCommandIDs := make([]contract.TaskCommandID, len(taskConfig.Commands))
	for i, commandConfig := range taskConfig.Commands {
		supportedCommandIDs[i] = commandConfig.ID
	}

	return nil, NewErrCommandNotSupported(commandID, supportedCommandIDs)
}

// MustRegister Task 설정을 전역 Registry에 등록하며, 실패 시 패닉을 발생시킵니다.
//
// "Fail Fast" 원칙에 따라 애플리케이션 초기화 단계에서 잘못된 설정을 즉시 감지합니다.
// 주로 init() 함수나 main() 함수에서 호출되어 설정 오류를 사전에 차단합니다.
//
// 매개변수:
//   - taskID: 등록할 Task의 고유 식별자
//   - cfg: Task 설정 객체 (nil 불가)
func MustRegister(taskID contract.TaskID, cfg *TaskConfig) {
	defaultRegistry.MustRegister(taskID, cfg)
}

// Register Task 설정을 전역 Registry에 등록합니다.
//
// 매개변수:
//   - taskID: 등록할 Task의 고유 식별자
//   - cfg: Task 설정 객체 (nil 불가)
//
// 반환값:
//   - error: 유효성 검증 실패 또는 중복 ID 시 에러 반환, 성공 시 nil
func Register(taskID contract.TaskID, cfg *TaskConfig) error {
	return defaultRegistry.Register(taskID, cfg)
}

// RegisterForTest 유효성 검증 절차를 우회하여 Task 설정을 전역 Registry에 강제 등록합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
func RegisterForTest(taskID contract.TaskID, cfg *TaskConfig) {
	defaultRegistry.RegisterForTest(taskID, cfg)
}

// ClearForTest 전역 Registry에 등록된 모든 Task 설정을 제거합니다.
//
// 테스트 간 격리를 위해 각 테스트 시작 시점에 호출하여 깨끗한 상태를 보장합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
// 실행 중인 서버의 모든 Task 설정이 삭제되어 서비스 장애로 이어질 수 있습니다.
func ClearForTest() {
	defaultRegistry.ClearForTest()
}

// FindConfig 전역 Registry를 통해 주어진 Task ID와 Command ID에 해당하는 설정을 검색합니다.
//
// 매개변수:
//   - taskID: 검색할 Task의 고유 식별자
//   - commandID: 검색할 Command의 고유 식별자
//
// 반환값:
//   - *ResolvedConfig: 매칭된 Task와 Command 설정 (깊은 복사본)
//   - error: Task 또는 Command를 찾을 수 없는 경우
func FindConfig(taskID contract.TaskID, commandID contract.TaskCommandID) (*ResolvedConfig, error) {
	return defaultRegistry.findConfig(taskID, commandID)
}
