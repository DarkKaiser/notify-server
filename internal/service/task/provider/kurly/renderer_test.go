package kurly

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRenderProductLink HTML/Text 모드에 따른 링크 생성 및 이스케이프 동작을 검증합니다.
func TestRenderProductLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		productID    string
		productName  string
		supportsHTML bool
		want         string
	}{
		{
			name:         "Text Mode: Special Characters (Should NOT Escape)",
			productID:    "123",
			productName:  "Bread & Butter <New>",
			supportsHTML: false,
			want:         "Bread & Butter <New>(123)",
		},
		{
			name:         "HTML Mode: Special Characters (Should Escape)",
			productID:    "456",
			productName:  "Bread & Butter <New>",
			supportsHTML: true,
			want:         `<a href="https://www.kurly.com/goods/456"><b>Bread &amp; Butter &lt;New&gt;</b></a>`,
		},
		{
			name:         "Text Mode: Normal",
			productID:    "789",
			productName:  "Fresh Apple",
			supportsHTML: false,
			want:         "Fresh Apple(789)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := renderProductLink(tt.productID, tt.productName, tt.supportsHTML)
			assert.Equal(t, tt.want, got)
		})
	}
}
