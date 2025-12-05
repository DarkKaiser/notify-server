package handler

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// validate 전역 validator 인스턴스입니다.
var validate = validator.New()

// ValidateRequest 구조체의 validation tag를 기반으로 검증을 수행합니다.
// 검증 실패 시 첫 번째 에러를 반환합니다.
func ValidateRequest(req interface{}) error {
	return validate.Struct(req)
}

// FormatValidationError validator 에러를 사용자 친화적인 한글 메시지로 변환합니다.
// 여러 검증 에러가 있을 경우 첫 번째 에러만 반환합니다.
func FormatValidationError(err error) string {
	if err == nil {
		return ""
	}

	// validator.ValidationErrors 타입으로 변환
	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		// validator 에러가 아닌 경우 원본 메시지 반환
		return err.Error()
	}

	// 첫 번째 에러만 처리
	if len(validationErrors) == 0 {
		return err.Error()
	}

	fieldErr := validationErrors[0]
	return formatFieldError(fieldErr)
}

// formatFieldError 개별 필드 에러를 한글 메시지로 변환합니다.
func formatFieldError(fieldErr validator.FieldError) string {
	fieldName := getFieldNameInKorean(fieldErr.Field())

	switch fieldErr.Tag() {
	case "required":
		return fmt.Sprintf("%s는 필수입니다", fieldName)
	case "min":
		if fieldErr.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s는 최소 %s자 이상이어야 합니다", fieldName, fieldErr.Param())
		}
		return fmt.Sprintf("%s는 최소 %s 이상이어야 합니다", fieldName, fieldErr.Param())
	case "max":
		if fieldErr.Type().Kind().String() == "string" {
			return fmt.Sprintf("%s는 최대 %s자까지 입력 가능합니다", fieldName, fieldErr.Param())
		}
		return fmt.Sprintf("%s는 최대 %s까지 입력 가능합니다", fieldName, fieldErr.Param())
	case "email":
		return fmt.Sprintf("%s는 올바른 이메일 형식이어야 합니다", fieldName)
	case "url":
		return fmt.Sprintf("%s는 올바른 URL 형식이어야 합니다", fieldName)
	default:
		return fmt.Sprintf("%s 검증 실패: %s", fieldName, fieldErr.Tag())
	}
}

// getFieldNameInKorean 필드명을 한글로 변환합니다.
func getFieldNameInKorean(field string) string {
	// 알려진 필드명 직접 매핑
	fieldNameMap := map[string]string{
		"ApplicationID": "애플리케이션 ID",
		"Message":       "메시지",
		"ErrorOccurred": "에러 발생 여부",
	}

	if koreanName, ok := fieldNameMap[field]; ok {
		return koreanName
	}

	// 매핑되지 않은 경우 원본 반환
	return field
}
