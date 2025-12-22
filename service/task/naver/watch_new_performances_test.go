package naver

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/pkg/strutil"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNaverWatchNewPerformancesSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		config        *watchNewPerformancesSettings
		expectedError string
		validate      func(t *testing.T, c *watchNewPerformancesSettings)
	}{
		{
			name: "성공: 정상적인 데이터 (기본값 적용 확인)",
			config: &watchNewPerformancesSettings{
				Query: "뮤지컬",
			},
			validate: func(t *testing.T, c *watchNewPerformancesSettings) {
				assert.Equal(t, 50, c.MaxPages, "MaxPages 기본값이 적용되어야 합니다")
				assert.Equal(t, 100, c.PageFetchDelay, "PageFetchDelay 기본값이 적용되어야 합니다")
			},
		},
		{
			name: "성공: 사용자 정의 설정",
			config: &watchNewPerformancesSettings{
				Query:          "뮤지컬",
				MaxPages:       10,
				PageFetchDelay: 200,
			},
			validate: func(t *testing.T, c *watchNewPerformancesSettings) {
				assert.Equal(t, 10, c.MaxPages)
				assert.Equal(t, 200, c.PageFetchDelay)
			},
		},
		{
			name: "실패: Query 누락",
			config: &watchNewPerformancesSettings{
				Query: "",
			},
			expectedError: "query가 입력되지 않았거나 공백입니다",
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable for parallel execution
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestNaverPerformance_String(t *testing.T) {
	t.Parallel()

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
			mark:         " 🆕",
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "<b>테스트 공연</b>")
				assert.Contains(t, result, "테스트 극장")
				assert.Contains(t, result, " 🆕")
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
		{
			name:         "Text 포맷 확인 (특수문자 비노출)",
			supportsHTML: false,
			mark:         "",
			validate: func(t *testing.T, result string) {
				p := &performance{Title: "Tom & Jerry", Place: "Cinema", Thumbnail: "img"}
				res := p.String(false, "")
				assert.Contains(t, res, "Tom & Jerry")
				assert.NotContains(t, res, "Tom &amp; Jerry")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := perf.String(tt.supportsHTML, tt.mark)
			tt.validate(t, result)
		})
	}
}

// TestNaverTask_Filtering_Behavior 은 문서화 차원에서 Naver Task의 필터링 규칙 예시를 나열합니다.
func TestNaverTask_Filtering_Behavior(t *testing.T) {
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := strutil.Filter(tt.item, tt.included, tt.excluded)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParsePerformancesFromHTML 파싱 로직을 HTML 입력값 기반으로 직접 테스트합니다. (Unit Test)
func TestParsePerformancesFromHTML(t *testing.T) {
	t.Parallel()

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
			name:          "성공: 썸네일 누락 (Soft Fail)",
			html:          `<ul><li><div class="item"><div class="title_box"><strong class="name">T</strong><span class="sub_text">P</span></div></div></li></ul>`, // thumb 없음
			filters:       &parsedFilters{},
			expectedCount: 1,
			expectedRaw:   1,
			expectError:   false,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "T", performances[0].Title)
				assert.Equal(t, "P", performances[0].Place)
				assert.Equal(t, "", performances[0].Thumbnail, "썸네일이 없으면 빈 문자열이어야 합니다")
			},
		},
		{
			name:          "성공: 빈 결과",
			html:          `<ul></ul>`,
			filters:       &parsedFilters{},
			expectedCount: 0,
			expectedRaw:   0,
		},
		{
			name: "성공: 실제 네이버 검색 결과 샘플 (Robust Selector Test)",
			html: `
			<ul>
				<li>
					<a href="#" class="inner">
						<div class="item">
							<div class="thumb">
								<img src="https://search.pstatic.net/common?type=f&size=224x338" alt="레미제라블 - 부산" onerror="this.src='no_img.png'">
							</div>
							<div class="title_box">
								<strong class="name line_3">레미제라블 - 부산</strong>
								<span class="sub_text line_1">드림씨어터</span>
							</div>
						</div>
					</a>
				</li>
			</ul>`,
			filters:       &parsedFilters{},
			expectedCount: 1,
			expectedRaw:   1,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "레미제라블 - 부산", performances[0].Title)
				assert.Equal(t, "드림씨어터", performances[0].Place)
				assert.Contains(t, performances[0].Thumbnail, "https://search.pstatic.net/common?type=f&size=224x338")
			},
		},
		{
			name:        "실패: HTML 파싱 에러 (내용 비어있음 - 제목)",
			html:        `<ul><li><div class="item"><div class="title_box"><strong class="name">   </strong><span class="sub_text">Place</span></div><div class="thumb"><img src="t.jpg"></div></div></li></ul>`,
			filters:     &parsedFilters{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

// TestPerformance_Key Key() 메서드의 동작을 검증합니다.
func TestPerformance_Key(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf     *performance
		expected string
	}{
		{
			name: "정상적인 키 생성",
			perf: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			expected: "뮤지컬 캣츠|브로드웨이극장",
		},
		{
			name: "특수문자 포함",
			perf: &performance{
				Title: "공연|제목",
				Place: "장소|이름",
			},
			expected: "공연|제목|장소|이름",
		},
		{
			name: "빈 문자열",
			perf: &performance{
				Title: "",
				Place: "",
			},
			expected: "|",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf.Key()
			assert.Equal(t, tt.expected, result, "Key() 결과가 예상과 일치해야 합니다")
		})
	}
}

