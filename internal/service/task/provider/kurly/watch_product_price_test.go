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
				WatchListFile: "products.csv",
			},
			wantErr: false,
		},
		{
			name: "성공: 대소문자 구분 없이 CSV 확장자 허용",
			settings: &watchProductPriceSettings{
				WatchListFile: "PRODUCTS.CSV",
			},
			wantErr: false,
		},
		{
			name: "실패: 파일 경로 미입력",
			settings: &watchProductPriceSettings{
				WatchListFile: "",
			},
			wantErr:   true,
			errSubstr: "watch_list_file이 설정되지 않았거나 공백입니다",
		},
		{
			name: "실패: 지원하지 않는 파일 확장자 (.txt)",
			settings: &watchProductPriceSettings{
				WatchListFile: "products.txt",
			},
			wantErr:   true,
			errSubstr: "watch_list_file은 .csv 파일 경로여야 합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.settings.Validate()
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
