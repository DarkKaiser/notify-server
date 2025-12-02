package utils

import (
	"fmt"
	"regexp"
	"strings"

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
		log.WithFields(log.Fields{
			"component": "utils",
			"error":     err,
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

func ToSnakeCase(str string) string {
	matchFirstRegexp := regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllRegexp := regexp.MustCompile("([a-z0-9])([A-Z])")

	snakeCaseString := matchFirstRegexp.ReplaceAllString(str, "${1}_${2}")
	snakeCaseString = matchAllRegexp.ReplaceAllString(snakeCaseString, "${1}_${2}")

	return strings.ToLower(snakeCaseString)
}

func Contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}

func Trim(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func TrimMultiLine(s string) string {
	var ret []string
	var appendedEmptyLine bool

	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimLine := Trim(line)
		if trimLine != "" {
			appendedEmptyLine = false
			ret = append(ret, trimLine)
		} else {
			if appendedEmptyLine == false {
				appendedEmptyLine = true
				ret = append(ret, "")
			}
		}
	}

	if len(ret) >= 2 {
		if ret[0] == "" {
			ret = ret[1:]
		}
		if ret[len(ret)-1] == "" {
			ret = ret[:len(ret)-1]
		}
	}

	return strings.Join(ret, "\r\n")
}

func FormatCommas(num int) string {
	str := fmt.Sprintf("%d", num)
	re := regexp.MustCompile("(\\d+)(\\d{3})")
	for n := ""; n != str; {
		n = str
		str = re.ReplaceAllString(str, "$1,$2")
	}
	return str
}

func SplitExceptEmptyItems(s, sep string) []string {
	tokens := strings.Split(s, sep)

	var t []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			t = append(t, token)
		}
	}

	return t
}