// TestPerformance_Equals Equals() 메서드의 동작을 검증합니다.
func TestPerformance_Equals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		perf1    *performance
		perf2    *performance
		expected bool
	}{
		{
			name: "동일한 공연 (Title, Place 일치)",
			perf1: &performance{
				Title:     "뮤지컬 캣츠",
				Place:     "브로드웨이극장",
				Thumbnail: "thumb1.jpg",
			},
			perf2: &performance{
				Title:     "뮤지컬 캣츠",
				Place:     "브로드웨이극장",
				Thumbnail: "thumb2.jpg",
			},
			expected: true,
		},
		{
			name: "다른 공연 (Title 불일치)",
			perf1: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			perf2: &performance{
				Title: "뮤지컬 레미제라블",
				Place: "브로드웨이극장",
			},
			expected: false,
		},
		{
			name: "다른 공연 (Place 불일치)",
			perf1: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			perf2: &performance{
				Title: "뮤지컬 캣츠",
				Place: "샤롯데씨어터",
			},
			expected: false,
		},
		{
			name:  "첫 번째가 nil",
			perf1: nil,
			perf2: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			expected: false,
		},
		{
			name: "두 번째가 nil",
			perf1: &performance{
				Title: "뮤지컬 캣츠",
				Place: "브로드웨이극장",
			},
			perf2:    nil,
			expected: false,
		},
		{
			name:     "둘 다 nil",
			perf1:    nil,
			perf2:    nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.perf1.Equals(tt.perf2)
			assert.Equal(t, tt.expected, result, "Equals() 결과가 예상과 일치해야 합니다")
		})
	}
}

// TestPerformance_KeyAndEquals_Consistency Key()와 Equals()의 일관성을 검증합니다.
func TestPerformance_KeyAndEquals_Consistency(t *testing.T) {
	t.Parallel()

	perf1 := &performance{
		Title:     "뮤지컬 캣츠",
		Place:     "브로드웨이극장",
		Thumbnail: "thumb1.jpg",
	}
	perf2 := &performance{
		Title:     "뮤지컬 캣츠",
		Place:     "브로드웨이극장",
		Thumbnail: "thumb2.jpg",
	}
	perf3 := &performance{
		Title:     "뮤지컬 레미제라블",
		Place:     "브로드웨이극장",
		Thumbnail: "thumb3.jpg",
	}

	t.Run("Equals가 true이면 Key도 동일해야 함", func(t *testing.T) {
		assert.True(t, perf1.Equals(perf2), "perf1과 perf2는 동일해야 합니다")
		assert.Equal(t, perf1.Key(), perf2.Key(), "동일한 공연은 같은 키를 가져야 합니다")
	})

	t.Run("Equals가 false이면 Key도 달라야 함", func(t *testing.T) {
		assert.False(t, perf1.Equals(perf3), "perf1과 perf3는 다른 공연이어야 합니다")
		assert.NotEqual(t, perf1.Key(), perf3.Key(), "다른 공연은 다른 키를 가져야 합니다")
	})
}

