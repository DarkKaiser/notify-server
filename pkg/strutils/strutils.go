package strutils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// ToSnakeCase에서 사용하는 정규식
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

// NormalizeSpaces 문자열의 앞뒤 공백을 제거하고 연속된 공백을 하나로 축약합니다.
// 예: "  hello   world  " -> "hello world"
func NormalizeSpaces(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

// NormalizeMultiLineSpaces 여러 줄 문자열의 각 줄을 정규화하고 연속된 빈 줄을 하나로 축약합니다.
// 앞뒤의 빈 줄도 제거됩니다.
func NormalizeMultiLineSpaces(s string) string {
	var ret []string
	var appendedEmptyLine bool

	lines := strings.Split(s, "\n")
	for _, line := range lines {
		trimLine := NormalizeSpaces(line)
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

// SplitAndTrim 주어진 구분자로 문자열을 분리한 후, 각 항목의 앞뒤 공백을 제거하고 빈 문자열을 제외한 슬라이스를 반환합니다.
// 결과가 없거나 입력 문자열이 비어있는 경우 nil을 반환합니다.
// 예: "a, , b,c" (구분자 ",") -> ["a", "b", "c"]
func SplitAndTrim(s, sep string) []string {
	tokens := strings.Split(s, sep)
	if len(tokens) == 0 {
		return nil
	}

	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token != "" {
			result = append(result, token)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
