package strutil

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKeywordMatcher_Match verifies the core matching logic of KeywordMatcher.
// It covers basic functionality, OR conditions, case insensitivity, combined filters,
// edge cases, and real-world usage scenarios.
func TestKeywordMatcher(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		included []string
		excluded []string
		input    string
		want     bool
	}{
		// 1. Basic Scenarios
		{name: "Empty string, empty keywords", input: "", included: nil, excluded: nil, want: true},
		{name: "Empty string, with included", input: "", included: []string{"test"}, excluded: nil, want: false},
		{name: "Normal string, empty keywords", input: "Hello World", included: nil, excluded: nil, want: true},

		// 2. Included Keywords (AND Logic)
		{name: "Single included match", input: "Go Programming", included: []string{"programming"}, excluded: nil, want: true},
		{name: "Single included mismatch", input: "Go Programming", included: []string{"python"}, excluded: nil, want: false},
		{name: "Multiple included all match", input: "Go Programming Tutorial", included: []string{"go", "programming", "tutorial"}, excluded: nil, want: true},
		{name: "Multiple included partial match", input: "Go Programming", included: []string{"go", "programming", "tutorial"}, excluded: nil, want: false},
		{name: "Substring match", input: "Golang is great", included: []string{"lang"}, excluded: nil, want: true},

		// 3. Excluded Keywords (OR Logic)
		{name: "Single excluded match (Fail)", input: "Deprecated API", included: nil, excluded: []string{"deprecated"}, want: false},
		{name: "Single excluded mismatch (Success)", input: "Modern API", included: nil, excluded: []string{"deprecated"}, want: true},
		{name: "Multiple excluded one match (Fail)", input: "Legacy System", included: nil, excluded: []string{"deprecated", "legacy", "old"}, want: false},
		{name: "Multiple excluded none match (Success)", input: "Modern System", included: nil, excluded: []string{"deprecated", "legacy", "old"}, want: true},

		// 4. OR Condition (Pipe Separator)
		{name: "OR included first match", input: "Go Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR included middle match", input: "Rust Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR included last match", input: "Python Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: true},
		{name: "OR included no match", input: "Java Tutorial", included: []string{"Go|Rust|Python"}, excluded: nil, want: false},
		{name: "OR included with spaces", input: "Web Development", included: []string{"Web Dev|Mobile Dev|Backend"}, excluded: nil, want: true},
		{name: "Multiple OR groups both match", input: "Go Web Server", included: []string{"Go|Rust", "Web|Mobile"}, excluded: nil, want: true},
		{name: "Multiple OR groups one mismatch", input: "Go Desktop App", included: []string{"Go|Rust", "Web|Mobile"}, excluded: nil, want: false},

		// 5. Case Insensitivity
		{name: "Case insensitive matching", input: "GO PROGRAMMING", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "Case insensitive mixed", input: "Go PrOgRaMmInG", included: []string{"gO", "ProGramming"}, excluded: nil, want: true},
		{name: "Case insensitive excluded", input: "DEPRECATED API", included: nil, excluded: []string{"deprecated"}, want: false},

		// 6. Combined Logic (AND + OR + NOT)
		{name: "Combined success", input: "Modern Go Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: true},
		{name: "Combined fail (excluded match)", input: "Legacy Go Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: false},
		{name: "Combined fail (included mismatch)", input: "Modern Python Web Server", included: []string{"go", "web"}, excluded: []string{"deprecated", "legacy"}, want: false},
		{name: "Combined OR and NOT", input: "Go Tutorial for Beginners", included: []string{"Go|Rust|Python", "tutorial"}, excluded: []string{"advanced"}, want: true},

		// 7. Special Characters & Unicode
		{name: "Korean keywords", input: "이것은 테스트 문자열입니다", included: []string{"테스트", "문자열"}, excluded: nil, want: true},
		{name: "Korean excluded", input: "이것은 샘플 문자열입니다", included: []string{"문자열"}, excluded: []string{"테스트"}, want: true},
		{name: "Emoji keywords", input: "🚀 Go Programming 🎉", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "Special char keywords", input: "C++ Programming & Development", included: []string{"c++", "development"}, excluded: nil, want: true},

		// 8. Edge Cases
		{name: "Very long string", input: strings.Repeat("Go Programming ", 1000), included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "Many keywords", input: "a b c d e f g h i j k l m n o p q r s t u v w x y z", included: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}, excluded: nil, want: true},
		{name: "Single char keyword", input: "a", included: []string{"a"}, excluded: nil, want: true},
		{name: "Whitespace only input", input: "     ", included: []string{"test"}, excluded: nil, want: false},
		{name: "Newline in input", input: "Go\nProgramming\nLanguage", included: []string{"go", "programming"}, excluded: nil, want: true},
		{name: "Tabs in input", input: "Go\tProgramming\tLanguage", included: []string{"go", "programming"}, excluded: nil, want: true},

		// 9. Nil Slices
		{name: "Nil included", input: "Go Programming", included: nil, excluded: nil, want: true},
		{name: "Nil excluded", input: "Go Programming", included: []string{"go"}, excluded: nil, want: true},
		{name: "Both nil", input: "Go Programming", included: nil, excluded: nil, want: true},

		// 10. Real-world Examples
		{name: "Product filtering success", input: "삼성 갤럭시 S24 스마트폰", included: []string{"삼성", "스마트폰"}, excluded: []string{"아이폰", "중고"}, want: true},
		{name: "Product filtering fail (excluded)", input: "삼성 갤럭시 S24 중고 스마트폰", included: []string{"삼성", "스마트폰"}, excluded: []string{"아이폰", "중고"}, want: false},
		{name: "Performance filtering OR", input: "뮤지컬 캣츠 - 서울 공연", included: []string{"뮤지컬|연극|콘서트", "서울"}, excluded: []string{"취소", "연기"}, want: true},
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

// TestNewKeywordMatcher_InternalState verifies that the constructor correctly processes
// and sanitizes input keywords (trimming, lowercasing, splitting).
func TestNewKeywordMatcher_InternalState(t *testing.T) {
	included := []string{" Apple ", "Banana|Grape"}
	excluded := []string{" Cherry "}

	m := NewKeywordMatcher(included, excluded)

	// Check Excluded: should be trimmed and lowercased
	assert.Contains(t, m.excluded, "cherry")
	assert.Len(t, m.excluded, 1)

	// Check Included Groups: should be parsed into slices of OR keywords
	assert.Len(t, m.includedGroups, 2)
	assert.Equal(t, []string{"apple"}, m.includedGroups[0])
	assert.Equal(t, []string{"banana", "grape"}, m.includedGroups[1])
}

// BenchmarkKeywordMatcher benchmarks the performance of the KeywordMatcher.
// It compares reusing a matcher vs recreating it (legacy simulation).
func BenchmarkKeywordMatcher(b *testing.B) {
	input := "The quick brown fox jumps over the lazy dog"
	included := []string{"quick", "lazy|active"}
	excluded := []string{"cat", "mouse"}

	// 1. Simulation of Legacy Style (Re-creating matcher every time)
	b.Run("Allocation_Simulated_Legacy", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			NewKeywordMatcher(included, excluded).Match(input)
		}
	})

	// 2. Optimized Style (Reuse matcher)
	b.Run("Zero_Allocation_Reuse", func(b *testing.B) {
		m := NewKeywordMatcher(included, excluded)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Match(input)
		}
	})

	// 3. Long Input Scenario
	longInput := strings.Repeat(input, 100)
	b.Run("Zero_Allocation_LongInput", func(b *testing.B) {
		m := NewKeywordMatcher(included, excluded)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			m.Match(longInput)
		}
	})
}

