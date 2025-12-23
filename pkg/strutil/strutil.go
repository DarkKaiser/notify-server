// Package strutil은 문자열 처리를 위한 다양한 유틸리티 함수들을 제공합니다.
package strutil

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

var (
	// ToSnakeCase에서 사용하는 정규식
	matchFirstRegexp = regexp.MustCompile("(.)([A-Z][a-z]+)")
	matchAllRegexp   = regexp.MustCompile("([a-z0-9])([A-Z])")

	// HTML 태그 제거에 사용하는 정규식
	// < 다음에 영문자가 오는 경우만 태그로 인식하여 수학 기호(<) 오탐지를 방지합니다.
	// 예: "3 < 5"는 유지되고, "<br>"이나 "<b>"는 제거됩니다.
	htmlTagRegexp = regexp.MustCompile(`</?([a-zA-Z]+)[^>]*>`)
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
	var result []string
	var appendedEmptyLine bool

	lineIter := strings.SplitSeq(s, "\n")
	for line := range lineIter {
		normalizedLine := NormalizeSpaces(line)
		if normalizedLine != "" {
			appendedEmptyLine = false
			result = append(result, normalizedLine)
		} else {
			if !appendedEmptyLine {
				appendedEmptyLine = true
				result = append(result, "")
			}
		}
	}

	// 앞뒤의 빈 줄 제거
	if len(result) >= 2 {
		if result[0] == "" {
			result = result[1:]
		}
		if len(result) > 0 && result[len(result)-1] == "" {
			result = result[:len(result)-1]
		}
	}

	return strings.Join(result, "\r\n")
}

// Integer 모든 정수 타입을 포괄하는 제네릭 인터페이스
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// FormatCommas 숫자를 천 단위 구분 기호(,)가 포함된 문자열로 변환합니다.
// 예: 1234567 -> "1,234,567"
func FormatCommas[T Integer](num T) string {
	str := fmt.Sprintf("%d", num)

	// 음수 처리 (문자열 기반으로 판단)
	startOffset := 0
	if strings.HasPrefix(str, "-") {
		startOffset = 1
	}

	// 콤마가 필요 없는 경우 (3자리 이하)
	if len(str)-startOffset <= 3 {
		return str
	}

	var builder strings.Builder

	// 예상 크기 미리 할당: 원래 길이 + 콤마 개수
	commaCount := (len(str) - startOffset - 1) / 3
	builder.Grow(len(str) + commaCount)

	if startOffset == 1 {
		builder.WriteByte('-')
		str = str[1:]
	}

	// 첫 번째 그룹 (1~3자리)
	firstGroupLen := len(str) % 3
	if firstGroupLen == 0 {
		firstGroupLen = 3
	}

	builder.WriteString(str[:firstGroupLen])

	// 나머지 그룹들 (3자리씩)
	for i := firstGroupLen; i < len(str); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(str[i : i+3])
	}

	return builder.String()
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

// MaskSensitiveData 민감한 정보를 마스킹합니다.
// 토큰, 키 등의 민감 정보를 안전하게 로깅하기 위해 사용합니다.
func MaskSensitiveData(data string) string {
	if data == "" {
		return ""
	}

	// 3자 이하는 전체 마스킹
	if len(data) <= 3 {
		return "***"
	}

	// 앞 4자만 표시하고 나머지는 마스킹
	if len(data) <= 12 {
		return data[:4] + "***"
	}

	// 긴 토큰은 앞 4자 + 마스킹 + 뒤 4자
	return data[:4] + "***" + data[len(data)-4:]
}

// StripHTMLTags 문자열에서 HTML 태그를 제거하고, HTML 엔티티를 디코딩하여 순수한 텍스트를 반환합니다.
// 예: "<b>Hello</b> &amp; World" -> "Hello & World"
func StripHTMLTags(s string) string {
	stripped := htmlTagRegexp.ReplaceAllString(s, "")
	return html.UnescapeString(stripped)
}
