package mark

import (
	"fmt"
	"strings"
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

	tests := []struct {
		name string
		mark Mark
	}{
		{"New", New},
		{"Modified", Modified},
		{"Unavailable", Unavailable},
		{"BestPrice", BestPrice},
		{"Alert", Alert},
	}

	for _, tt := range tests {
		tt := tt // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 1. 값 존재성
			assert.NotEmpty(t, tt.mark, "Mark constant should not be empty")

			// 2. 데이터 순수성 (Leading Space 제거 확인)
			// 설계 원칙: 마크는 순수 이모지 데이터만 보유하며, 표현(공백)은 WithSpace()로 처리한다.
			assert.False(t, strings.HasPrefix(string(tt.mark), " "),
				"Mark constant should be pure data without leading space padding")

			// 3. UTF-8 유효성
			assert.True(t, utf8.ValidString(string(tt.mark)), "Mark should be a valid UTF-8 string")
		})
	}
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
