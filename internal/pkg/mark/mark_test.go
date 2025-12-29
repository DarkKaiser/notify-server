package mark

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// -----------------------------------------------------------------------------
// Unit Tests: Constants Integrity
// -----------------------------------------------------------------------------

// TestMarks_Integrity는 패키지 내 정의된 마크 상수들의 무결성을 검증합니다.
//
// [검증 항목]
// 1. 값의 존재성: 빈 문자열이 아니어야 함.
// 2. 포맷 규칙: 선행 공백(padding)을 포함하지 않아야 함 (데이터 순수성 유지).
// 3. UTF-8 유효성: 올바른 UTF-8 인코딩이어야 함.
func TestMarks_Integrity(t *testing.T) {
	t.Parallel()

	// mark.Values()를 통해 모든 마크를 자동으로 검증합니다.
	// 개발자가 새로운 마크를 추가하고 mark.Values()에 등록만 하면, 이 테스트는 자동으로 커버합니다.
	allMarks := Values()
	for _, mark := range allMarks {
		mark := mark // capture range variable
		t.Run(string(mark), func(t *testing.T) {
			t.Parallel()

			// 1. 값 존재성
			assert.NotEmpty(t, mark, "Mark constant should not be empty")

			// 2. 데이터 순수성 (Leading Space 제거 확인)
			// 설계 원칙: 마크는 순수 이모지 데이터만 보유하며, 표현(공백)은 WithSpace()로 처리한다.
			assert.False(t, strings.HasPrefix(string(mark), " "),
				"Mark constant should be pure data without leading space padding")

			// 3. UTF-8 유효성
			assert.True(t, utf8.ValidString(string(mark)), "Mark should be a valid UTF-8 string")
		})
	}

	// [추가 검증] 알려진 모든 상수가 Values()에 포함되어 있는지 확인
	// 누락 방지를 위한 안전망
	expectedMarks := []Mark{New, Modified, Unavailable, BestPrice, Alert}
	assert.ElementsMatch(t, expectedMarks, Values(), "Values() slice must contain all defined constants")
}

// TestMark_Values_Immutability는 Values()가 반환한 슬라이스가 외부 변경으로부터 안전한지 검증합니다.
func TestMark_Values_Immutability(t *testing.T) {
	t.Parallel()

	original := Values()
	modified := Values()

	// 외부에서 슬라이스 내용 변경 시도
	modified[0] = "MUTATED"

	// 원본에 영향이 없어야 함
	assert.NotEqual(t, original[0], modified[0], "Modification of returned slice must not affect other calls")
	assert.Equal(t, New, original[0], "Original values must remain unchanged")
}

// TestValues_Concurrency는 멀티 고루틴 환경에서 Values() 호출의 안전성을 검증합니다.
// 전역 변수 `all`에 대한 읽기 작업이 Race Condition 없이 수행되는지 확인합니다.
func TestValues_Concurrency(t *testing.T) {
	t.Parallel()

	const (
		goroutines = 100
		iterations = 1000
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// 동시 다발적으로 Values() 호출
				vals := Values()
				// 반환된 값의 기본 무결성 체크 (Panic 유발 가능성 등 확인)
				if len(vals) == 0 {
					t.Error("Values() returned empty slice unexpectedly")
				}
			}
		}()
	}

	wg.Wait()
}

// TestMark_Parse는 문자열을 Mark로 파싱하는 기능을 검증합니다.
func TestMark_Parse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		wantMark Mark
		wantErr  bool
	}{
		{"🆕", New, false},
		{"🔥", BestPrice, false},
		{"Invalid", "", true},
		{"", "", true},
		{" 🆕", "", true}, // 공백 포함된 것은 순수 마크가 아님
	}

	for _, tt := range tests {
		tt := tt
		t.Run(fmt.Sprintf("Input_%q", tt.input), func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantMark, got)
			}
		})
	}
}

