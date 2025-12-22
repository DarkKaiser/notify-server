package kurly

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchProductPriceSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		settings  *watchProductPriceSettings
		wantErr   bool
		errSubstr string
	}{
		{
			name: "성공: 정상적인 CSV 파일 경로",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "products.csv",
			},
			wantErr: false,
		},
		{
			name: "성공: 대소문자 구분 없이 CSV 확장자 허용",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "PRODUCTS.CSV",
			},
			wantErr: false,
		},
		{
			name: "실패: 파일 경로 미입력",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "",
			},
			wantErr:   true,
			errSubstr: "watch_products_file이 입력되지 않았거나 공백입니다",
		},
		{
			name: "실패: 지원하지 않는 파일 확장자 (.txt)",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "products.txt",
			},
			wantErr:   true,
			errSubstr: ".csv 확장자를 가진 파일 경로만 지정할 수 있습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.settings.validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProduct_UpdateLowestPrice(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name              string
		initialProduct    *product
		wantLowestPrice   int
		wantTimeCheck     bool // 최저가 갱신 시간 업데이트 여부 확인
		timeShouldBeAfter time.Time
	}{
		{
			name: "초기 상태: 최저가가 0일 때 현재 가격으로 설정",
			initialProduct: &product{
				Price: 10000,
			},
			wantLowestPrice: 10000,
			wantTimeCheck:   true,
		},
		{
			name: "초기 상태: 최저가가 0일 때 할인 가격 우선 설정",
			initialProduct: &product{
				Price:           10000,
				DiscountedPrice: 8000,
			},
			wantLowestPrice: 8000,
			wantTimeCheck:   true,
		},
		{
			name: "갱신: 기존 최저가보다 낮은 가격 발생",
			initialProduct: &product{
				Price:           9000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   9000,
			wantTimeCheck:     true,
			timeShouldBeAfter: now,
		},
		{
			name: "유지: 기존 최저가보다 높은 가격",
			initialProduct: &product{
				Price:           12000,
				LowestPrice:     10000,
				LowestPriceTime: now,
			},
			wantLowestPrice:   10000,
			wantTimeCheck:     false, // 시간 업데이트 안 됨
			timeShouldBeAfter: now,   // 시간은 그대로 now여야 함
		},
		{
			name: "갱신: 할인 가격이 최저가보다 낮음",
			initialProduct: &product{
				Price:           12000,
				DiscountedPrice: 9000,
				LowestPrice:     10000,
			},
			wantLowestPrice: 9000,
			wantTimeCheck:   true,
		},
		{
			name: "엣지 케이스: 가격이 0원인 경우 (오류 상황)",
			initialProduct: &product{
				Price:           0,
				DiscountedPrice: 0,
				LowestPrice:     0,
			},
			wantLowestPrice: 0, // 0원은 무시 (로직상 0 < 0 은 false, 0 == 0 일때도 무시)
			wantTimeCheck:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			p := tt.initialProduct
			startTime := time.Now()

			// Execute
			p.updateLowestPrice()

			// Verify
			assert.Equal(t, tt.wantLowestPrice, p.LowestPrice)

			if tt.wantTimeCheck {
				// 시간이 갱신되었어야 함 (startTime 이후)
				assert.True(t, p.LowestPriceTime.After(startTime.Add(-time.Second)), "최저가 갱신 시간이 업데이트 되어야 합니다")
			} else if !tt.timeShouldBeAfter.IsZero() {
				// 시간이 변경되지 않았어야 함
				assert.Equal(t, tt.timeShouldBeAfter, p.LowestPriceTime, "최저가 갱신 시간이 변경되지 않아야 합니다")
			}
		})
	}
}