// TestTask_DiffAndNotify 변경 감지 및 알림 생성 로직을 검증합니다. (핵심 로직)
func TestTask_DiffAndNotify(t *testing.T) {
	t.Parallel()

	// 테스트용 데이터 셋업
	perfA := &performance{Title: "A", Place: "Theater1"}
	perfB := &performance{Title: "B", Place: "Theater2"}

	tests := []struct {
		name              string
		current           []*performance
		prev              []*performance
		runBy             tasksvc.RunBy // 자동(Scheduler) vs 수동(User)
		expectMsgContains []string      // 메시지에 포함되어야 할 문자열들
		expectNilMsg      bool          // 메시지가 비어야 하는지
		expectSnapshot    bool          // 스냅샷 업데이트가 필요한지
	}{
		{
			name:              "신규 공연 발견 (A 추가)",
			current:           []*performance{perfA, perfB},
			prev:              []*performance{perfB},
			runBy:             tasksvc.RunByScheduler,
			expectMsgContains: []string{"새로운 공연정보가 등록되었습니다", "A", "🆕"},
			expectSnapshot:    true,
		},
		{
			name:           "변동 없음",
			current:        []*performance{perfA},
			prev:           []*performance{perfA},
			runBy:          tasksvc.RunByScheduler,
			expectNilMsg:   true,
			expectSnapshot: false,
		},
		{
			name:              "초기 실행 (Prev가 nil) - Scheduler",
			current:           []*performance{perfA},
			prev:              nil,
			runBy:             tasksvc.RunByScheduler,
			expectMsgContains: []string{"새로운 공연정보가 등록되었습니다", "A"},
			expectSnapshot:    true,
		},
		{
			name:              "사용자 수동 실행 - 변동 없어도 전체 목록 반환",
			current:           []*performance{perfA},
			prev:              []*performance{perfA},
			runBy:             tasksvc.RunByUser,
			expectMsgContains: []string{"현재 등록된 공연정보는 아래와 같습니다", "A"}, // 🆕 마크 없어야 함
			expectSnapshot:    false,
		},
		{
			name:              "사용자 수동 실행 - 데이터 없음",
			current:           []*performance{}, // Empty
			prev:              nil,
			runBy:             tasksvc.RunByUser,
			expectMsgContains: []string{"등록된 공연정보가 존재하지 않습니다"},
			expectSnapshot:    false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// *task 생성 (naver 패키지 내부이므로 접근 가능)
			// task 구조체는 tasksvc.Task 인터페이스를 임베딩합니다.
			// 실제 구현체인 BaseTask를 사용하여 RunBy만 설정하면 됩니다.
			baseTask := tasksvc.NewBaseTask("TEST_TASK", "TEST_CMD", "TEST_INSTANCE", "TEST_NOTIFIER", tt.runBy)

			testTask := &task{
				Task: baseTask,
			}

			currentSnap := &watchNewPerformancesSnapshot{Performances: tt.current}
			var prevSnap *watchNewPerformancesSnapshot
			if tt.prev != nil {
				prevSnap = &watchNewPerformancesSnapshot{Performances: tt.prev}
			}

			msg, newSnapData, err := testTask.diffAndNotify(currentSnap, prevSnap, false) // Text Mode Test

			assert.NoError(t, err)

			if tt.expectNilMsg {
				assert.Empty(t, msg)
				assert.Nil(t, newSnapData)
			} else {
				assert.NotEmpty(t, msg)
				for _, s := range tt.expectMsgContains {
					assert.Contains(t, msg, s)
				}

				if tt.expectSnapshot {
					assert.NotNil(t, newSnapData)
					// 스냅샷 데이터 검증
					snap, ok := newSnapData.(*watchNewPerformancesSnapshot)
					assert.True(t, ok)
					assert.Equal(t, len(tt.current), len(snap.Performances))
				} else {
					assert.Nil(t, newSnapData)
				}
			}
		})
	}
}

