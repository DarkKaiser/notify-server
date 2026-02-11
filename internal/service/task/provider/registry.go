package provider

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/internal/config"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	applog "github.com/darkkaiser/notify-server/pkg/log"
)

// NewTaskFunc 새로운 Task 인스턴스를 생성하는 팩토리 함수 타입입니다.
type NewTaskFunc func(contract.TaskInstanceID, *contract.TaskSubmitRequest, *config.AppConfig, contract.TaskResultStore, fetcher.Fetcher, NewSnapshotFunc) (Task, error)

// NewSnapshotFunc Task 결과 데이터 구조체를 생성하는 팩토리 함수입니다.
type NewSnapshotFunc func() interface{}

// Config Task 생성 및 명령어(Command) 구성을 위한 메타데이터를 정의하는 불변(Immutable) 설정 객체입니다.
// 레지스트리에 등록된 이후에는 상태가 변경되지 않으며(Read-Only), Task 인스턴스를 생성하기 위한 청사진(Blueprint)으로 사용됩니다.
type Config struct {
	// Commands 이 Task가 수행할 수 있는 모든 하위 명령어(Command)의 정의 목록입니다.
	// Task는 최소 하나 이상의 CommandConfig를 포함해야 하며, 이를 통해 지원 가능한 기능의 범위를 결정합니다.
	Commands []*CommandConfig

	NewTask NewTaskFunc
}

// CommandConfig 개별 명령어(Command)에 대한 실행 정책과 결과 데이터 구조체를 생성하는 구조체입니다.
type CommandConfig struct {
	ID contract.TaskCommandID

	// AllowMultiple 동일 명령어의 중복 실행(Concurrency) 허용 여부입니다.
	// - true: 여러 인스턴스가 동시에 병렬 실행될 수 있습니다.
	// - false: 이미 실행 중인 인스턴스가 있다면 새로운 요청은 무시됩니다 (Throttling/Idempotency).
	AllowMultiple bool

	NewSnapshot NewSnapshotFunc
}

// ConfigLookup 요청된 ID(Task/Command)를 통해 Registry에서 조회된(Found) 설정 조합입니다.
type ConfigLookup struct {
	Task    *Config
	Command *CommandConfig
}

// Registry 등록된 모든 Task와 Command의 설정을 관리하는 중앙 저장소(Repository)입니다.
type Registry struct {
	configs map[contract.TaskID]*Config

	mu sync.RWMutex
}

// defaultRegistry 기본 Registry 인스턴스(Singleton)입니다.
var defaultRegistry = newRegistry()

// newRegistry 새로운 Registry 인스턴스를 생성합니다.
func newRegistry() *Registry {
	return &Registry{
		configs: make(map[contract.TaskID]*Config),
	}
}

// Validate 설정 객체(Config)의 무결성을 검증합니다.
func (c *Config) Validate() error {
	if len(c.Commands) == 0 {
		return apperrors.New(apperrors.InvalidInput, "Commands는 비어있을 수 없습니다")
	}
	if c.NewTask == nil {
		return apperrors.New(apperrors.InvalidInput, "NewTask는 nil일 수 없습니다")
	}

	seenCommands := make(map[contract.TaskCommandID]bool)
	for _, commandConfig := range c.Commands {
		if commandConfig.ID == "" {
			return apperrors.New(apperrors.InvalidInput, "CommandID는 비어있을 수 없습니다")
		}
		// 명령어 ID 중복 검사
		if seenCommands[commandConfig.ID] {
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("중복된 CommandID입니다: %s", commandConfig.ID))
		}
		if commandConfig.NewSnapshot == nil {
			return apperrors.New(apperrors.InvalidInput, "NewSnapshot은 nil일 수 없습니다")
		}

		// NewSnapshot이 nil을 반환하는지 사전 검증
		// 런타임에 발생할 수 있는 잠재적 오류를 등록 시점에 차단합니다.
		if snapshot := commandConfig.NewSnapshot(); snapshot == nil {
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Command(%s)의 NewSnapshot 결과값은 nil일 수 없습니다", commandConfig.ID))
		}

		seenCommands[commandConfig.ID] = true
	}

	return nil
}

