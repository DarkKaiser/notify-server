package strutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKeywordMatcher_Match 매처의 핵심 매칭 로직을 검증합니다.
// 기본 기능, OR 조건, 대소문자 구분 없음, 복합 필터, 엣지 케이스 및 실제 사용 시나리오를 포괄합니다.
func TestKeywordMatcher_Match(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		included []string
		excluded []string
		input    string
		want     bool
	}{
		// 1. 기본 시나리오 (Basic Scenarios)
		{name: "빈 문자열, 키워드 없음", input: "", included: nil, excluded: nil, want: true},
		{name: "빈 문자열, 포함 키워드 있음", input: "", included: []string{"test"}, excluded: nil, want: false},
		{name: "일반 문자열, 키워드 없음", input: "Hello World", included: nil, excluded: nil, want: true},

		// 2. 포함 키워드 (AND Logic)
		{name: "단일 포함 일치", input: "Go Programming", included: []string{"programming"}, excluded: nil, want: true},
		{name: "단일 포함 불일치", input: "Go Programming", included: []string{"python"}, excluded: nil, want: false},
		{name: "다수 포함 모두 일치", input: "Go Programming Tutorial", included: []string{"go", "programming", "tutorial"}, excluded: nil, want: true},
		{name: "다수 포함 일부 불일치", input: "Go Programming", included: []string{"go", "programming", "tutorial"}, excluded: nil, want: false},
		{name: "부분 문자열 일치", input: "Golang is great", included: []string{"lang"}, excluded: nil, want: true},

		// 3. 제외 키워드 (OR Logic - 하나라도 있으면 탈락)
		{name: "단일 제외 일치 (실패)", input: "Deprecated API", included: nil, excluded: []string{"deprecated"}, want: false},
		{name: "단일 제외 불일치 (성공)", input: "Modern API", included: nil, excluded: []string{"deprecated"}, want: true},
		{name: "다수 제외 중 하나 일치 (실패)", input: "Legacy System", included: nil, excluded: []string{"deprecated", "legacy", "old"}, want: false},
		{name: "다수 제외 모두 불일치 (성공)", input: "Modern System", included: nil, excluded: []string{"deprecated", "legacy", "old"}, want: true},

		// 4. OR 조건 (파이프 Separator)
		{name: "OR 포함 첫 번째 일치", input: "Go Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR 포함 중간 일치", input: "Rust Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR 포함 마지막 일치", input: "Python Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR 포함 불일치", input: "Java Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: false},
		{name: "OR 포함 공백 처리", input: "Web Development", included: []string{"Web Dev | Mobile Dev | Backend"}, excluded: nil, want: true}, // 파이프 주변 공백 테스트
		{name: "다중 OR 그룹 모두 일치", input: "Go Web Server", included: []string{"Go|Rust", "Web|Mobile"}, excluded: nil, want: true},
		{name: "다중 OR 그룹 하나 불일치", input: "Go Desktop App", included: []string{"Go|Rust", "Web|Mobile"}, excluded: nil, want: false},

		// 5. 대소문자 구분 없음 (Case Insensitivity)
		{name: "대소문자 섞임 일치", input: "GO PROGRAMMING", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "대소문자 혼합", input: "Go PrOgRaMmInG", included: []string{"gO", "ProGramming"}, excluded: nil, want: true},
		{name: "대소문자 섞인 제외 키워드", input: "DEPRECATED API", included: nil, excluded: []string{"deprecated"}, want: false},

		// 6. 복합 로직 (AND + OR + NOT)
		{name: "복합 성공", input: "Modern Go Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: true},
		{name: "복합 실패 (제외 키워드 포함)", input: "Legacy Go Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: false},
		{name: "복합 실패 (포함 키워드 누락)", input: "Modern Python Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: false},

		// 7. 특수 문자 및 유니코드 (Korean, Emoji)
		{name: "한글 키워드", input: "이것은 테스트 문자열입니다", included: []string{"테스트", "문자열"}, excluded: nil, want: true},
		{name: "한글 제외 키워드", input: "이것은 샘플 문자열입니다", included: []string{"문자열"}, excluded: []string{"테스트"}, want: true},
		{name: "이모지 키워드", input: "🚀 Go Programming 🎉", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "특수 문자 키워드", input: "C++ Programming & Development", included: []string{"c++", "development"}, excluded: nil, want: true},

		// 8. 엣지 케이스 (Edge Cases)
		{name: "매우 긴 문자열", input: strings.Repeat("Go Programming ", 1000), included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "단일 문자 키워드", input: "a", included: []string{"a"}, excluded: nil, want: true},
		{name: "공백만 있는 입력", input: "     ", included: []string{"test"}, excluded: nil, want: false},
		{name: "개행 문자 포함", input: "Go\nProgramming\nLanguage", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "탭 문자 포함", input: "Go\tProgramming\tLanguage", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "잘못된 OR 패턴 (빈 파이프)", input: "apple", included: []string{"||apple||"}, excluded: nil, want: true}, // SplitClean 빈 항목 제거

		// 9. Nil Slices
		{name: "Nil 포함 목록", input: "Go Programming", included: nil, excluded: nil, want: true},
		{name: "Nil 제외 목록", input: "Go Programming", included: []string{"go"}, excluded: nil, want: true},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			m := NewKeywordMatcher(tt.included, tt.excluded)
			assert.Equal(t, tt.want, m.Match(tt.input))
		})
	}
}

// TestNewKeywordMatcher_InternalState 생성자가 입력 키워드를 올바르게 전처리하는지 검증합니다.
// 공백 제거, 소문자 변환, 파이프 분리 등의 로직을 확인합니다.
func TestNewKeywordMatcher_InternalState(t *testing.T) {
	// 입력: 공백이 섞인 파이프 구문과 대소문자가 섞인 키워드
	included := []string{" Apple ", "Banana | Grape | "}
	excluded := []string{" Cherry "}

	m := NewKeywordMatcher(included, excluded)

	// 제외 키워드 검증: Trim 및 소문자 변환 확인
	assert.Contains(t, m.excluded, "cherry")
	assert.Len(t, m.excluded, 1)

	// 포함 키워드 그룹 검증: OR 그룹 파싱 확인
	assert.Len(t, m.includedGroups, 2)
	assert.Equal(t, []string{"apple"}, m.includedGroups[0], "단일 키워드 처리 실패")
	assert.Equal(t, []string{"banana", "grape"}, m.includedGroups[1], "OR 그룹 파싱 및 빈 항목 제거 실패")
}

// BenchmarkKeywordMatcher KeywordMatcher의 매칭 성능을 벤치마킹합니다.
// 재사용(Reuse) 시나리오와 긴 입력값에 대한 성능을 측정합니다.
func BenchmarkKeywordMatcher(b *testing.B) {
	input := "The quick brown fox jumps over the lazy dog"
	included := []string{"quick", "lazy|active"}
	excluded := []string{"cat", "mouse"}

	// 1. 매처 재사용 (권장 패턴)
	b.Run("Zero_Allocation_Reuse", func(b *testing.B) {
		m := NewKeywordMatcher(included, excluded)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !m.Match(input) {
				b.Fatal("match failed")
			}
		}
	})

	// 2. 긴 입력값 시나리오
	longInput := strings.Repeat(input, 100)
	b.Run("Zero_Allocation_LongInput", func(b *testing.B) {
		m := NewKeywordMatcher(included, excluded)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if !m.Match(longInput) {
				b.Fatal("match failed")
			}
		}
	})

	// 3. 많은 키워드 시나리오
	manyKeywords := make([]string, 100)
	for i := 0; i < 100; i++ {
		manyKeywords[i] = fmt.Sprintf("keyword%d", i)
	}
	b.Run("Many_Keywords", func(b *testing.B) {
		m := NewKeywordMatcher(manyKeywords, nil)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Match(input) // 매칭 실패 케이스가 더 부하가 큼 (전체 순회)
		}
	})
}

