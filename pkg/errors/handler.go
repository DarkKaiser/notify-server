package errors

import (
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

// ErrorHandler 에러 처리 전략을 정의하는 인터페이스입니다.
type ErrorHandler interface {
	Handle(err error)
}

// FatalErrorHandler log.Fatal을 사용하는 기본 에러 핸들러입니다.
type FatalErrorHandler struct{}

// Handle 에러가 있을 경우 log.Fatal을 호출하여 프로세스를 종료합니다.
func (h *FatalErrorHandler) Handle(err error) {
	if err != nil {
		applog.WithComponentAndFields("errors", log.Fields{
			"error": err,
		}).Fatal("치명적인 오류 발생")
	}
}

// 전역 에러 핸들러 (프로덕션에서는 FatalErrorHandler, 테스트에서는 교체 가능)
var errorHandler ErrorHandler = &FatalErrorHandler{}

// SetErrorHandler 에러 핸들러를 설정합니다 (주로 테스트용).
func SetErrorHandler(handler ErrorHandler) {
	errorHandler = handler
}

// ResetErrorHandler 에러 핸들러를 기본값(FatalErrorHandler)으로 복원합니다.
func ResetErrorHandler() {
	errorHandler = &FatalErrorHandler{}
}

// CheckErr 에러를 확인하고 설정된 핸들러를 통해 처리합니다.
func CheckErr(err error) {
	if err != nil {
		errorHandler.Handle(err)
	}
}