// TestTask_ExecuteWatchNewPerformances executeWatchNewPerformances 메서드의 통합 흐름을 테스트합니다.
// (Fetching -> Parsing -> Filtering)
func TestTask_ExecuteWatchNewPerformances(t *testing.T) {
	t.Parallel()

	// 테스트 데이터 생성 헬퍼
	makePerformanceHTML := func(title, place string) string {
		return fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">%s</strong><span class="sub_text">%s</span></div><div class="thumb"><img src="thumb.jpg"></div></div></li>`, title, place)
	}

	makeJSONResponse := func(htmlContent string) string {
		m := map[string]string{"html": htmlContent}
		b, _ := json.Marshal(m)
		return string(b)
	}

	tests := []struct {
		name            string
		settings        *watchNewPerformancesSettings
		mockResponses   map[string]string // URL Query -> HTML Body
		mockErrors      map[string]error  // URL Query -> Error
		expectedMessage []string          // 예상되는 알림 메시지 포함 문자열
		expectedError   string            // 예상되는 에러 메시지
		validate        func(t *testing.T, snapshot *watchNewPerformancesSnapshot)
	}{
		{
			name: "성공: 단일 페이지 수집 및 신규 공연 알림",
			settings: &watchNewPerformancesSettings{
				Query:    "뮤지컬",
				MaxPages: 1,
			},
			mockResponses: map[string]string{
				"u7=1": makeJSONResponse(fmt.Sprintf("<ul>%s</ul>", makePerformanceHTML("New Musical", "Seoul"))), // Page 1
			},
			expectedMessage: []string{"새로운 공연정보가 등록되었습니다", "New Musical", "Seoul"},
			validate: func(t *testing.T, snapshot *watchNewPerformancesSnapshot) {
				assert.Equal(t, 1, len(snapshot.Performances))
				assert.Equal(t, "New Musical", snapshot.Performances[0].Title)
			},
		},
		{
			name: "성공: 페이지네이션 (2페이지까지 수집)",
			settings: &watchNewPerformancesSettings{
				Query:    "콘서트",
				MaxPages: 2,
			},
			mockResponses: map[string]string{
				"u7=1": makeJSONResponse(fmt.Sprintf("<ul>%s</ul>", makePerformanceHTML("Concert 1", "Stadium"))), // Page 1
				"u7=2": makeJSONResponse(fmt.Sprintf("<ul>%s</ul>", makePerformanceHTML("Concert 2", "Hall"))),    // Page 2
			},
			expectedMessage: []string{"Concert 1", "Concert 2"},
			validate: func(t *testing.T, snapshot *watchNewPerformancesSnapshot) {
				assert.Equal(t, 2, len(snapshot.Performances))
			},
		},
		{
			name: "성공: 중복 데이터 제거 (페이지 밀림 현상 대응)",
			settings: &watchNewPerformancesSettings{
				Query:    "Overlap",
				MaxPages: 2,
			},
			mockResponses: map[string]string{
				"u7=1": makeJSONResponse(fmt.Sprintf("<ul>%s</ul>", makePerformanceHTML("Perf A", "Place A"))), // Page 1
				"u7=2": makeJSONResponse(fmt.Sprintf("<ul>%s%s</ul>",
					makePerformanceHTML("Perf A", "Place A"),   // Page 1 내용이 다시 넘어옴 (중복)
					makePerformanceHTML("Perf B", "Place B"))), // Page 2 신규
			},
			validate: func(t *testing.T, snapshot *watchNewPerformancesSnapshot) {
				assert.Equal(t, 2, len(snapshot.Performances), "중복된 Perf A는 하나만 저장되어야 합니다")
			},
		},
		{
			name: "실패: 네트워크 에러 발생",
			settings: &watchNewPerformancesSettings{
				Query: "ErrorCase",
			},
			mockErrors: map[string]error{
				"u7=1": fmt.Errorf("network timeout"),
			},
			expectedError: "network timeout",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Mock Fetcher 설정
			mockFetcher := testutil.NewMockHTTPFetcher()
			baseParams := url.Values{}
			// 기본 파라미터 (watch_new_performances.go 참조)
			baseParams.Set("key", "kbList")
			baseParams.Set("pkid", "269")
			baseParams.Set("where", "nexearch")
			baseParams.Set("u1", tt.settings.Query)
			baseParams.Set("u2", "all")
			baseParams.Set("u3", "")
			baseParams.Set("u4", "ingplan")
			baseParams.Set("u5", "date")
			baseParams.Set("u6", "N")
			baseParams.Set("u8", "all")
			// u7(Page)만 가변

			// Mock Response 등록
			for queryPart, body := range tt.mockResponses {
				// 쿼리 파라미터 조합
				// 주의: url.Values.Encode()는 키 정렬을 보장하므로 순서 문제 없음
				// 하지만 테스트 편의를 위해 전체 URL을 구성해야 함
				// 여기서는 간단히 하기 위해, 실제 코드와 동일한 방식으로 URL 생성 후 매핑

				// 실제 코드의 URL 생성 로직을 흉내내야 매칭 가능
				// 하지만 u7과 같은 페이지 번호는 동적이므로, 테스트 케이스의 queryPart (예: u7=1)를 파싱하여 병합

				fullParams := url.Values{} // 복사
				for k, v := range baseParams {
					fullParams[k] = v
				}

				// queryPart 파싱 (ex: u7=1)
				q, _ := url.ParseQuery(queryPart)
				for k, v := range q {
					fullParams[k] = v
				}

				fullURL := fmt.Sprintf("%s?%s", searchAPIBaseURL, fullParams.Encode())
				mockFetcher.SetResponse(fullURL, []byte(body))
			}

			// Mock Error 등록
			for queryPart, err := range tt.mockErrors {
				fullParams := url.Values{}
				for k, v := range baseParams {
					fullParams[k] = v
				}
				q, _ := url.ParseQuery(queryPart)
				for k, v := range q {
					fullParams[k] = v
				}
				fullURL := fmt.Sprintf("%s?%s", searchAPIBaseURL, fullParams.Encode())
				mockFetcher.SetError(fullURL, err) // 에러 설정
			}

			// Task 생성 및 설정
			if tt.settings.MaxPages == 0 {
				tt.settings.MaxPages = 50 // 기본값
			}
			if tt.settings.PageFetchDelay == 0 {
				tt.settings.PageFetchDelay = 1 // 테스트 속도를 위해 최소화
			}

			// executeWatchNewPerformances는 task 구조체의 메서드이므로 task 인스턴스 필요
			baseTask := tasksvc.NewBaseTask("NAVER", "WATCH", "INSTANCE", "NOTI", tasksvc.RunByScheduler)
			naverTask := &task{
				Task: baseTask,
			}
			naverTask.SetFetcher(mockFetcher)

			// 실행
			// prevSnapshot은 nil로 가정 (수집 테스트이므로)
			msg, resultData, err := naverTask.executeWatchNewPerformances(tt.settings, nil, false)

			// 검증
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				require.NoError(t, err)

				for _, expMsg := range tt.expectedMessage {
					assert.Contains(t, msg, expMsg)
				}

				if tt.validate != nil {
					snapshot, ok := resultData.(*watchNewPerformancesSnapshot)
					require.True(t, ok, "결과 데이터는 watchNewPerformancesSnapshot 타입이어야 합니다")
					tt.validate(t, snapshot)
				}
			}
		})
	}
}