// Register 주어진 태스크 ID와 설정 정보를 Registry에 등록합니다.
func (r *Registry) Register(taskID contract.TaskID, config *Config) {
	if config == nil {
		panic("태스크 설정(config)은 nil일 수 없습니다")
	}

	// 설정 유효성 검증 (실패 시 패닉)
	if err := config.Validate(); err != nil {
		panic(err.Error())
	}

	// 외부에서 원본 config를 수정하더라도 레지스트리 내부 상태에 영향을 주지 않도록 복제합니다.
	configCopy := *config
	if config.Commands != nil {
		configCopy.Commands = make([]*CommandConfig, len(config.Commands))
		for i, commandConfig := range config.Commands {
			if commandConfig != nil {
				// CommandConfig 구조체 자체를 복사하여 포인터가 가리키는 원본이 수정되어도 영향이 없도록 합니다 (Deep Copy).
				copiedCommand := *commandConfig
				configCopy.Commands[i] = &copiedCommand
			}
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 중복 등록 방지
	if _, exists := r.configs[taskID]; exists {
		panic(fmt.Sprintf("중복된 TaskID입니다: %s", taskID))
	}

	r.configs[taskID] = &configCopy

	applog.WithComponentAndFields("task.registry", applog.Fields{
		"task_id": taskID,
	}).Info("태스크 정보가 성공적으로 등록되었습니다")
}

// RegisterForTest 유효성 검증 절차를 우회하여 Task 설정을 강제 등록합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
func (r *Registry) RegisterForTest(taskID contract.TaskID, config *Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[taskID] = config
}

// ClearForTest Registry에 등록된 모든 Task 설정을 제거합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
// 실행 중인 서버의 모든 Task 설정이 삭제되어 서비스 장애로 이어질 수 있습니다.
func (r *Registry) ClearForTest() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs = make(map[contract.TaskID]*Config)
}

// findConfig 주어진 식별자(ID)에 해당하는 Task 및 Command 설정을 검색하는 내부 메서드입니다.
// CommandID 매칭 시 와일드카드(*) 패턴을 지원하기 위해, Map 조회 후 커맨드 목록에 대한 순차 탐색(Sequential Search)을 수행합니다.
func (r *Registry) findConfig(taskID contract.TaskID, commandID contract.TaskCommandID) (*ConfigLookup, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskConfig, exists := r.configs[taskID]
	if exists {
		// 순차 탐색을 통해 명령어 ID 매칭 (와일드카드 지원 고려)
		for _, commandConfig := range taskConfig.Commands {
			if commandConfig.ID.Match(commandID) {
				return &ConfigLookup{
					Task:    taskConfig,
					Command: commandConfig,
				}, nil
			}
		}

		return nil, ErrCommandNotSupported
	}

	return nil, ErrTaskNotSupported
}

// Register 전역 Registry에 새로운 Task를 등록하는 패키지 레벨 진입점(Entry Point)입니다.
// "Fail Fast" 원칙에 따라, 유효하지 않은 설정이나 중복 ID 감지 시 즉시 패닉(Panic)을 발생시켜
// 애플리케이션 시작 단계에서 잠재적 설정 오류를 확실하게 차단합니다.
func Register(taskID contract.TaskID, config *Config) {
	defaultRegistry.Register(taskID, config)
}

// RegisterForTest 유효성 검증 절차를 우회하여 Task 설정을 강제 등록합니다.
func RegisterForTest(taskID contract.TaskID, config *Config) {
	defaultRegistry.RegisterForTest(taskID, config)
}

// ClearForTest Registry에 등록된 모든 Task 설정을 제거합니다.
//
// 경고: 이 메서드는 프로덕션 환경에서 절대 호출되어서는 안 됩니다.
// 실행 중인 서버의 모든 Task 설정이 삭제되어 서비스 장애로 이어질 수 있습니다.
func ClearForTest() {
	defaultRegistry.ClearForTest()
}

// FindConfig 전역 Registry를 통해 특정 Task 및 Command의 설정을 조회합니다.
// 주로 Task 실행 시점에 호출되며, 설정 정보가 존재하지 않을 경우 적절한 에러를 반환합니다.
func FindConfig(taskID contract.TaskID, commandID contract.TaskCommandID) (*ConfigLookup, error) {
	return defaultRegistry.findConfig(taskID, commandID)
}
