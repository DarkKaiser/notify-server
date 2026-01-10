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

// checkStruct 구조체 인스턴스의 유효성을 태그 규칙에 따라 검증하고, 발생한 오류를 사용자 친화적인 도메인 에러로 변환합니다.
//
// 선택적 인자인 fields를 제공하면 해당 필드 범위 내에서만 부분 검증(Partial Validation)을 수행합니다.
// 이는 복합적인 중첩 구조체 검증 시, 특정 필드 집합에 대한 검증 로직을 격리하여 제어할 때 유용합니다.
func checkStruct(v *validator.Validate, s interface{}, contextName string, fields ...string) error {
	var err error
	if len(fields) > 0 {
		err = v.StructPartial(s, fields...)
	} else {
		err = v.Struct(s)
	}

	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			// 첫 번째 에러만 상세히 보고
			firstErr := validationErrors[0]

			// 필드별(Field) 커스텀 에러 처리
			switch firstErr.StructField() {
			case "MaxRetries":
				return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("HTTP 최대 재시도 횟수(max_retries)는 0 이상이어야 합니다: '%v'", firstErr.Value()))
			case "RetryDelay":
				return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("HTTP 재시도 대기 시간(retry_delay)은 0보다 커야 합니다: '%v'", firstErr.Value()))
			case "ListenPort":
				return apperrors.New(apperrors.InvalidInput, "웹 서비스 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다")
			case "TLSCertFile":
				switch firstErr.Tag() {
				case "required_if":
					return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 TLS 인증서 파일 경로(tls_cert_file)는 필수입니다")
				case "file":
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지정된 TLS 인증서 파일(tls_cert_file)을 찾을 수 없습니다: '%v'", firstErr.Value()))
				default:
					return apperrors.New(apperrors.InvalidInput, "TLS 인증서 파일 경로(tls_cert_file) 설정이 올바르지 않습니다")
				}
			case "TLSKeyFile":
				switch firstErr.Tag() {
				case "required_if":
					return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 TLS 키 파일 경로(tls_key_file)는 필수입니다")
				case "file":
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지정된 TLS 키 파일(tls_key_file)을 찾을 수 없습니다: '%v'", firstErr.Value()))
				default:
					return apperrors.New(apperrors.InvalidInput, "TLS 키 파일 경로(tls_key_file) 설정이 올바르지 않습니다")
				}
			case "Commands":
				if firstErr.Tag() == "min" {
					return apperrors.New(apperrors.InvalidInput, "작업(Task)은 최소 1개 이상의 명령(Command)를 포함해야 합니다")
				}
				// 다른 태그(예: unique)는 아래 공통 핸들러로 위임
			}

			// 태그별(Tag) 커스텀 에러 처리 (범용)
			switch firstErr.Tag() {
			case "unique":
				// 필드명이 'Tasks', 'Applications' 등 복수형일 때 단수형으로 변환하여 메시지 생성
				target := firstErr.Field()
				switch target {
				case "tasks":
					target = "작업(Task)"
				case "commands":
					target = "명령(Command)"
				case "telegrams":
					target = "알림 채널"
				case "applications":
					target = "애플리케이션(Application)"
				}

				// unique 태그 에러는 "중복된 {Target} ID가 존재합니다" 형태로 통일 (전체 슬라이스 덤프 방지)
				return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s 내에 중복된 %s ID가 존재합니다 (설정 값을 확인해주세요)", contextName, target))

			case "cors_origin":
				return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("CORS Origin 형식이 올바르지 않습니다: '%v' (형식: Scheme://Host[:Port], 예: https://example.com)", firstErr.Value()))

			case "telegram_bot_token":
				return apperrors.New(apperrors.InvalidInput, "텔레그램 BotToken 형식이 올바르지 않습니다 (올바른 형식: 123456:ABC-DEF...)")
			}

			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s의 설정이 올바르지 않습니다: %s (조건: %s)", contextName, firstErr.Field(), firstErr.Tag()))
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s 유효성 검증에 실패했습니다", contextName))
	}
	return nil
}
