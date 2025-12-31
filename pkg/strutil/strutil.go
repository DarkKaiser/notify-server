// Package strutil은 문자열 처리를 위한 다양한 유틸리티 함수들을 제공합니다.
package strutil

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeSpaces 문자열의 앞뒤 공백을 제거하고, 내부의 연속된 공백을 단일 공백(' ')으로 정규화합니다.
//
// [동작 방식]
// 1. Trim: 문자열 양 끝의 모든 유니코드 공백(Unicode Space)을 제거합니다.
// 2. Collapse: "Hello   World" -> "Hello World"와 같이 내부의 연속된 공백을 하나로 축약합니다.
//
// [성능]
// 한 번의 순회(One-Pass)로 처리를 완료하며, strings.Builder를 사용하여 메모리 재할당을 최소화합니다.
func NormalizeSpaces(s string) string {
	if s == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(s))

	spaceCount := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			spaceCount++
		} else {
			if spaceCount > 0 && builder.Len() > 0 {
				builder.WriteByte(' ')
			}
			builder.WriteRune(r)
			spaceCount = 0
		}
	}

	return builder.String()
}

// NormalizeMultiLineSpaces 여러 줄로 된 문자열을 정리(Clean-up)합니다.
//
// [동작 방식]
// 1. Line Normalization: 각 줄에 대해 NormalizeSpaces를 수행(앞뒤 공백 제거, 내부 공백 축약)합니다.
// 2. Vertical Collapse: 연속된 빈 줄을 하나의 빈 줄로 축약하여 문단 구분은 유지하되 불필요한 공백 라인을 제거합니다.
// 3. Trim: 전체 텍스트의 시작과 끝에 있는 빈 줄을 제거합니다.
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

// FormatCommas 정수(Integer)를 천 단위 구분 기호(,)가 포함된 문자열로 변환합니다.
// 예: 1234567 -> "1,234,567"
//
// [지원 타입]
// 제네릭(Integer)을 사용하여 int, int64, uint, uint64 등 모든 정수 타입을 지원합니다.
// Signed 정수의 경우 음수 부호(-)를 올바르게 처리합니다.
//
// [성능]
// 1. Stack Allocation: 숫자 변환 시 힙 대신 스택 버퍼([24]byte)를 사용하여 중간 할당을 제거했습니다.
// 2. Single Allocation: 최종 결과 문자열 생성 시에만 단 1회의 메모리 할당이 발생합니다(strings.Builder 활용).
func FormatCommas[T Integer](num T) string {
	// 1. 부호 있는(Signed) 정수와 부호 없는(Unsigned) 정수 변환 처리
	var val uint64
	var negative bool

	// 리플렉션을 사용하지 않고 Type Switch를 통해 모든 정수 타입을 효율적으로 처리
	switch v := any(num).(type) {
	case int:
		if v < 0 {
			negative, val = true, uint64(-v)
		} else {
			val = uint64(v)
		}
	case int8:
		if v < 0 {
			negative, val = true, uint64(-v)
		} else {
			val = uint64(v)
		}
	case int16:
		if v < 0 {
			negative, val = true, uint64(-v)
		} else {
			val = uint64(v)
		}
	case int32:
		if v < 0 {
			negative, val = true, uint64(-v)
		} else {
			val = uint64(v)
		}
	case int64:
		// int64의 최솟값(MinInt64)은 절대값이 MaxInt64보다 1 큽니다.
		// 따라서 단순 부호 반전(-v)을 하면 int64 범위를 초과(Overflow)하게 됩니다.
		// 이를 방지하기 위해 uint64로 캐스팅 후 비트 연산(2의 보수)을 수행합니다.
		if v < 0 {
			negative = true
			val = uint64(^v + 1) // 2의 보수(2's Complement)를 사용하여 양수로 변환
		} else {
			val = uint64(v)
		}
	case uint:
		val = uint64(v)
	case uint8:
		val = uint64(v)
	case uint16:
		val = uint64(v)
	case uint32:
		val = uint64(v)
	case uint64:
		val = v
	case uintptr:
		val = uint64(v)
	}

	return formatUint64(val, negative)
}

