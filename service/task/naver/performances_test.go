package naver

import (
	"testing"

	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNaverWatchNewPerformancesCommandConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		config        *watchNewPerformancesCommandConfig
		expectedError string
		validate      func(t *testing.T, c *watchNewPerformancesCommandConfig)
	}{
		{
			name: "성공: 정상적인 데이터 (기본값 적용 확인)",
			config: &watchNewPerformancesCommandConfig{
				Query: "뮤지컬",
			},
			validate: func(t *testing.T, c *watchNewPerformancesCommandConfig) {
				assert.Equal(t, 50, c.MaxPages, "MaxPages 기본값이 적용되어야 합니다")
				assert.Equal(t, 100, c.PageFetchDelay, "PageFetchDelay 기본값이 적용되어야 합니다")
				assert.NotNil(t, c.parsedFilters, "필터가 Eager Initialization 되어야 합니다")
			},
		},
		{
			name: "성공: 사용자 정의 설정",
			config: &watchNewPerformancesCommandConfig{
				Query:          "뮤지컬",
				MaxPages:       10,
				PageFetchDelay: 200,
			},
			validate: func(t *testing.T, c *watchNewPerformancesCommandConfig) {
				assert.Equal(t, 10, c.MaxPages)
				assert.Equal(t, 200, c.PageFetchDelay)
			},
		},
		{
			name: "실패: Query 누락",
			config: &watchNewPerformancesCommandConfig{
				Query: "",
			},
			expectedError: "query가 입력되지 않았습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, tt.config)
				}
			}
		})
	}
}

func TestNaverWatchNewPerformancesCommandConfig_FilterParsing(t *testing.T) {
	config := &watchNewPerformancesCommandConfig{
		Query: "뮤지컬",
	}
	config.Filters.Title.IncludedKeywords = "A,B"
	config.Filters.Title.ExcludedKeywords = "C"

	err := config.validate()
	require.NoError(t, err)

	assert.Equal(t, []string{"A", "B"}, config.parsedFilters.TitleIncluded)
	assert.Equal(t, []string{"C"}, config.parsedFilters.TitleExcluded)
}

func TestNaverPerformance_String(t *testing.T) {
	perf := &performance{
		Title:     "테스트 공연",
		Place:     "테스트 극장",
		Thumbnail: "<img src=\"https://example.com/thumb.jpg\">",
	}

	tests := []struct {
		name         string
		supportsHTML bool
		mark         string
		validate     func(t *testing.T, result string)
	}{
		{
			name:         "HTML 포맷 확인",
			supportsHTML: true,
			mark:         "🆕",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "<b>테스트 공연</b>")
				assert.Contains(t, result, "테스트 극장")
				assert.Contains(t, result, "🆕")
			},
		},
		{
			name:         "Text 포맷 확인",
			supportsHTML: false,
			mark:         "",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "테스트 공연")
				assert.Contains(t, result, "테스트 극장")
				assert.NotContains(t, result, "<b>")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := perf.String(tt.supportsHTML, tt.mark)
			tt.validate(t, result)
		})
	}
}

// TestNaverTask_Filtering_Behavior 은 문서화 차원에서 Naver Task의 필터링 규칙 예시를 나열합니다.
func TestNaverTask_Filtering_Behavior(t *testing.T) {
	tests := []struct {
		name     string
		item     string
		included []string
		excluded []string
		want     bool
	}{
		{"기본: 키워드 없음", "Anything", nil, nil, true},
		{"포함: 매칭", "Musical Cats", []string{"Cats"}, nil, true},
		{"포함: 미매칭", "Musical Dogs", []string{"Cats"}, nil, false},
		{"제외: 매칭", "Musical Cats", nil, []string{"Cats"}, false},
		{"제외: 미매칭", "Musical Dogs", nil, []string{"Cats"}, true},
		{"복합: 포함O 제외X", "Musical Cats", []string{"Cats"}, []string{"Dogs"}, true},
		{"복합: 포함O 제외O", "Musical Cats Dogs", []string{"Cats"}, []string{"Dogs"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tasksvc.Filter(tt.item, tt.included, tt.excluded)
			assert.Equal(t, tt.want, got)
		})
	}
}
