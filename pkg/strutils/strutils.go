package strutils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// ToSnakeCase에서 사용하는 정규식 (패키지 초기화 시 한 번만 컴파일)
	matchFirstRegexp = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllRegexp   = regexp.MustCompile("([a-z0-9])([A-Z])")

	// FormatCommas에서 사용하는 정규식
	commaRegexp = regexp.MustCompile(`(\d+)(\d{3})`)
)

// ToSnakeCase CamelCase 문자열을 snake_case로 변환합니다.
// 예: "MyVariableName" -> "my_variable_name"
func ToSnakeCase(str string) string {
	snakeCaseString := matchFirstRegexp.ReplaceAllString(str, "${1}_${2}")
	snakeCaseString = matchAllRegexp.ReplaceAllString(snakeCaseString, "${1}_${2}")
	return strings.ToLower(snakeCaseString)
}

// Trim 문자열의 앞뒤 공백을 제거하고 연속된 공백을 하나로 축약합니다.
// 예: "  hello   world  " -> "hello world"
func Trim(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// TrimMultiLine 여러 줄 문자열의 각 줄을 trim하고 연속된 빈 줄을 하나로 축약합니다.
// 앞뒤의 빈 줄도 제거됩니다.
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
			if !appendedEmptyLine {
				appendedEmptyLine = true
				ret = append(ret, "")
			}
		}
	}

	// 앞뒤의 빈 줄 제거
	if len(ret) >= 2 {
		if ret[0] == "" {
			ret = ret[1:]
		}
		if len(ret) > 0 && ret[len(ret)-1] == "" {
			ret = ret[:len(ret)-1]
		}
	}

	return strings.Join(ret, "\r\n")
}

// FormatCommas 숫자를 천 단위 구분 기호(,)가 포함된 문자열로 변환합니다.
// 예: 1234567 -> "1,234,567"
func FormatCommas(num int) string {
	str := fmt.Sprintf("%d", num)
	for {
		result := commaRegexp.ReplaceAllString(str, "$1,$2")
		if result == str {
			break
		}
		str = result
	}
	return str
}

// SplitNonEmpty 문자열을 분리하고 공백을 제거한 후 빈 항목을 제외합니다.
// 예: "a, , b,c" (구분자 ",") -> ["a", "b", "c"]
func SplitNonEmpty(s, sep string) []string {
	tokens := strings.Split(s, sep)

	var result []string
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			result = append(result, token)
		}
	}

	return result
}
