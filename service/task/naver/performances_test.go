package naver

import (
	"fmt"
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

// TestParsePerformancesFromHTML 파싱 로직을 HTML 입력값 기반으로 직접 테스트합니다. (Unit Test)
func TestParsePerformancesFromHTML(t *testing.T) {
	// Helper to make full list item HTML
	makeItem := func(title, place, thumbSrc string) string {
		return fmt.Sprintf(`
			<li>
				<div class="item">
					<div class="title_box">
						<strong class="name">%s</strong>
						<span class="sub_text">%s</span>
					</div>
					<div class="thumb">
						<img src="%s">
					</div>
				</div>
			</li>`, title, place, thumbSrc)
	}

	tests := []struct {
		name          string
		html          string
		filters       *parsedFilters
		expectedCount int                                             // 필터링 후 예상 개수
		expectedRaw   int                                             // 필터링 전 raw 개수
		expectError   bool                                            // 에러 발생 여부
		validateItems func(t *testing.T, performances []*performance) // 세부 항목 검증
	}{
		{
			name:          "성공: 단일 항목 파싱",
			html:          fmt.Sprintf("<ul>%s</ul>", makeItem("Cats", "Broadway", "cats.jpg")),
			filters:       &parsedFilters{}, // 필터 없음
			expectedCount: 1,
			expectedRaw:   1,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Cats", performances[0].Title)
				assert.Equal(t, "Broadway", performances[0].Place)
				assert.Contains(t, performances[0].Thumbnail, "cats.jpg")
			},
		},
		{
			name: "성공: 필터링 (Include)",
			html: fmt.Sprintf("<ul>%s%s</ul>",
				makeItem("Cats Musical", "Seoul", "1.jpg"),
				makeItem("Dog Show", "Seoul", "2.jpg")),
			filters: &parsedFilters{
				TitleIncluded: []string{"Musical"},
			},
			expectedCount: 1, // Cats only
			expectedRaw:   2,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Cats Musical", performances[0].Title)
			},
		},
		{
			name: "성공: 필터링 (Exclude)",
			html: fmt.Sprintf("<ul>%s%s</ul>",
				makeItem("Happy Musical", "Seoul", "1.jpg"),
				makeItem("Sad Drama", "Seoul", "2.jpg")),
			filters: &parsedFilters{
				TitleExcluded: []string{"Drama"},
			},
			expectedCount: 1, // Happy only
			expectedRaw:   2,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Happy Musical", performances[0].Title)
			},
		},
		{
			name:        "실패: HTML 파싱 에러 (필수 요소 누락 - 제목)",
			html:        `<ul><li><div class="item"><div class="title_box"></div></div></li></ul>`, // strong.name 없음
			filters:     &parsedFilters{},
			expectError: true,
		},
		{
			name:        "실패: HTML 파싱 에러 (필수 요소 누락 - 썸네일)",
			html:        `<ul><li><div class="item"><div class="title_box"><strong class="name">T</strong><span class="sub_text">P</span></div></div></li></ul>`, // thumb 없음
			filters:     &parsedFilters{},
			expectError: true,
		},
		{
			name:          "성공: 빈 결과",
			html:          `<ul></ul>`,
			filters:       &parsedFilters{},
			expectedCount: 0,
			expectedRaw:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perfs, rawCount, err := parsePerformancesFromHTML(tt.html, tt.filters)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(perfs), "필터링 후 개수가 일치해야 합니다")
				assert.Equal(t, tt.expectedRaw, rawCount, "Raw 개수가 일치해야 합니다")
				if tt.validateItems != nil {
					tt.validateItems(t, perfs)
				}
			}
		})
	}
}
