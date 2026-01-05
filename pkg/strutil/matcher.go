package strutil

import (
	"strings"
)

// KeywordMatcher 키워드 매칭을 수행하는 상태 기반(Stateful) 구조체입니다.
//
// 생성 시점에 키워드 파싱과 전처리(소문자 변환)를 수행합니다.
// 따라서 동일한 키워드 셋으로 여러 문자열을 검사해야 하는 대량 처리 상황에서
// 반복적인 파싱과 메모리 할당 비용을 제거하여 높은 성능을 제공합니다.
type KeywordMatcher struct {
	// includedGroups 포함 키워드 그룹 (AND 조건)
	// 각 그룹 내부([])는 파이프(|)로 구분된 OR 조건입니다.
	// 예: ["A", "B|C"] -> A를 포함하고, (B 또는 C)를 포함해야 함
	includedGroups [][]string

	// excluded 제외 키워드 목록 (OR 조건)
	// 이 중 하나라도 포함되면 매칭 실패로 간주합니다.
	excluded []string
}

// NewKeywordMatcher 주어진 포함/제외 키워드로 새로운 KeywordMatcher를 생성합니다.
//
// 초기화 과정에서 다음 작업이 수행됩니다:
// 1. 모든 키워드를 소문자로 변환 (Case-Insensitive 매칭 준비)
// 2. 포함 키워드 내의 파이프(|) 구문을 파싱하여 그룹화
// 3. 빈 키워드 필터링
func NewKeywordMatcher(included, excluded []string) *KeywordMatcher {
	m := &KeywordMatcher{
		includedGroups: make([][]string, 0, len(included)),
		excluded:       make([]string, 0, len(excluded)),
	}

	// 제외 키워드 전처리
	for _, k := range excluded {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		m.excluded = append(m.excluded, strings.ToLower(k))
	}

	// 포함 키워드 전처리
	for _, k := range included {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}

		// 파이프(|) 처리
		if strings.Contains(k, "|") {
			orGroup := SplitClean(k, "|")
			for i, v := range orGroup {
				orGroup[i] = strings.ToLower(v)
			}
			if len(orGroup) > 0 {
				m.includedGroups = append(m.includedGroups, orGroup)
			}
		} else {
			// 단일 키워드
			m.includedGroups = append(m.includedGroups, []string{strings.ToLower(k)})
		}
	}

	return m
}

// Match 대상 문자열이 키워드 조건을 만족하는지 검사합니다.
//
// 문자열 s가 제외 키워드를 포함하지 않고, 모든 포함 키워드 그룹 조건을 만족하면 true를 반환합니다.
func (m *KeywordMatcher) Match(s string) bool {
	// 1. 제외 키워드 검사
	// 제외 조건은 하나라도 걸리면 즉시 실패하므로 먼저 검사하여 불필요한 연산을 줄입니다.
	for _, k := range m.excluded {
		if containsFold(s, k) {
			return false
		}
	}

	// 2. 포함 키워드 검사
	// 모든 그룹(AND)을 만족해야 합니다.
	for _, group := range m.includedGroups {
		// 그룹 내부는 OR 조건이므로 하나라도 매칭되면 해당 그룹은 통과(matched=true)입니다.
		matched := false
		for _, k := range group {
			if containsFold(s, k) {
				matched = true
				break
			}
		}

		// 그룹 내에서 매칭되는 키워드가 하나도 없으면 전체 조건 실패
		if !matched {
			return false
		}
	}

	return true
}

// containsFold 문자열 s가 substr을 대소문자 구분 없이 포함하는지 검사합니다.
//
// [설계 의도]
// 표준 라이브러리의 strings.ToLower(s)를 사용할 경우, 매 호출마다 전체 문자열의 복사본을 힙에 할당하게 됩니다.
// 이 함수는 메모리 할당을 0(Zero Allocation)으로 억제하기 위해, 원본 문자열을 순회하며
// 필요한 부분만 슬라이싱하여 strings.EqualFold로 비교하는 방식을 채택했습니다.
//
// [제한사항]
// 이 최적화는 대소문자 변환 시 바이트 길이가 동일하다는 가정에 의존합니다.
// 대부분의 언어(ASCII, 한글, 중국어, 일본어 등)에서는 정상 동작하지만,
// 터키어(İ/i)와 같이 대소문자 변환 시 바이트 길이가 달라지는 특수 케이스에서는
// 정확하지 않은 결과를 반환할 수 있습니다.
func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	if len(s) < len(substr) {
		return false
	}

	// [최적화 루프]
	// 문자열 s를 range로 순회하면 각 Rune의 '시작 바이트 인덱스(i)'를 얻을 수 있습니다.
	// 이를 통해 유효한 문자 경계에서만 비교를 수행하여 불필요한 연산을 줄입니다.
	//
	// 비교 방식:
	// 현재 위치(i)에서 substr 길이만큼의 부분 문자열을 슬라이싱(Zero Allocation)하여,
	// strings.EqualFold로 대소문자 구분 없이 비교합니다.
	//
	// 주의: 이 방식은 '대소문자 변환 후에도 바이트 길이가 동일하다'는 일반적인 가정(ASCII, 한글 등)에 의존합니다.
	// 특수한 유니코드 언어셋(예: 터키어)에서는 바이트 길이가 달라질 수 있으나,
	// 성능과 복잡도 균형을 위해 이 최적화 방식을 채택했습니다.
	for i := range s {
		if i+len(substr) > len(s) {
			break
		}
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}
