package kurly

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWatchProductPriceSettings_Validate
// watchProductPriceSettings.Validate()의 유효성 검증 로직을 전방위적으로 검증합니다.
//
// 검증 대상:
//  1. 공백 정규화 (앞뒤 공백 Trim 후 재검사)
//  2. 빈 값 거부 (ErrWatchListFileEmpty)
//  3. .csv 확장자 강제 (ErrWatchListFileNotCSV)
//  4. 대소문자 무관한 확장자 검사
//  5. 정상 경로 통과
//  6. Validate() 호출 후 WatchListFile 필드 정규화 사이드이펙트 검증
func TestWatchProductPriceSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		watchListFile string
		wantErr       bool
		wantErrTarget error  // errors.Is 비교용 sentinel error
		wantNormFile  string // Validate 호출 후 WatchListFile의 기대값 (빈 문자열이면 검사 생략)
	}{
		// ── 정상 케이스 ──────────────────────────────────────────────────────
		{
			name:          "성공: 정상적인 CSV 파일 경로",
			watchListFile: "/data/watch_list.csv",
			wantErr:       false,
			wantNormFile:  "/data/watch_list.csv",
		},
		{
			name:          "성공: Windows 절대 경로 형식",
			watchListFile: `C:\Users\darkk\watch_list.csv`,
			wantErr:       false,
		},
		{
			name:          "성공: 상대 경로",
			watchListFile: "./config/products.csv",
			wantErr:       false,
		},
		{
			name:          "성공: 앞뒤 공백이 있어도 TrimSpace 후 유효한 경로 → 통과",
			watchListFile: "  /data/watch_list.csv  ",
			wantErr:       false,
			// Validate가 공백을 제거하고 필드를 정규화해야 합니다.
			wantNormFile: "/data/watch_list.csv",
		},
		{
			name:          "성공: 확장자 대문자 .CSV → 통과 (대소문자 무관)",
			watchListFile: "/data/watch_list.CSV",
			wantErr:       false,
		},
		{
			name:          "성공: 확장자 혼합 대소문자 .Csv → 통과",
			watchListFile: "/data/watch_list.Csv",
			wantErr:       false,
		},

		// ── 빈 값 에러 케이스 ─────────────────────────────────────────────────
		{
			name:          "실패: 완전히 빈 문자열 → ErrWatchListFileEmpty",
			watchListFile: "",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileEmpty,
		},
		{
			name:          "실패: 공백만 있는 문자열 → TrimSpace 후 빈 문자열 → ErrWatchListFileEmpty",
			watchListFile: "   ",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileEmpty,
		},
		{
			name:          "실패: 탭 문자만 있는 문자열 → ErrWatchListFileEmpty",
			watchListFile: "\t\t",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileEmpty,
		},

		// ── CSV 확장자 에러 케이스 ─────────────────────────────────────────────
		{
			name:          "실패: 확장자 없음 → ErrWatchListFileNotCSV",
			watchListFile: "/data/watch_list",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileNotCSV,
		},
		{
			name:          "실패: 잘못된 확장자 .txt → ErrWatchListFileNotCSV",
			watchListFile: "/data/watch_list.txt",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileNotCSV,
		},
		{
			name:          "실패: 잘못된 확장자 .json → ErrWatchListFileNotCSV",
			watchListFile: "/data/settings.json",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileNotCSV,
		},
		{
			name: "실패: .csv가 중간에 포함된 잘못된 경로 → ErrWatchListFileNotCSV",
			// 파일명은 .csv로 끝나지 않으므로 실패해야 합니다.
			watchListFile: "/data/watch.csv.bak",
			wantErr:       true,
			wantErrTarget: ErrWatchListFileNotCSV,
		},
		{
			name:          "실패: 파일명 없이 .csv만 존재하는 경우 → 통과 (HasSuffix 기준)",
			watchListFile: ".csv",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := &watchProductPriceSettings{WatchListFile: tt.watchListFile}
			err := s.Validate()

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrTarget != nil {
					assert.True(t, errors.Is(err, tt.wantErrTarget),
						"에러 타입이 일치해야 합니다. got: %v, want: %v", err, tt.wantErrTarget)
				}
			} else {
				require.NoError(t, err)
			}

			// Validate() 호출 후 WatchListFile 필드가 정규화되었는지 검증합니다.
			if tt.wantNormFile != "" {
				assert.Equal(t, tt.wantNormFile, s.WatchListFile,
					"Validate() 호출 후 WatchListFile이 TrimSpace 정규화되어야 합니다")
			}
		})
	}
}

// TestWatchProductPriceSettings_Validate_NormalizationSideEffect
// Validate()가 WatchListFile 필드를 인플레이스(In-place)로 정규화하는
// 사이드이펙트를 별도로 집중 검증합니다.
//
// 이 동작은 설계 상 의도된 것으로, 호출자가 이미 Validate()를 통과한
// 설정 객체를 그대로 사용했을 때 공백이 제거된 깨끗한 경로를 보장합니다.
func TestWatchProductPriceSettings_Validate_NormalizationSideEffect(t *testing.T) {
	t.Parallel()

	t.Run("앞뒤 공백 제거 후 필드 값이 갱신됨", func(t *testing.T) {
		t.Parallel()
		s := &watchProductPriceSettings{WatchListFile: "  /path/to/list.csv  "}
		err := s.Validate()
		require.NoError(t, err)
		assert.Equal(t, "/path/to/list.csv", s.WatchListFile)
	})

	t.Run("공백만 있는 경우 필드가 빈 문자열로 정규화되고 에러 반환", func(t *testing.T) {
		t.Parallel()
		s := &watchProductPriceSettings{WatchListFile: "   "}
		err := s.Validate()
		require.Error(t, err)
		// 에러를 반환했더라도 내부 필드는 TrimSpace가 적용된 상태여야 합니다.
		assert.Equal(t, "", s.WatchListFile,
			"Validate()가 에러를 반환할 때도 TrimSpace 정규화가 적용되어야 합니다")
	})
}
