package config

import (
	"fmt"
	"reflect"
	"strings"

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
