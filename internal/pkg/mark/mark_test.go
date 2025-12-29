package mark

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Unit Tests
// =============================================================================

// TestMarks_Format은 모든 마크 상수가 일관된 형식을 유지하는지 검증합니다.
//
// 검증 항목:
//   - 모든 마크는 " "(공백)으로 시작해야 합니다 (시각적 분리).
//   - 빈 문자열이면 안 됩니다.
func TestMarks_Format(t *testing.T) {
	t.Parallel()

	marks := map[string]string{
		"New":       New,
		"Change":    Change,
		"Disabled":  Disabled,
		"Up":        Up,
		"Down":      Down,
		"BestPrice": BestPrice,
		"Alert":     Alert,
	}

	for name, mark := range marks {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, mark, "Mark constant should not be empty")
			assert.True(t, strings.HasPrefix(mark, " "), "Mark constant should start with a space for visual padding")
		})
	}
}

// =============================================================================
// Documentation Examples
// =============================================================================

// Example은 마크 상수의 실제 출력 형태를 보여줍니다.
func Example() {
	fmt.Printf("Status:%s\n", New)
	fmt.Printf("Price:%s\n", Down)
	fmt.Printf("Stock:%s\n", Disabled)

	// Output:
	// Status: 🆕
	// Price: 🔻
	// Stock: 🚫
}
