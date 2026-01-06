package config

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/validation"
	"github.com/go-playground/validator/v10"
)

var (
	// 텔레그램 봇 토큰 검증을 위한 정규식 (예: 123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11)
	telegramBotTokenRegex = regexp.MustCompile(`^\d{3,20}:[a-zA-Z0-9_-]{30,50}$`)
)

// newValidator 새로운 Validator 인스턴스를 생성하고 커스텀 유효성 검사 함수를 등록합니다.
func newValidator() *validator.Validate {
	v := validator.New()

	// 검증 에러가 났을 때, 에러 메시지에 Go 구조체 필드명(예: CORSOrigin) 대신 JSON 이름(예: cors_origin)을 보여주도록 설정합니다.
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	// 커스텀 유효성 검사 함수 등록
	if err := v.RegisterValidation("cors_origin", validateCORSOrigin); err != nil {
		panic(fmt.Sprintf("초기화 치명적 오류: 'cors_origin' 커스텀 유효성 검사 함수 등록에 실패했습니다: %v", err))
	}
	if err := v.RegisterValidation("telegram_bot_token", validateTelegramBotToken); err != nil {
		panic(fmt.Sprintf("초기화 치명적 오류: 'telegram_bot_token' 커스텀 유효성 검사 함수 등록에 실패했습니다: %v", err))
	}

	return v
}

// validateCORSOrigin `validator` 라이브러리의 검증 인터페이스를 도메인 로직과 연결하는 어댑터(Adapter)입니다.
//
// 설정 파일에 정의된 CORS Origin 문자열을 추출한 뒤, 실제 검증은 `validation.ValidateCORSOrigin` 함수로 위임합니다.
// 이를 통해 외부 라이브러리(`validator`)와 내부 비즈니스 로직(`pkg/validation`) 간의 결합도를 낮춥니다.
func validateCORSOrigin(fl validator.FieldLevel) bool {
	return validation.ValidateCORSOrigin(fl.Field().String()) == nil
}

// validateTelegramBotToken 입력된 문자열이 유효한 텔레그램 봇 토큰 형식인지 검증합니다.
//
// 텔레그램 봇 토큰은 식별자(숫자)와 비밀키(문자열)가 콜론(:)으로 구분된 형태여야 합니다.
// 예: "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
func validateTelegramBotToken(fl validator.FieldLevel) bool {
	return telegramBotTokenRegex.MatchString(fl.Field().String())
}

// checkStruct 구조체의 유효성을 검사하고, 사용자 친화적인 에러 메시지를 반환합니다.
func checkStruct(v *validator.Validate, s interface{}, contextName string) error {
	if err := v.Struct(s); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			// 첫 번째 에러만 상세히 보고
			firstErr := validationErrors[0]

			// 커스텀 메시지가 필요한 경우 (예: unique 태그 중첩)
			if firstErr.Tag() == "unique" {
				target := "ID"
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

// checkUniqueField 슬라이스 내의 특정 필드 값이 유일한지 검사합니다.
func checkUniqueField(v *validator.Validate, data interface{}, fieldName, contextName string) error {
	if err := v.Var(data, "unique="+fieldName); err != nil {
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
