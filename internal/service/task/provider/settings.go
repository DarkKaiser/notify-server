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
	var taskSettings *T

	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			settings, err := maputil.Decode[T](t.Data)
			if err != nil {
				return nil, newErrInvalidTaskSettings(err)
			}

			// Validator 인터페이스를 구현하고 있다면 유효성 검증을 수행합니다.
			// T가 구조체인 경우 *T가 Validator를 구현할 수 있으므로 양쪽 모두 확인합니다.
			if v, ok := any(settings).(Validator); ok {
				if err := v.Validate(); err != nil {
					return nil, newErrInvalidTaskSettings(err)
				}
			} else if v, ok := any(&settings).(Validator); ok {
				if err := v.Validate(); err != nil {
					return nil, newErrInvalidTaskSettings(err)
				}
			}

			taskSettings = settings

			break
		}
	}

	if taskSettings == nil {
		return nil, ErrTaskSettingsNotFound
	}

	return taskSettings, nil
}

// FindCommandSettings AppConfig에서 특정 Task와 Command에 해당하는 설정을 찾아 디코딩하고 검증합니다.
// Validator 인터페이스를 구현한 경우 자동으로 유효성 검사(Validate)를 수행합니다.
func FindCommandSettings[T any](appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*T, error) {
	var commandSettings *T

	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					settings, err := maputil.Decode[T](c.Data)
					if err != nil {
						return nil, newErrInvalidCommandSettings(err)
					}

					// Validator 인터페이스를 구현하고 있다면 유효성 검증을 수행합니다.
					// T가 구조체인 경우 *T가 Validator를 구현할 수 있으므로 양쪽 모두 확인합니다.
					if v, ok := any(settings).(Validator); ok {
						if err := v.Validate(); err != nil {
							return nil, newErrInvalidCommandSettings(err)
						}
					} else if v, ok := any(&settings).(Validator); ok {
						if err := v.Validate(); err != nil {
							return nil, newErrInvalidCommandSettings(err)
						}
					}

					commandSettings = settings

					break
				}
			}
			break
		}
	}

	if commandSettings == nil {
		return nil, ErrCommandSettingsNotFound
	}

	return commandSettings, nil
}