// FuzzKeywordMatcher 무작위 입력을 사용해 Match 함수가 패닉을 일으키지 않는지 검증합니다.
func FuzzKeywordMatcher(f *testing.F) {
	f.Add("Go Programming", "go", "", "")
	f.Add("Hello World", "hello", "world", "java")
	f.Add("Complex String", "complex|simple", "hard", "easy")

	f.Fuzz(func(t *testing.T, input, inc, exc, sep string) {
		var included, excluded []string
		if inc != "" {
			included = append(included, inc)
		}
		if exc != "" {
			excluded = append(excluded, exc)
		}
		if sep != "" {
			// 복잡한 OR 패턴 시뮬레이션
			included = append(included, sep)
		}

		m := NewKeywordMatcher(included, excluded)

		// 패닉이 발생하지 않아야 함
		assert.NotPanics(t, func() {
			m.Match(input)
		})
	})
}

// ExampleKeywordMatcher KeywordMatcher의 사용 예시를 보여줍니다.
func ExampleKeywordMatcher() {
	// 필터 조건: "go"를 포함하고, ("web" 또는 "http")를 포함해야 하며, "legacy"나 "v1"은 제외.
	included := []string{"go", "web|http"}
	excluded := []string{"legacy", "v1"}

	matcher := NewKeywordMatcher(included, excluded)

	candidates := []string{
		"Modern Go Web Framework",
		"Legacy Go HTTP Server (v1)",
		"Python Web Server",
		"Experimental Go HTTP Library",
	}

	for _, c := range candidates {
		if matcher.Match(c) {
			fmt.Println("Matched:", c)
		}
	}

	// Output:
	// Matched: Modern Go Web Framework
	// Matched: Experimental Go HTTP Library
}

