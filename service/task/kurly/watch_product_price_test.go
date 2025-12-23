package kurly

import (
	"testing"

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
