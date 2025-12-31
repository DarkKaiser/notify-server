// Package strutil 문자열 처리를 위한 다양한 유틸리티 함수들을 제공합니다.
package strutil

import (
	"html"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NormalizeSpace 문자열의 앞뒤 공백을 제거하고, 내부의 연속된 공백을 단일 공백(' ')으로 정규화합니다.
func NormalizeSpace(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	// 버퍼에 유효한 콘텐츠(Non-Space)가 기록되기 시작했는지 여부입니다.
	// 첫 번째 유효 문자 이전의 모든 공백을 무시하는 데 사용됩니다.
	firstValWritten := false

	// 이전에 공백이 감지되었으나 아직 버퍼에 기록되지 않은 상태(Pending Space)를 나타냅니다.
	// 다음 비공백 문자가 올 때 단 한 번의 공백(' ')만 기록하여 연속된 공백을 압축합니다.
	spaceWritten := false

	for _, r := range s {
		if !unicode.IsSpace(r) {
			if firstValWritten && spaceWritten {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			firstValWritten = true
			spaceWritten = false
		} else {
			if firstValWritten {
				spaceWritten = true
			}
		}
	}

	return b.String()
}

// NormalizeMultiline 각 줄의 공백을 정규화하고, 연속된 빈 줄을 하나로 축약하여 전체 텍스트를 정리합니다.
func NormalizeMultiline(s string) string {
	if s == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(s))

	// 버퍼에 유효한 콘텐츠가 최소 한 줄 이상 기록되었는지 여부입니다.
	// 첫 번째 라인 이전에 불필요한 개행(Leading Newline)이 삽입되는 것을 방지합니다.
	var firstValWritten bool

	// 유효한 라인 이후에 빈 줄이 감지되었으나 아직 버퍼에 기록되지 않은 상태입니다.
	// 연속된 빈 줄을 하나로 압축하고, 후행 빈 줄 방지를 위해 기록 시점을 지연시킵니다.
	var pendingEmpty bool

	for line := range strings.SplitSeq(s, "\n") {
		normalizedLine := NormalizeSpace(line)

		if normalizedLine != "" {
			if firstValWritten {
				if pendingEmpty {
					b.WriteByte('\n')
					b.WriteByte('\n')
				} else {
					b.WriteByte('\n')
				}
			}
			b.WriteString(normalizedLine)
			firstValWritten = true
			pendingEmpty = false
		} else {
			if firstValWritten {
				pendingEmpty = true
			}
		}
	}

	return b.String()
}

// Integer 모든 정수 타입을 포괄하는 제네릭 인터페이스
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Comma 정수를 천 단위 구분 기호(,)가 포함된 문자열로 변환합니다. (예: 1234567 -> "1,234,567")
func Comma[T Integer](num T) string {
	// 1. 부호 있는(Signed) 정수와 부호 없는(Unsigned) 정수 변환 처리
	var val uint64
	var negative bool

	switch v := any(num).(type) {
	case int, int8, int16, int32, int64:
		// Signed Integer 처리
		var i64 int64
		switch val := v.(type) {
		case int:
			i64 = int64(val)
		case int8:
			i64 = int64(val)
		case int16:
			i64 = int64(val)
		case int32:
			i64 = int64(val)
		case int64:
			i64 = val
		}

		if i64 < 0 {
			negative = true

			// int64의 최솟값(MinInt64)은 절대값이 MaxInt64보다 1 큽니다.
			// 단순 부호 반전(-v) 시 오버플로우가 발생하므로 2의 보수(^v + 1)를 사용합니다.
			val = uint64(^i64 + 1)
		} else {
			val = uint64(i64)
		}

	case uint, uint8, uint16, uint32, uint64, uintptr:
		// Unsigned Integer 처리
		switch vTyped := v.(type) {
		case uint:
			val = uint64(vTyped)
		case uint8:
			val = uint64(vTyped)
		case uint16:
			val = uint64(vTyped)
		case uint32:
			val = uint64(vTyped)
		case uint64:
			val = vTyped
		case uintptr:
			val = uint64(vTyped)
		}
	}

	return commaUint64(val, negative)
}

// commaUint64 uint64 값을 천 단위 구분 기호(,)가 포함된 문자열로 포맷팅합니다.
// negative가 true일 경우 결과 문자열 앞에 마이너스 부호(-)를 추가합니다.
func commaUint64(n uint64, negative bool) string {
	if n == 0 {
		return "0"
	}

	// 1. 스택 버퍼에 숫자 추출 (역순 저장)
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

		// 남은 자릿수(i)가 3의 배수이고, 마지막 자리가 아닐 때(i > 0) 콤마를 추가합니다.
		if i > 0 && i%3 == 0 {
			b.WriteByte(',')
		}
	}

	return b.String()
}

// SplitClean 주어진 구분자로 문자열을 분리한 후, 각 항목의 앞뒤 공백을 제거하고 빈 문자열을 제외(Filter)한 슬라이스를 반환합니다.
// 입력 문자열이 비어있거나 유효한 항목이 없는 경우 nil을 반환합니다.
// 예: "apple, , banana, " (구분자 ",") -> ["apple", "banana"]
func SplitClean(s, sep string) []string {
	if s == "" {
		return nil
	}

	// 구분자의 개수를 미리 세어 슬라이스 용량을 예약합니다.
	// 빈 문자열이 제거되기 전이라 정확한 크기는 아니지만, 메모리 재할당 비용을 줄이는 데 효과적입니다.
	estimatedCap := strings.Count(s, sep) + 1
	result := make([]string, 0, estimatedCap)

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

// Mask API 키나 토큰 등 민감한 정보의 일부를 가려서(Masking) 안전하게 로깅할 수 있도록 합니다.
func Mask(data string) string {
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

// StripHTML 문자열에서 HTML 태그와 주석을 제거하고, HTML 엔티티를 디코딩하여 순수한 텍스트만 추출합니다.
func StripHTML(s string) string {
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

	// HTML 엔티티 디코딩
	return html.UnescapeString(builder.String())
}

// AnyContent 주어진 문자열 목록 중 하나라도 비어있지 않은(공백 제외) 값이 존재하는지 검사합니다.
func AnyContent(strs ...string) bool {
	for _, s := range strs {
		if len(strings.TrimSpace(s)) > 0 {
			return true
		}
	}
	return false
}