// formatUint64 uint64 값을 천 단위 콤마가 포함된 문자열로 포맷팅합니다.
// negative가 true일 경우 결과 문자열 앞에 마이너스 부호(-)를 추가합니다.
func formatUint64(n uint64, negative bool) string {
	if n == 0 {
		return "0"
	}

	// 1. 스택 버퍼에 숫자 추출 (역순 저장)
	// 힙 할당을 피하기 위해 고정 크기 스택 배열을 사용합니다.
	var buf [24]byte // uint64 최대값은 20자리입니다. 여유분을 포함해 24바이트를 할당합니다.
	pos := 0
	for n > 0 {
		buf[pos] = byte(n%10) + '0'
		n /= 10
		pos++
	}

	// 2. 최종 문자열 길이 계산
	// 콤마 개수 = (전체 자릿수 - 1) / 3
	commaCount := (pos - 1) / 3
	totalLen := pos + commaCount
	if negative {
		totalLen++
	}

	// 3. 문자열 조합
	var b strings.Builder
	b.Grow(totalLen) // 정확한 크기를 미리 계산하여 재할당 방지

	if negative {
		b.WriteByte('-')
	}

	// 버퍼에 역순(일의 자리 -> 높은 자리)으로 저장된 숫자를
	// 다시 역순(높은 자리 -> 일의 자리)으로 순회하며 문자열을 생성합니다.
	for i := pos - 1; i >= 0; i-- {
		b.WriteByte(buf[i])

		// 콤마 삽입 조건:
		// 남은 자릿수(i)가 3의 배수이고, 마지막 자리가 아닐 때(i > 0) 콤마를 추가합니다.
		if i > 0 && i%3 == 0 {
			b.WriteByte(',')
		}
	}

	return b.String()
}