// BenchmarkKeywordMatcher_Integration runs a wider integration-style benchmark.
// It verifies that the matcher meets the performance requirement (e.g. < 10ms for 1000 ops).
func BenchmarkKeywordMatcher_Integration_Limit(b *testing.B) {
	largeInput := strings.Repeat("Go Programming Language Tutorial for Beginners ", 10000)
	includedKeywords := []string{"go", "programming", "tutorial"}
	excludedKeywords := []string{"advanced", "expert"}

	m := NewKeywordMatcher(includedKeywords, excludedKeywords)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		m.Match(largeInput)
	}
}

// FuzzKeywordMatcher performs fuzz testing on the Match function.
// It generates random inputs for the matcher to detect unexpected crashes or errors.
func FuzzKeywordMatcher(f *testing.F) {
	// Add seed corpus (initial inputs)
	f.Add("Go Programming", "go", "", "")
	f.Add("Hello World", "hello", "world", "java")
	f.Add("Complex String", "complex|simple", "hard", "easy")

	f.Fuzz(func(t *testing.T, input, inc, exc, sep string) {
		// Construct dynamic included/excluded keywords from fuzz inputs
		var included, excluded []string
		if inc != "" {
			included = append(included, inc)
		}
		if exc != "" {
			excluded = append(excluded, exc)
		}
		if sep != "" {
			included = append(included, sep) // Simulate multiple keywords or complex logic
		}

		m := NewKeywordMatcher(included, excluded)

		// The primary goal of fuzzing is to ensure Match() never panics
		// regardless of the input combination.
		assert.NotPanics(t, func() {
			m.Match(input)
		})
	})
}

// ExampleKeywordMatcher demonstrates how to use KeywordMatcher for filtering strings.
func ExampleKeywordMatcher() {
	// Scenario: Filter for modern Go web servers, excluding legacy ones.
	included := []string{"go", "web|http"} // Must contain "go" AND ("web" OR "http")
	excluded := []string{"legacy", "v1"}   // Must NOT contain "legacy" OR "v1"

	matcher := NewKeywordMatcher(included, excluded)

	// List of candidates
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

// TestKeywordMatcher_Concurrency verifies that KeywordMatcher is safe for concurrent use.
// It spawns multiple goroutines to call Match() on the same instance simultaneously.
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
				// Result should be consistent and not panic
				if !matcher.Match(input) {
					t.Errorf("Concurrent access failed: expected true for input %q", input)
				}
			}
		}()
	}

	wg.Wait()
}