func TestProduct_String(t *testing.T) {
	t.Parallel()

	baseProduct := &product{
		ID:    12345,
		Name:  "맛있는 사과",
		Price: 10000,
	}
	discountProduct := &product{
		ID:              12345,
		Name:            "할인 사과",
		Price:           10000,
		DiscountedPrice: 8000,
		DiscountRate:    20,
	}

	expectedIDString := "12345" // For URL check

	tests := []struct {
		name         string
		product      *product
		supportsHTML bool
		mark         string
		prevProduct  *product
		wantContains []string
		wantNot      []string
	}{
		{
			name:         "HTML: 일반 상품",
			product:      baseProduct,
			supportsHTML: true,
			wantContains: []string{
				"맛있는 사과",
				"10,000원",
				fmt.Sprintf(productPageURLFormat, expectedIDString), // URL 포맷 사용 검증
				"<b>", "</b>", "<a href=", // HTML 태그 확인
			},
		},
		{
			name:         "Text: 일반 상품",
			product:      baseProduct,
			supportsHTML: false,
			wantContains: []string{
				"맛있는 사과",
				"10,000원",
				"☞", // Prefix 확인
			},
			wantNot: []string{"<a href=", "<b>", "</b>"},
		},
		{
			name:         "HTML: 할인 상품",
			product:      discountProduct,
			supportsHTML: true,
			wantContains: []string{
				"<s>10,000원</s>", // 취소선
				"8,000원",         // 할인가
				"(20%)",          // 할인율
			},
		},
		{
			name:         "Text: 할인 상품",
			product:      discountProduct,
			supportsHTML: false,
			wantContains: []string{
				"10,000원 ⇒ 8,000원 (20%)", // 텍스트 포맷
			},
			wantNot: []string{"<s>", "</s>"},
		},
		{
			name:         "Text: 마크(Mark) 포함",
			product:      baseProduct,
			supportsHTML: false,
			mark:         " 🆕",
			wantContains: []string{"맛있는 사과 🆕"},
		},
		{
			name:         "Text: 이전 가격 비교",
			product:      baseProduct,
			supportsHTML: false,
			prevProduct: &product{
				Price: 12000,
			},
			wantContains: []string{
				"이전 가격 : 12,000원",
			},
		},
		{
			name:         "XSS 방지: 특수문자 이스케이프 확인",
			product:      &product{ID: 1, Name: "<script>alert(1)</script>", Price: 1000},
			supportsHTML: true,
			wantContains: []string{"&lt;script&gt;alert(1)&lt;/script&gt;"},
			wantNot:      []string{"<script>"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.product.String(tt.supportsHTML, tt.mark, tt.prevProduct)

			for _, s := range tt.wantContains {
				assert.Contains(t, got, s)
			}
			for _, s := range tt.wantNot {
				assert.NotContains(t, got, s)
			}
		})
	}
}

func TestNormalizeDuplicateProducts(t *testing.T) {
	t.Parallel() // Task instance is stateless for this method

	tsk := &task{}

	tests := []struct {
		name          string
		input         [][]string
		wantDistinct  int
		wantDuplicate int
	}{
		{
			name: "중복 없음",
			input: [][]string{
				{"1001", "A", "1"},
				{"1002", "B", "1"},
			},
			wantDistinct:  2,
			wantDuplicate: 0,
		},
		{
			name: "단일 중복 발생",
			input: [][]string{
				{"1001", "A", "1"},
				{"1001", "A", "1"}, // Duplicate
			},
			wantDistinct:  1,
			wantDuplicate: 1,
		},
		{
			name: "다수 중복 발생",
			input: [][]string{
				{"1001", "A", "1"},
				{"1002", "B", "1"},
				{"1001", "A", "1"}, // Duplicate
				{"1002", "B", "1"}, // Duplicate
				{"1003", "C", "1"},
			},
			wantDistinct:  3,
			wantDuplicate: 2,
		},
		{
			name: "빈 행 무시",
			input: [][]string{
				{"1001", "A", "1"},
				{}, // Empty row
				{"1002", "B", "1"},
			},
			wantDistinct:  2,
			wantDuplicate: 0,
		},
		{
			name:          "빈 입력",
			input:         [][]string{},
			wantDistinct:  0,
			wantDuplicate: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			distinct, duplicate := tsk.normalizeDuplicateProducts(tt.input)

			assert.Equal(t, tt.wantDistinct, len(distinct), "고유 상품 개수 불일치")
			assert.Equal(t, tt.wantDuplicate, len(duplicate), "중복 상품 개수 불일치")
		})
	}
}