// SplitAndTrim 주어진 구분자로 문자열을 분리한 후, 각 항목의 앞뒤 공백을 제거하고 빈 문자열을 제외한 슬라이스를 반환합니다.
// 입력 문자열이 비어있거나 유효한 항목이 없는 경우 nil을 반환합니다.
// 예: "apple, , banana, " (구분자 ",") -> ["apple", "banana"]
func SplitAndTrim(s, sep string) []string {
	if s == "" {
		return nil
	}

	// separator 개수를 미리 세어 슬라이스 용량을 예약
	// 정확한 개수는 아니지만(빈 문자열 제외 전), 재할당 횟수를 줄이는 데 효과적입니다.
	count := strings.Count(s, sep) + 1
	result := make([]string, 0, count)

	for token := range strings.SplitSeq(s, sep) {
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

// MaskSensitiveData API 키, 토큰 등 민감한 정보를 안전하게 로깅하기 위해 일부를 가립니다(Masking).
//
// [마스킹 규칙]
// 1. 3자 이하: 전체를 가립니다 ("***").
// 2. 4자: 앞 1자만 노출하고 나머지를 가립니다 ("a***").
// 3. 5자 ~ 12자: 앞 4자만 노출하고 나머지를 가립니다 ("abcd***").
// 4. 12자 초과: 앞 4자와 뒤 4자를 노출하고 중간을 가립니다 ("abcd***wxyz").
func MaskSensitiveData(data string) string {
	if data == "" {
		return ""
	}

	// 문자열 길이(룬 글자 수) 계산
	length := utf8.RuneCountInString(data)

	// 1. 3자 이하: 전체 마스킹
	if length <= 3 {
		return "***"
	}

	// 바이트 인덱스 찾기 함수 (Closure로 캡쳐하여 재사용)
	getByteIndex := func(n int) int {
		idx := 0
		for i := 0; i < n; i++ {
			_, size := utf8.DecodeRuneInString(data[idx:])
			idx += size
		}
		return idx
	}

	// 2. 4자: 앞 1자 + *** (비밀번호 등 짧은 중요 데이터 보호)
	if length == 4 {
		end := getByteIndex(1)
		return data[:end] + "***"
	}

	// 3. 12자 이하: 앞 4자 + ***
	if length <= 12 {
		end := getByteIndex(4)
		return data[:end] + "***"
	}

	// 4. 12자 초과: 앞 4자 + *** + 뒤 4자
	// 앞 4자 인덱스
	prefixEnd := getByteIndex(4)

	// 뒤 4자 인덱스 (뒤에서부터 찾는 것이 효율적일 수 있으나 UTF-8은 앞에서부터가 안전/간단)
	suffixStart := getByteIndex(length - 4)

	return data[:prefixEnd] + "***" + data[suffixStart:]
}

// StripHTMLTags 입력된 문자열에서 HTML 태그(<...>)를 모두 제거하고, HTML 엔티티(예: &amp;)를 디코딩하여 순수한 텍스트만 반환합니다.
//
// [동작 방식]
// 1. 빠른 검사: '<' 문자가 발견되면 즉시 다음 문자를 확인하여 태그 가능성을 검사합니다.
//   - 태그가 아닌 패턴(예: "3 < 5", "<123>")은 스캔을 건너뛰어 성능을 보존합니다.
//
// 2. 태그 제거: '<'로 시작해 '>'로 끝나는 블록을 제거합니다.
//   - 속성 값 내의 '>' 문자(예: <a title=">">)를 오인하지 않도록 따옴표(', ") 상태를 추적합니다(State Machine).
//
// 3. 주석 제거: '<!--' ... '-->' 형태의 HTML 주석도 함께 제거됩니다.
//
// 4. 엔티티 디코딩: 남은 텍스트에 대해 html.UnescapeString을 수행합니다.
//
// [성능 및 주의사항]
// - Zero Allocation: 정규식 대신 바이트 단위 순회(Linear Scan)를 사용하여 메모리 할당을 최소화했습니다.
// - 안전성: XSS 방지용이 아니며, 잘못된 형식의 HTML(깨진 태그 등)에 대해서는 최선의 노력으로 처리합니다.
func StripHTMLTags(s string) string {
	if !strings.ContainsAny(s, "<&") {
		return s
	}

	var builder strings.Builder
	builder.Grow(len(s))

	n := len(s)
	for i := 0; i < n; i++ {
		b := s[i]

		// Tag 시작 ('<')
		if b == '<' {
			// [Fail-Fast] 태그 이름 유효성 검사 (즉시 수행)
			// < 뒤에 유효한 태그 시작 문자(알파벳, /, !, ?)가 없으면 태그가 아니므로 스킵
			if i+1 < n {
				next := s[i+1]
				isValidTagStart := (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') || next == '/' || next == '!' || next == '?'
				if !isValidTagStart {
					builder.WriteByte(b)
					continue
				}
			} else {
				// < 로 끝나는 경우
				builder.WriteByte(b)
				continue
			}

			// 주석 처리: <!-- ... -->
			if i+3 < n && s[i+1] == '!' && s[i+2] == '-' && s[i+3] == '-' {
				closeIndex := strings.Index(s[i+4:], "-->")
				if closeIndex != -1 {
					i += 4 + closeIndex + 2 // i를 --> 끝으로 이동
					continue
				}
			}

			// 태그 닫기 ('>') 찾기 - 따옴표 컨텍스트 고려
			inQuote := false
			var quoteChar byte
			closed := false

			// 현재 위치 이후를 순회
			j := i + 1
			for ; j < n; j++ {
				curr := s[j]

				if inQuote {
					if curr == quoteChar {
						inQuote = false // 따옴표 종료
					}
					// 따옴표 내부이므로 '>'가 나와도 무시하고 계속 진행
				} else {
					if curr == '"' || curr == '\'' {
						inQuote = true
						quoteChar = curr
					} else if curr == '>' {
						// 따옴표 밖에서 '>'를 만났으므로 태그 종료 -> 루프 탈출
						closed = true
						break
					}
				}
			}

			// 태그가 정상적으로 닫혔다면, i를 j로 이동시켜 태그 내용 스킵
			if closed {
				i = j
				continue
			}

			// 닫히지 않은 '<'는 일반 텍스트로 취급하여 출력 (루프 계속)
		}

		builder.WriteByte(b)
	}

	// HTML Entity 디코딩
	return html.UnescapeString(builder.String())
}

// HasAnyContent 전달된 문자열 중 하나라도 비어있지 않은(non-empty) 값이 존재하는지 확인합니다.
//
// [동작 방식]
// 인자를 순차적으로 순회하며 길이가 1 이상인 문자열을 발견하면 즉시 true를 반환합니다(Short-circuit).
// 인자가 없거나 모든 문자열이 비어있는 경우 false를 반환합니다.
//
// [주의사항]
// 공백 문자(" ")나 제어 문자도 내용이 있는 것으로 간주합니다.
// 의미 있는 텍스트 존재 여부를 확인하려면 먼저 strings.TrimSpace 등을 적용해야 합니다.
func HasAnyContent(strs ...string) bool {
	for _, s := range strs {
		if len(s) > 0 {
			return true
		}
	}
	return false
}