// TestKeywordMatcher_Concurrency KeywordMatcher가 고루틴 안전(Concurrency Safe)한지 검증합니다.
// Match 메서드는 읽기 전용이므로 동시 호출에 안전해야 합니다.
func TestKeywordMatcher_Concurrency(t *testing.T) {
	const (
		numGoroutines = 100
		numIterations = 1000
	)

	included := []string{"go", "concurrency"}
	excluded := []string{"race", "deadlock"}
	matcher := NewKeywordMatcher(included, excluded)
	input := "Go Concurrency is awesome and safe"

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				if !matcher.Match(input) {
					t.Errorf("Concurrent access failed: expected true for input %q", input)
				}
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// Helper Function Verification (containsFold)
// =============================================================================

// TestContainsFold 내부 헬퍼 함수 containsFold의 정확성을 검증합니다.
// ASCII, 유니코드(한글 등), 대소문자 처리 등을 확인합니다.
func TestContainsFold(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		// 1. ASCII (대소문자 무시)
		{"ASCII 정확 일치", "Hello World", "Hello", true},
		{"ASCII 대소문자 불일치 1", "Hello World", "hello", true},
		{"ASCII 대소문자 불일치 2", "Hello World", "WORLD", true},
		{"ASCII 부분 대소문자", "Hello World", "WoRLd", true},
		{"ASCII 불일치", "Hello World", "Python", false},
		{"ASCII 빈 부분문자열", "Hello World", "", true},
		{"ASCII 빈 원본", "", "Hello", false},
		{"ASCII 길이 초과", "Hi", "Hello", false},

		// 2. 유니코드 (한글)
		{"한글 정확 일치", "안녕하세요", "안녕", true},
		{"한글 중간 일치", "제 이름은 김철수입니다", "김철수", true},
		{"한글 불일치", "안녕하세요", "반갑", false},
		{"한글+영어 혼합", "Go 언어 화이팅", "go", true},

		// 3. 유니코드 케이스 폴딩 (특수 문자)
		// 그리스어 시그마: 'Σ' (U+03A3, 대문자) vs 'σ' (U+03C3, 소문자) -> EqualFold True
		{"그리스어 시그마", "Σigma", "σigma", true},

		// 4. 엣지 케이스
		{"매우 긴 패턴", "short", "longer string", false},
		{"단일 문자 소문자 매칭", "A", "a", true},
		{"단일 문자 대문자 매칭", "a", "A", true},
		{"반복 패턴 일치", "nananananana batman", "batman", true},
		{"반복 패턴 부분 일치", "nanananana", "nana", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := containsFold(tt.s, tt.substr); got != tt.want {
				t.Errorf("containsFold(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}

// BenchmarkContainsFold 표준 라이브러리 vs containsFold 성능 비교
func BenchmarkContainsFold(b *testing.B) {
	s := "The Quick Brown Fox Jumps Over The Lazy Dog"
	substr := "lazy"

	// 1. 표준 라이브러리 사용 (메모리 할당 발생)
	b.Run("StdLib_ToLower_Contains", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = strings.Contains(strings.ToLower(s), strings.ToLower(substr))
		}
	})

	// 2. 최적화된 containsFold (Zero Allocation)
	b.Run("Custom_containsFold", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !containsFold(s, substr) {
				b.Fatal("should match")
			}
		}
	})
}