// FuzzParse는 다양한 임의의 입력값에 대해 Parse 함수가 견고하게 동작하는지 검증합니다.
// Crash나 Panic이 발생하지 않고, 적절히 에러를 반환하거나 성공해야 합니다.
func FuzzParse(f *testing.F) {
	// Seed corpus 추가 (유효한 값들)
	f.Add("🆕")
	f.Add("🔥")
	f.Add("InvalidString")
	f.Add("")

	f.Fuzz(func(t *testing.T, orig string) {
		mark, err := Parse(orig)

		if err == nil {
			// 파싱 성공 시:
			// 1. 반환된 마크는 유효해야 함
			assert.True(t, mark.IsValid(), "Parsed mark must be valid if no error returned")
			// 2. 원본 문자열과 같아야 함 (Mark는 string alias이므로)
			assert.Equal(t, Mark(orig), mark, "Parsed mark should match original string")
		} else {
			// 에러 발생 시:
			// 1. 마크는 빈 문자열이어야 함 (Zero Value)
			assert.Empty(t, mark, "Mark should be empty on error")
		}
	})
}

// -----------------------------------------------------------------------------
// Unit Tests: Methods
// -----------------------------------------------------------------------------

// TestMark_WithSpace_TableDriven은 WithSpace 메서드의 동작을 다양한 입력값에 대해 검증합니다.
//
// [규칙]
// - Empty Mark -> Empty String (No padding)
// - Valid Mark -> Space + Mark
func TestMark_WithSpace_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mark Mark
		want string
	}{
		{
			name: "Standard Mark (New)",
			mark: New,
			want: " 🆕",
		},
		{
			name: "Standard Mark (BestPrice)",
			mark: BestPrice,
			want: " 🔥",
		},
		{
			name: "Empty Mark (Edge Case)",
			mark: Mark(""),
			want: "", // 빈 마크는 공백도 없어야 함
		},
		{
			name: "Custom Text Mark",
			mark: Mark("TEST"),
			want: " TEST",
		},
		{
			name: "Already Spaced Mark (Edge Case)",
			mark: Mark(" A"), // 이미 공백이 있는 데이터라도 동작의 일관성을 위해 공백 추가
			want: "  A",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.mark.WithSpace())
		})
	}
}

// TestMark_String_Interface는 fmt.Stringer 인터페이스 구현을 검증합니다.
func TestMark_String_Interface(t *testing.T) {
	t.Parallel()

	// Type Assertion to verify interface compliance
	var _ fmt.Stringer = New

	tests := []struct {
		name string
		mark Mark
		want string
	}{
		{"New", New, "🆕"},
		{"Modified", Modified, "🔁"},
		{"Empty", Mark(""), ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.mark.String())
			// fmt 패키지와의 통합 동작 확인
			assert.Equal(t, tt.want, fmt.Sprintf("%s", tt.mark))
		})
	}
}

// TestMark_IsValid는 IsValid 메서드의 동작을 검증합니다.
func TestMark_IsValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mark Mark
		want bool
	}{
		{"Valid Mark (New)", New, true},
		{"Valid Mark (Alert)", Alert, true},
		{"Invalid Mark (Random String)", Mark("Invalid"), false},
		{"Invalid Mark (Empty)", Mark(""), false},
		{"Invalid Mark (Space + New)", Mark(" 🆕"), false}, // 순수 데이터가 아니므로 유효하지 않음
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.mark.IsValid(), "IsValid() check failed for %v", tt.mark)
		})
	}
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

// BenchmarkMark_WithSpace WithSpace 메서드의 성능을 측정합니다.
// 빈번하게 호출되는 메서드이므로 제로 할당 또는 최소 할당을 확인합니다.
func BenchmarkMark_WithSpace(b *testing.B) {
	m := New
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.WithSpace()
	}
}

func BenchmarkMark_String(b *testing.B) {
	m := New
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.String()
	}
}

// -----------------------------------------------------------------------------
// Documentation Examples
// -----------------------------------------------------------------------------

func ExampleMark_WithSpace() {
	// 1. 표준 마크 사용 (자동 패딩)
	fmt.Printf("Title%s\n", New.WithSpace())
	fmt.Printf("Price%s\n", BestPrice.WithSpace())

	// 2. 빈 마크 사용 (패딩 없음)
	empty := Mark("")
	fmt.Printf("Empty%s\n", empty.WithSpace())

	// Output:
	// Title 🆕
	// Price 🔥
	// Empty
}

func ExampleMark_String() {
	// String() 메서드나 %s 포맷팅은 순수 값을 반환합니다.
	fmt.Println(New)
	fmt.Println(Modified.String())

	// Output:
	// 🆕
	// 🔁
}
