package config

import (
	"fmt"
	"reflect"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/validation"
	"github.com/go-playground/validator/v10"
)

var (
	// validate 설정 구조체의 유효성 검사를 담당하는 싱글톤 Validator 인스턴스입니다.
	validate = validator.New()
)

func init() {
	// 검증 에러가 났을 때, 에러 메시지에 Go 구조체 필드명(예: CORSOrigin) 대신 JSON 이름(예: cors_origin)을 보여주도록 설정합니다.
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// 커스텀 유효성 검사 함수 등록
	if err := validate.RegisterValidation("cors_origin", validateCORSOrigin); err != nil {
		panic(fmt.Sprintf("초기화 치명적 오류: 'cors_origin' 커스텀 유효성 검사 함수 등록에 실패했습니다: %v", err))
	}
}

// validateCORSOrigin 커스텀 유효성 검사 함수(Adapter)입니다.
//
// `validator` 라이브러리의 인터페이스와 `validation` 패키지의
// 순수 검증 로직(`ValidateCORSOrigin`) 사이를 연결해주는 역할을 합니다.
// 즉, 설정 파일에 있는 CORS Origin 문자열을 꺼내서 실제 검증 함수에 전달하고 그 결과를 반환합니다.
func validateCORSOrigin(fl validator.FieldLevel) bool {
	return validation.ValidateCORSOrigin(fl.Field().String()) == nil
}

// checkUniqueField 슬라이스 내의 특정 필드 값이 유일한지 검사합니다.
func checkUniqueField(data interface{}, fieldName, contextName string) error {
	if err := validate.Var(data, "unique="+fieldName); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				if fieldErr.Tag() == "unique" {
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("중복된 %s ID가 존재합니다: '%v'", contextName, fieldErr.Value()))
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s 유일성 검증에 실패했습니다", contextName))
	}
	return nil
}

// validateStruct 구조체의 유효성을 검사하고, 사용자 친화적인 에러 메시지를 반환합니다.
func validateStruct(s interface{}, contextName string) error {
	if err := validate.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			// 첫 번째 에러만 상세히 보고
			firstErr := validationErrors[0]

			// 커스텀 메시지가 필요한 경우 (예: unique 태그 중첩)
			if firstErr.Tag() == "unique" {
				target := "ID"
				// Commands 필드의 경우 "Command ID"로 명시
				if firstErr.Field() == "commands" {
					target = "Command ID"
				}
				return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s 내에 중복된 %s가 존재합니다: '%v'", contextName, target, firstErr.Value()))
			}

			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s의 설정이 올바르지 않습니다: %s (조건: %s)", contextName, firstErr.Field(), firstErr.Tag()))
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s 유효성 검증에 실패했습니다", contextName))
	}
	return nil
}
