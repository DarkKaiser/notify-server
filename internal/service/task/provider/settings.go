package provider

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/maputil"
)

// Validator 설정 데이터의 유효성을 스스로 검증하는 인터페이스입니다.
type Validator interface {
	Validate() error
}

// FindTaskSettings AppConfig에서 특정 Task에 해당하는 설정을 찾아 디코딩하고 검증합니다.
// Validator 인터페이스를 구현한 경우 자동으로 유효성 검사(Validate)를 수행합니다.
func FindTaskSettings[T any](appConfig *config.AppConfig, taskID contract.TaskID) (*T, error) {
	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			settings, err := decodeAndValidate[T](t.Data)
			if err != nil {
				return nil, newErrInvalidTaskSettings(taskID, err)
			}
			return settings, nil
		}
	}

	return nil, NewErrTaskSettingsNotFound(taskID)
}

// FindCommandSettings AppConfig에서 특정 Task와 Command에 해당하는 설정을 찾아 디코딩하고 검증합니다.
// Validator 인터페이스를 구현한 경우 자동으로 유효성 검사(Validate)를 수행합니다.
func FindCommandSettings[T any](appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*T, error) {
	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					settings, err := decodeAndValidate[T](c.Data)
					if err != nil {
						return nil, newErrInvalidCommandSettings(taskID, commandID, err)
					}
					return settings, nil
				}
			}
			break
		}
	}

	return nil, NewErrCommandSettingsNotFound(taskID, commandID)
}

// decodeAndValidate 데이터를 지정된 타입 T로 디코딩하고 Validator 인터페이스 구현 여부에 따라 유효성 검사를 수행합니다.
func decodeAndValidate[T any](data map[string]any) (*T, error) {
	settings, err := maputil.Decode[T](data)
	if err != nil {
		return nil, err
	}

	// Validator 인터페이스를 구현하고 있다면 유효성 검증을 수행합니다.
	// T가 구조체인 경우 *T가 Validator를 구현할 수 있으므로 양쪽 모두 확인합니다.
	if v, ok := any(settings).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	} else if v, ok := any(&settings).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}

	return settings, nil
}