// TestBuildSearchAPIURL buildSearchAPIURL 함수가 올바른 URL을 생성하는지 검증합니다.
func TestBuildSearchAPIURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		query        string
		page         int
		expectedVars map[string]string // 가변 파라미터 검증용
	}{
		{
			name:  "기본: 영문 검색어 및 1페이지",
			query: "musical",
			page:  1,
			expectedVars: map[string]string{
				"u1": "musical",
				"u7": "1",
			},
		},
		{
			name:  "인코딩: 한글 검색어 및 중간 페이지",
			query: "서울 뮤지컬",
			page:  5,
			expectedVars: map[string]string{
				"u1": "서울 뮤지컬", // url.Parse가 디코딩해주므로 평문 비교
				"u7": "5",
			},
		},
		{
			name:  "특수문자: URL 인코딩이 필요한 검색어",
			query: "Cats & Dogs",
			page:  10,
			expectedVars: map[string]string{
				"u1": "Cats & Dogs",
				"u7": "10",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotURLStr := buildSearchAPIURL(tt.query, tt.page)
			gotURL, err := url.Parse(gotURLStr)
			require.NoError(t, err, "생성된 URL은 유효한 형식이어야 합니다")

			// 1. Base URL 검증
			// searchAPIBaseURL 상수는 쿼리 파라미터를 포함하지 않는 순수 경로라고 가정
			expectedBaseURL, _ := url.Parse(searchAPIBaseURL)
			assert.Equal(t, expectedBaseURL.Scheme, gotURL.Scheme, "Scheme이 일치해야 합니다")
			assert.Equal(t, expectedBaseURL.Host, gotURL.Host, "Host가 일치해야 합니다")
			assert.Equal(t, expectedBaseURL.Path, gotURL.Path, "Path가 일치해야 합니다")

			// 2. 쿼리 파라미터 검증
			q := gotURL.Query()

			// 2-1. 고정 파라미터 검증 (Invariant)
			assert.Equal(t, "kbList", q.Get("key"), "key 파라미터 불일치")
			assert.Equal(t, "269", q.Get("pkid"), "pkid 파라미터 불일치")
			assert.Equal(t, "nexearch", q.Get("where"), "where 파라미터 불일치")
			assert.Equal(t, "all", q.Get("u2"), "u2 (장르) 파라미터 불일치")
			assert.Equal(t, "", q.Get("u3"), "u3 (날짜) 파라미터 불일치")
			assert.Equal(t, "ingplan", q.Get("u4"), "u4 (상태) 파라미터 불일치")
			assert.Equal(t, "date", q.Get("u5"), "u5 (정렬) 파라미터 불일치")
			assert.Equal(t, "N", q.Get("u6"), "u6 (성인여부) 파라미터 불일치")
			assert.Equal(t, "all", q.Get("u8"), "u8 (세부장르) 파라미터 불일치")

			// 2-2. 가변 파라미터 검증 (Variant)
			for k, v := range tt.expectedVars {
				assert.Equal(t, v, q.Get(k), "가변 파라미터 %s 불일치", k)
			}
		})
	}
}
