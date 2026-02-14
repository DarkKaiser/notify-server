package provider

import (
	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/maputil"
)

// Validator 설정 정보 구조체가 스스로 유효성을 검증할 수 있도록 하는 인터페이스입니다.
//
// 이 인터페이스를 구현한 설정 정보 구조체는 decodeAndValidate 함수에서 자동으로 Validate 메서드가 호출되어
// 설정값의 유효성을 검증받게 됩니다. 이를 통해 잘못된 설정 정보로 인한 런타임 오류를 사전에 방지할 수 있습니다.
type Validator interface {
	Validate() error
}

// FindTaskSettings AppConfig에서 특정 Task에 해당하는 추가 설정 정보를 찾아 타입 T로 디코딩하고 검증합니다.
//
// 이 함수는 제네릭을 사용하여 다양한 Task의 추가 설정 정보 타입을 안전하게 처리할 수 있습니다.
// 설정 정보 구조체가 Validator 인터페이스를 구현한 경우, 디코딩 후 자동으로 Validate 메서드를 호출하여
// 설정값의 유효성을 검증합니다.
//
// 파라미터:
//   - appConfig: 전체 애플리케이션 설정
//   - taskID: Task의 고유 식별자
//
// 반환값:
//   - *T: 디코딩되고 검증된 설정 정보 구조체 포인터 (성공 시)
//   - error: Task를 찾지 못하거나, 디코딩 또는 유효성 검증에 실패한 경우 에러 반환
func FindTaskSettings[T any](appConfig *config.AppConfig, taskID contract.TaskID) (*T, error) {
	// AppConfig의 모든 Task를 순회하며 일치하는 taskID를 찾습니다
	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			// 일치하는 Task를 찾으면 설정 정보를 디코딩하고 검증합니다
			settings, err := decodeAndValidate[T](t.Data)
			if err != nil {
				return nil, newErrTaskSettingsProcessingFailed(err, taskID)
			}

			return settings, nil
		}
	}

	// 일치하는 Task를 찾지 못한 경우 에러를 반환합니다
	return nil, newErrTaskNotFound(taskID)
}

// FindCommandSettings AppConfig에서 특정 Task 내의 Command에 해당하는 추가 설정 정보를 찾아 타입 T로 디코딩하고 검증합니다.
//
// 설정 정보 구조체가 Validator 인터페이스를 구현한 경우, 디코딩 후 자동으로 Validate 메서드를 호출하여
// 설정값의 유효성을 검증합니다.
//
// 파라미터:
//   - appConfig: 전체 애플리케이션 설정
//   - taskID: Task의 고유 식별자
//   - commandID: Command의 고유 식별자
//
// 반환값:
//   - *T: 디코딩되고 검증된 설정 정보 구조체 포인터 (성공 시)
//   - error: Task/Command를 찾지 못하거나, 디코딩 또는 유효성 검증에 실패한 경우 에러 반환
func FindCommandSettings[T any](appConfig *config.AppConfig, taskID contract.TaskID, commandID contract.TaskCommandID) (*T, error) {
	// AppConfig의 모든 Task를 순회하며 일치하는 taskID를 찾습니다
	for _, t := range appConfig.Tasks {
		if taskID == contract.TaskID(t.ID) {
			// 일치하는 Task를 찾았으면, 해당 Task 내의 모든 Command를 순회합니다
			for _, c := range t.Commands {
				if commandID == contract.TaskCommandID(c.ID) {
					// 일치하는 Command를 찾으면 설정 정보를 디코딩하고 검증합니다
					settings, err := decodeAndValidate[T](c.Data)
					if err != nil {
						return nil, newErrCommandSettingsProcessingFailed(err, taskID, commandID)
					}

					return settings, nil
				}
			}

			// Task는 찾았지만, 해당 Task 내에서 일치하는 Command를 찾지 못한 경우 에러를 반환합니다
			return nil, newErrCommandNotFound(taskID, commandID)
		}
	}

	// 일치하는 Task를 찾지 못한 경우 에러를 반환합니다
	return nil, newErrTaskNotFound(taskID)
}

// decodeAndValidate 비구조화된 맵 데이터를 지정된 타입 T로 디코딩하고,
// Validator 인터페이스 구현 여부에 따라 자동으로 유효성 검사를 수행하는 헬퍼 함수입니다.
//
// 파라미터:
//   - data: 디코딩할 원본 데이터
//
// 반환값:
//   - *T: 디코딩되고 검증된 설정 정보 구조체 포인터 (성공 시)
//   - error: 디코딩 실패 또는 유효성 검증 실패 시 에러 반환
func decodeAndValidate[T any](data map[string]any) (*T, error) {
	// 1단계: map 데이터를 타입 T의 구조체로 디코딩
	settings, err := maputil.Decode[T](data)
	if err != nil {
		return nil, err
	}

	// 2단계: Validator 인터페이스 구현 여부를 확인하고, 구현했다면 유효성 검증 수행
	if v, ok := any(settings).(Validator); ok {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}

	return settings, nil
}
