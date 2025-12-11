package task

import (
	"fmt"
	"sync"

	"github.com/darkkaiser/notify-server/config"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// NewTaskFunc Task 인스턴스를 생성하는 팩토리 함수입니다.
type NewTaskFunc func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error)

// NewTaskResultDataFunc Task 결과 데이터 구조체를 생성하는 팩토리 함수입니다.
type NewTaskResultDataFunc func() interface{}

// Config 태스크의 실행 동작과 지원하는 명령어 명세를 정의하는 불변(Immutable) 설정 구조체입니다.
// 이 구조체는 태스크 인스턴스 생성을 위한 청사진(Blueprint) 역할을 수행하며, 레지스트리에 등록된 후에는 변경되지 않습니다.
type Config struct {
	// CommandConfigs 태스크가 지원하는 명령어(Command)들의 상세 명세 목록입니다.
	// 각 항목은 명령어 식별자, 동시성 정책, 결과 데이터 스키마 등을 정의합니다.
	CommandConfigs []*CommandConfig

	NewTaskFn NewTaskFunc
}

// Validate Config의 유효성을 검사하고, 문제가 있으면 에러를 반환합니다.
func (c *Config) Validate() error {
	if c.NewTaskFn == nil {
		return apperrors.New(apperrors.ErrInvalidInput, "NewTaskFn은 nil일 수 없습니다")
	}
	if len(c.CommandConfigs) == 0 {
		return apperrors.New(apperrors.ErrInvalidInput, "CommandConfigs는 비어있을 수 없습니다")
	}

	seenCommands := make(map[CommandID]bool)
	for _, cmdConfig := range c.CommandConfigs {
		if cmdConfig.TaskCommandID == "" {
			return apperrors.New(apperrors.ErrInvalidInput, "TaskCommandID는 비어있을 수 없습니다")
		}
		// 명령어 ID 중복 검사
		if seenCommands[cmdConfig.TaskCommandID] {
			return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("중복된 TaskCommandID입니다: %s", cmdConfig.TaskCommandID))
		}
		if cmdConfig.NewTaskResultDataFn == nil {
			return apperrors.New(apperrors.ErrInvalidInput, "NewTaskResultDataFn은 nil일 수 없습니다")
		}
		seenCommands[cmdConfig.TaskCommandID] = true
	}

	return nil
}

// CommandConfig 개별 명령어의 동작 규칙을 정의합니다.
type CommandConfig struct {
	TaskCommandID CommandID

	// AllowMultiple 동일 명령어의 중복 실행(Concurrency) 허용 여부입니다.
	// - true: 여러 인스턴스가 동시에 병렬 실행될 수 있습니다.
	// - false: 이미 실행 중인 인스턴스가 있다면 새로운 요청은 무시됩니다 (Throttling/Idempotency).
	AllowMultiple bool

	NewTaskResultDataFn NewTaskResultDataFunc
}

// defaultRegistry 시스템 전역에서 공유되는 싱글톤(Singleton) 레지스트리 인스턴스입니다.
var defaultRegistry = newRegistry()

// Registry 태스크 구성요소(문서, 핸들러 등)의 설정 정보를 동시성 안전(Thread-Safe)하게 관리하는 중앙 저장소입니다.
type Registry struct {
	// configs 등록된 태스크 설정
	configs map[ID]*Config

	mu sync.RWMutex
}

// newRegistry 새로운 Registry 인스턴스를 초기화하여 반환합니다.
// 주로 단위 테스트 시 격리된 환경(Isolated Environment)을 구성하거나,
// 의존성 주입(Dependency Injection)이 필요한 경우에 사용됩니다.
func newRegistry() *Registry {
	return &Registry{
		configs: make(map[ID]*Config),
	}
}

// Register 주어진 태스크 ID와 설정 정보를 레지스트리에 등록합니다.
// 중복 등록 시 패닉이 발생하며, Thread-Safe하게 동작합니다.
func (r *Registry) Register(taskID ID, config *Config) {
	if config == nil {
		panic("태스크 설정(config)은 nil일 수 없습니다")
	}

	// 설정 유효성 검증 (실패 시 패닉)
	if err := config.Validate(); err != nil {
		panic(err.Error())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 중복 등록 방지
	if _, exists := r.configs[taskID]; exists {
		panic(fmt.Sprintf("중복된 TaskID입니다: %s", taskID))
	}

	r.configs[taskID] = config

	applog.WithComponentAndFields("task.registry", log.Fields{
		"task_id": taskID,
	}).Info("태스크 정보가 성공적으로 등록되었습니다")
}

// Register 시스템 초기화 시 태스크 정보를 기본 레지스트리에 등록하는 진입점입니다.
// "Fail Fast" 원칙을 따르며, 유효하지 않은 설정이나 중복 ID에 대해 즉시 패닉을 발생시켜 잠재적 오류를 조기에 차단합니다.
func Register(taskID ID, config *Config) {
	defaultRegistry.Register(taskID, config)
}

// findConfig 레지스트리 내부 저장소에서 태스크 및 명령어 설정을 검색하는 내부 구현 메서드입니다.
func (r *Registry) findConfig(taskID ID, taskCommandID CommandID) (*Config, *CommandConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	taskConfig, exists := r.configs[taskID]
	if exists {
		// 순차 탐색을 통해 명령어 ID 매칭 (와일드카드 지원 고려)
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.TaskCommandID.Match(taskCommandID) {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrCommandNotSupported
	}

	return nil, nil, ErrTaskNotSupported
}

// findConfig 특정 태스크 및 명령어 ID에 해당하는 설정 정보를 스레드 안전하게 조회합니다.
func findConfig(taskID ID, taskCommandID CommandID) (*Config, *CommandConfig, error) {
	return defaultRegistry.findConfig(taskID, taskCommandID)
}

// registerForTest 유효성 검증 절차를 우회하여 설정을 강제 등록하는 테스트 전용 헬퍼 메서드입니다 (프로덕션 사용 금지).
func (r *Registry) registerForTest(taskID ID, config *Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[taskID] = config
}
