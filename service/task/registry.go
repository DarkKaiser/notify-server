package task

import "github.com/darkkaiser/notify-server/config"

// NewTaskFunc 새로운 TaskHandler 인스턴스를 생성하는 팩토리 함수 타입입니다.
// 각 태스크 구현체는 이 함수를 통해 초기화됩니다.
type NewTaskFunc func(InstanceID, *RunRequest, *config.AppConfig) (TaskHandler, error)

// NewTaskResultDataFunc 태스크 실행 결과를 담을 데이터 구조체를 생성하는 함수 타입입니다.
type NewTaskResultDataFunc func() interface{}

// configs 시스템에 등록된 모든 태스크의 설정 정보를 저장하는 중앙 저장소입니다.
var configs = make(map[ID]*Config)

// Register 새로운 태스크를 시스템에 등록합니다.
// 일반적으로 각 태스크 패키지의 init() 함수에서 호출되어, 실행 가능한 작업을 등록하는 데 사용됩니다.
func Register(taskID ID, config *Config) {
	configs[taskID] = config
}

// Config 특정 태스크(Task)가 수행할 수 있는 명령어들과 실행 방법을 정의합니다.
type Config struct {
	// CommandConfigs는 이 태스크가 지원하는 하위 명령어(Command)들의 설정 목록입니다.
	CommandConfigs []*CommandConfig

	// NewTaskFn은 해당 태스크의 실제 실행 객체(TaskHandler)를 생성하는 함수입니다.
	NewTaskFn NewTaskFunc
}

// CommandConfig 태스크 내부의 특정 명령어(Command)에 대한 실행 규칙을 정의합니다.
type CommandConfig struct {
	// TaskCommandID는 명령어의 고유 식별자입니다. (예: "WatchPrice", "CheckStatus")
	TaskCommandID CommandID

	// AllowMultiple 동일한 명령어가 동시에 여러 개 실행될 수 있는지 여부를 결정합니다.
	// false일 경우, 이미 실행 중인 동일한 명령어 태스크가 있으면 새로운 요청은 무시됩니다.
	AllowMultiple bool

	// NewTaskResultDataFn은 이 명령어의 실행 결과를 저장할 데이터 객체를 생성합니다.
	NewTaskResultDataFn NewTaskResultDataFunc
}

// findConfig 주어진 ID에 해당하는 태스크 및 명령어 설정을 조회합니다.
// 등록되지 않은 태스크나 명령어인 경우 에러를 반환합니다.
func findConfig(taskID ID, taskCommandID CommandID) (*Config, *CommandConfig, error) {
	taskConfig, exists := configs[taskID]
	if exists == true {
		for _, commandConfig := range taskConfig.CommandConfigs {
			if commandConfig.TaskCommandID.Match(taskCommandID) == true {
				return taskConfig, commandConfig, nil
			}
		}

		return nil, nil, ErrCommandNotSupported
	}

	return nil, nil, ErrTaskNotSupported
}
