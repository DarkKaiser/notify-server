package naver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Unit Tests: Configuration & Filtering
// -----------------------------------------------------------------------------

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
			err := tt.config.Validate()
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

// TestNaverTask_Filtering_Behavior 은 문서화 차원에서 Naver Task의 키워드 매칭 규칙 예시를 나열합니다.
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
			matcher := strutil.NewKeywordMatcher(tt.included, tt.excluded)
			got := matcher.Match(tt.item)
			assert.Equal(t, tt.want, got)
		})
	}
}

// -----------------------------------------------------------------------------
// Component Tests: Parsing & Diff Logic
// -----------------------------------------------------------------------------

// TestParsePerformancesFromHTML HTML 파싱 로직의 정확성과 견고성을 검증합니다.
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
		filters       *keywordMatchers
		expectedCount int                                             // 키워드 매칭 후 예상 개수
		expectedRaw   int                                             // 키워드 매칭 전 raw 개수
		expectError   bool                                            // 에러 발생 여부
		validateItems func(t *testing.T, performances []*performance) // 세부 항목 검증
	}{
		{
			name:          "성공: 단일 항목 파싱",
			html:          fmt.Sprintf("<ul>%s</ul>", makeItem("Cats", "Broadway", "cats.jpg")),
			filters:       &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)}, // 필터 없음
			expectedCount: 1,
			expectedRaw:   1,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Cats", performances[0].Title)
				assert.Equal(t, "Broadway", performances[0].Place)
				assert.Contains(t, performances[0].Thumbnail, "cats.jpg")
			},
		},
		{
			name: "성공: 키워드 매칭 (Include)",
			html: fmt.Sprintf("<ul>%s%s</ul>",
				makeItem("Cats Musical", "Seoul", "1.jpg"),
				makeItem("Dog Show", "Seoul", "2.jpg")),
			filters: &keywordMatchers{
				TitleMatcher: strutil.NewKeywordMatcher([]string{"Musical"}, nil),
				PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
			},
			expectedCount: 1, // Cats only
			expectedRaw:   2,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Cats Musical", performances[0].Title)
			},
		},
		{
			name: "성공: 키워드 매칭 (Exclude)",
			html: fmt.Sprintf("<ul>%s%s</ul>",
				makeItem("Happy Musical", "Seoul", "1.jpg"),
				makeItem("Sad Drama", "Seoul", "2.jpg")),
			filters: &keywordMatchers{
				TitleMatcher: strutil.NewKeywordMatcher(nil, []string{"Drama"}),
				PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
			},
			expectedCount: 1, // Happy only
			expectedRaw:   2,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Happy Musical", performances[0].Title)
			},
		},
		{
			name: "성공: 키워드 매칭 (OR 조건 - A 또는 B)",
			html: fmt.Sprintf("<ul>%s%s%s</ul>",
				makeItem("Musical Cats", "Seoul", ""),
				makeItem("Musical Dogs", "Seoul", ""),
				makeItem("Musical Birds", "Seoul", "")),
			filters: &keywordMatchers{
				TitleMatcher: strutil.NewKeywordMatcher([]string{"Cats|Dogs"}, nil), // "Cats" OR "Dogs"
				PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
			},
			expectedCount: 2, // Cats, Dogs
			expectedRaw:   3,
			validateItems: func(t *testing.T, performances []*performance) {
				require.Len(t, performances, 2)
				assert.Equal(t, "Musical Cats", performances[0].Title)
				assert.Equal(t, "Musical Dogs", performances[1].Title)
			},
		},
		{
			name: "성공: 키워드 매칭 (복합 조건 - 포함 AND 제외)",
			html: fmt.Sprintf("<ul>%s%s</ul>",
				makeItem("Perfect Musical", "Seoul", ""),
				makeItem("Boring Musical", "Seoul", "")),
			filters: &keywordMatchers{
				TitleMatcher: strutil.NewKeywordMatcher([]string{"Musical"}, []string{"Boring"}), // Musical 포함 AND Boring 제외
				PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
			},
			expectedCount: 1, // Perfect Musical only
			expectedRaw:   2,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "Perfect Musical", performances[0].Title)
			},
		},
		{
			name: "성공: 키워드 매칭 (대소문자 및 공백 처리)",
			html: fmt.Sprintf("<ul>%s</ul>", makeItem("musical CATS", "Seoul", "")),
			filters: &keywordMatchers{
				TitleMatcher: strutil.NewKeywordMatcher([]string{"  cats  "}, nil), // 공백이 있어도 Trim 후 매칭, 대소문자 무시
				PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
			},
			expectedCount: 1,
			expectedRaw:   1,
			validateItems: func(t *testing.T, performances []*performance) {
				assert.Equal(t, "musical CATS", performances[0].Title)
			},
		},
		{
			name:        "실패: HTML 파싱 에러 (필수 요소 누락 - 제목)",
			html:        `<ul><li><div class="item"><div class="title_box"></div></div></li></ul>`, // strong.name 없음
			filters:     &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)},
			expectError: true,
		},
		{
			name:          "성공: 썸네일 누락 (Soft Fail)",
			html:          `<ul><li><div class="item"><div class="title_box"><strong class="name">T</strong><span class="sub_text">P</span></div></div></li></ul>`, // thumb 없음
			filters:       &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)},
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
			filters:       &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)},
			expectedCount: 0,
			expectedRaw:   0,
			expectError:   false,
		},
		{
			name:        "실패: 필수 속성 누락 (class=name 없음)",
			html:        `<ul><li><div class="item"><div class="title_box"><strong>NoClass</strong></div></div></li></ul>`,
			filters:     &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)},
			expectError: true, // selectorTitle = ".title_box .name" 이므로 매칭 실패 -> 에러
		},
		{
			name:        "실패: 필수 속성 누락 (class=sub_text 없음)",
			html:        `<ul><li><div class="item"><div class="title_box"><strong class="name">Title</strong></div></div></li></ul>`,
			filters:     &keywordMatchers{TitleMatcher: strutil.NewKeywordMatcher(nil, nil), PlaceMatcher: strutil.NewKeywordMatcher(nil, nil)},
			expectError: true, // selectorPlace = ".title_box .sub_text" 이므로 매칭 실패 -> 에러
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// parsePerformancesFromHTML은 (performances, rawCount, error) 반환
			taskInstance := &task{
				Base: provider.NewBase("T", "C", "I", "N", contract.TaskRunByScheduler, nil),
			}
			taskInstance.SetScraper(scraper.New(mocks.NewMockHTTPFetcher()))

			items, raw, err := taskInstance.parsePerformancesFromHTML(context.Background(), tt.html, tt.filters)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRaw, raw, "Raw count failed")
				assert.Equal(t, tt.expectedCount, len(items), "Filtered items count failed")
				if tt.validateItems != nil {
					tt.validateItems(t, items)
				}
			}
		})
	}
}

// TestCalculatePerformanceDiffs 신규 공연 식별 로직(Set Difference: Current - Previous)을 단위 테스트합니다.
func TestCalculatePerformanceDiffs(t *testing.T) {
	t.Parallel()

	// Helper: 간단한 Performance 객체 생성
	makePerf := func(title, place string) *performance {
		return &performance{Title: title, Place: place}
	}

	tests := []struct {
		name          string
		current       []*performance
		prev          []*performance
		expectedDiffs []performanceDiff // 예상되는 Diff 목록
	}{
		{
			name:    "신규 공연 발견 (순수 추가)",
			current: []*performance{makePerf("P1", "L1"), makePerf("P2", "L2")},
			prev:    []*performance{makePerf("P1", "L1")},
			expectedDiffs: []performanceDiff{
				{Type: eventNewPerformance, Performance: makePerf("P2", "L2")},
			},
		},
		{
			name:          "변동 없음",
			current:       []*performance{makePerf("P1", "L1")},
			prev:          []*performance{makePerf("P1", "L1")},
			expectedDiffs: nil, // 또는 Empty
		},
		{
			name:    "초기 실행 (Prev is nil) -> 모두 신규로 간주",
			current: []*performance{makePerf("P1", "L1")},
			prev:    nil,
			expectedDiffs: []performanceDiff{
				{Type: eventNewPerformance, Performance: makePerf("P1", "L1")},
			},
		},
		{
			name:          "공연 삭제 (Current에 없음) -> 현재 로직상 Diff 제외",
			current:       []*performance{},
			prev:          []*performance{makePerf("P1", "L1")},
			expectedDiffs: nil, // 삭제된 건은 알림 대상이 아님
		},
		{
			name:    "장소가 다른 동명의 공연 -> 다른 공연으로 취급 (Key = Title + Place)",
			current: []*performance{makePerf("Cats", "Seoul"), makePerf("Cats", "Busan")},
			prev:    []*performance{makePerf("Cats", "Seoul")},
			expectedDiffs: []performanceDiff{
				{Type: eventNewPerformance, Performance: makePerf("Cats", "Busan")},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			taskInstance := &task{
				Base: provider.NewBase("T", "C", "I", "N", contract.TaskRunByUser, nil),
			}

			currSnap := &watchNewPerformancesSnapshot{Performances: tt.current}
			var prevSnap *watchNewPerformancesSnapshot
			if tt.prev != nil {
				prevSnap = &watchNewPerformancesSnapshot{Performances: tt.prev}
			}

			prevPerformancesSet := make(map[string]bool)
			if prevSnap != nil {
				for _, p := range prevSnap.Performances {
					prevPerformancesSet[p.Key()] = true
				}
			}
			gotDiffs := taskInstance.calculatePerformanceDiffs(currSnap, prevPerformancesSet)

			assert.Equal(t, len(tt.expectedDiffs), len(gotDiffs), "Diff 개수가 일치해야 합니다")

			// 순서 무관하게 내용 검증 (Set 비교)
			// 실제 구현은 순서를 보장하지 않을 수 있으나 현재 append 순서대로임.
			// 정확성을 위해 각 요소 비교
			for i, want := range tt.expectedDiffs {
				got := gotDiffs[i]
				assert.Equal(t, want.Type, got.Type)
				assert.Equal(t, want.Performance.Title, got.Performance.Title)
				assert.Equal(t, want.Performance.Place, got.Performance.Place)
			}
		})
	}
}

// TestTask_RenderPerformanceDiffs 알림 메시지 생성 로직을 검증합니다. (Text vs HTML)
// 이 테스트는 renderPerformanceDiffs 메서드가 각 포맷(HTML, Text)에 맞춰 올바르게 렌더링하는지,
// 특히 신규 공연(eventNewPerformance)에 대해 Render 메서드를 사용하여 일관된 출력을 보장하는지 확인합니다.
func TestTask_RenderPerformanceDiffs(t *testing.T) {
	t.Parallel()

	// 테스트 데이터 준비
	p1 := &performance{Title: "Cats", Place: "Seoul"}
	diffNew := performanceDiff{Type: eventNewPerformance, Performance: p1}

	// 예상되는 링크 URL (Title 기반 생성)
	expectedLink := "https://search.naver.com/search.naver?query=Cats"

	tests := []struct {
		name         string
		diffs        []performanceDiff
		supportsHTML bool
		validate     func(t *testing.T, msg string)
	}{
		{
			name:         "Diff 없음 -> 빈 문자열 반환",
			diffs:        nil,
			supportsHTML: false,
			validate: func(t *testing.T, msg string) {
				assert.Empty(t, msg)
			},
		},
		{
			name:         "HTML 모드: 링크 태그 및 스타일 포함 확인",
			diffs:        []performanceDiff{diffNew},
			supportsHTML: true,
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, mark.New, "신규 마크가 포함되어야 합니다")
				assert.Contains(t, msg, fmt.Sprintf(`<a href="%s?query=Cats"><b>Cats</b></a>`, searchResultPageURL), "HTML 링크 포맷이 올바라야 합니다")
				assert.Contains(t, msg, "Seoul")
			},
		},
		{
			name:         "Text 모드: HTML 태그 제거 및 텍스트 포맷 확인",
			diffs:        []performanceDiff{diffNew},
			supportsHTML: false,
			validate: func(t *testing.T, msg string) {
				assert.Contains(t, msg, mark.New)
				assert.Contains(t, msg, "Cats")
				assert.Contains(t, msg, "Seoul")

				// Negative Assertion (Text 모드에서는 HTML 태그가 없어야 함)
				assert.NotContains(t, msg, "<a href=", "Text 모드에서는 앵커 태그가 없어야 합니다")
				assert.NotContains(t, msg, "<b>", "Text 모드에서는 볼드 태그가 없어야 합니다")
				assert.NotContains(t, msg, expectedLink, "Render 메서드는 Text 모드에서 URL을 직접 노출하지 않아야 합니다") // performance.go의 Render 구현 의도 확인
			},
		},
		{
			name: "복수 항목 렌더링 (구분자 확인)",
			diffs: []performanceDiff{
				{Type: eventNewPerformance, Performance: &performance{Title: "A", Place: "P1"}},
				{Type: eventNewPerformance, Performance: &performance{Title: "B", Place: "P2"}},
			},
			supportsHTML: false,
			validate: func(t *testing.T, msg string) {
				// 두 항목 사이에 줄바꿈 구분자가 있는지 확인
				assert.Contains(t, msg, "A")
				assert.Contains(t, msg, "B")
				// strings.Join이나 루프 내 구분자 처리가 올바른지 확인 (Code: sb.WriteString("\n\n"))
				assert.Contains(t, msg, "\n\n", "항목 간에는 개행 문자로 구분되어야 합니다")
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			taskInstance := &task{
				Base: provider.NewBase("T", "C", "I", "N", contract.TaskRunByUser, nil),
			}
			gotMsg := taskInstance.renderPerformanceDiffs(tt.diffs, tt.supportsHTML)

			if tt.validate != nil {
				tt.validate(t, gotMsg)
			}
		})
	}
}

// TestTask_AnalyzeAndReport_TableDriven 네이버 공연 알림 로직의 핵심인 analyzeAndReport 메서드를
// 다양한 시나리오(Table-Driven)를 통해 철저하게 검증합니다.
//
// [검증 범위]
// 1. 신규 공연 감지 (New Performance)
// 2. 실행 주체(User vs Scheduler)에 따른 알림 정책 차이
// 3. 데이터가 없을 때의 피드백
// 4. 불변식(Invariant) 검증: 변경 사항 존재 시 메시지 필히 생성
func TestTask_AnalyzeAndReport_TableDriven(t *testing.T) {
	t.Parallel()

	// 테스트용 더미 데이터 생성 헬퍼
	createPerf := func(id, title string) *performance {
		return &performance{
			Title: title,
			Place: "예술의전당",
		}
	}

	tests := []struct {
		name string
		// Input
		runBy               contract.TaskRunBy
		currentPerformances []*performance
		prevPerformances    []*performance
		supportsHTML        bool

		// Expected Output
		wantShouldSave    bool
		wantMsgContent    []string // 메시지에 반드시 포함되어야 할 텍스트 조각들
		wantMsgNotContent []string // 메시지에 절대 포함되어서는 안 될 텍스트 조각들
		wantEmptyMsg      bool     // 메시지가 아예 비어있어야 하는지
	}{
		{
			name:                "신규 공연 감지 (Scheduler) - 알림 발송 및 저장",
			runBy:               contract.TaskRunByScheduler,
			currentPerformances: []*performance{createPerf("1", "뮤지컬 영웅")},
			prevPerformances:    []*performance{}, // 이전 기록 없음 (완전 신규)
			supportsHTML:        false,
			wantShouldSave:      true,
			wantMsgContent:      []string{"새로운 공연정보가 등록되었습니다", "뮤지컬 영웅"},
			wantMsgNotContent:   []string{},
			wantEmptyMsg:        false,
		},
		{
			name:                "변경 없음 (Scheduler) - 침묵 (알림 X, 저장 X)",
			runBy:               contract.TaskRunByScheduler,
			currentPerformances: []*performance{createPerf("1", "뮤지컬 영웅")},
			prevPerformances:    []*performance{createPerf("1", "뮤지컬 영웅")}, // 동일 데이터
			supportsHTML:        false,
			wantShouldSave:      false,
			wantMsgContent:      []string{},
			wantMsgNotContent:   []string{},
			wantEmptyMsg:        true,
		},
		{
			name:                "변경 없음 (User) - 현황 보고 (알림 O, 저장 X)",
			runBy:               contract.TaskRunByUser,
			currentPerformances: []*performance{createPerf("1", "뮤지컬 영웅")},
			prevPerformances:    []*performance{createPerf("1", "뮤지컬 영웅")},
			supportsHTML:        false,
			wantShouldSave:      false, // 변경 사항 자체는 없으므로 저장할 필요 없음
			wantMsgContent:      []string{"신규로 등록된 공연정보가 없습니다", "현재 등록된 공연정보는 아래와 같습니다", "뮤지컬 영웅"},
			wantMsgNotContent:   []string{"새로운 공연정보가 등록되었습니다"},
			wantEmptyMsg:        false,
		},
		{
			name:                "데이터 없음 (User) - 안내 메시지 (알림 O, 저장 X)",
			runBy:               contract.TaskRunByUser,
			currentPerformances: []*performance{}, // 수집된 공연 0개
			prevPerformances:    []*performance{},
			supportsHTML:        false,
			wantShouldSave:      false,
			wantMsgContent:      []string{"등록된 공연정보가 존재하지 않습니다"},
			wantMsgNotContent:   []string{},
			wantEmptyMsg:        false,
		},
		{
			name:                "부분 신규 감지 (Scheduler) - 신규 건만 알림",
			runBy:               contract.TaskRunByScheduler,
			currentPerformances: []*performance{createPerf("1", "기존 공연"), createPerf("2", "신규 공연")},
			prevPerformances:    []*performance{createPerf("1", "기존 공연")},
			supportsHTML:        false,
			wantShouldSave:      true,
			wantMsgContent:      []string{"새로운 공연정보가 등록되었습니다", "신규 공연"},
			wantMsgNotContent:   []string{"기존 공연"}, // 변경 알림 메시지에는 신규 건만 나와야 함
			wantEmptyMsg:        false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup Task
			tsk := &task{
				Base: provider.NewBase("T", "C", "I", "N", tt.runBy, nil),
			}

			// Prepare Snapshots
			currentSnap := &watchNewPerformancesSnapshot{Performances: tt.currentPerformances}

			// Map 변환
			prevMap := make(map[string]bool)
			for _, p := range tt.prevPerformances {
				prevMap[p.Key()] = true
			}

			// Execute Logic
			msg, shouldSave := tsk.analyzeAndReport(currentSnap, prevMap, tt.supportsHTML)

			// Verification
			assert.Equal(t, tt.wantShouldSave, shouldSave, "ShouldSave 상태 불일치")

			if tt.wantEmptyMsg {
				assert.Empty(t, msg, "메시지가 비어있어야 합니다")
			} else {
				assert.NotEmpty(t, msg, "메시지가 생성되어야 합니다")
				for _, content := range tt.wantMsgContent {
					assert.Contains(t, msg, content, "메시지에 필수 내용 누락")
				}
				for _, notContent := range tt.wantMsgNotContent {
					assert.NotContains(t, msg, notContent, "메시지에 포함되지 말아야 할 내용 존재")
				}
			}

			// [Invariant Check]
			if shouldSave {
				assert.NotEmpty(t, msg, "[Invariant Failure] 변경 사항이 있어 저장(shouldSave=true)하려는데 알림 메시지가 없습니다.")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration Tests: Full Flow (Fetching -> Parsing -> Processing)
// -----------------------------------------------------------------------------

// TestTask_ExecuteWatchNewPerformances executeWatchNewPerformances 메서드의 통합 흐름을 테스트합니다.
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

	// Filter Test Settings
	settingsWithFilters := &watchNewPerformancesSettings{
		Query:    "FilterTest",
		MaxPages: 1,
	}
	settingsWithFilters.Filters.Title.IncludedKeywords = "Keep"
	settingsWithFilters.Filters.Title.ExcludedKeywords = "Drop"

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
		{
			name: "실패: HTML 파싱 에러 (필수 태그 누락)",
			settings: &watchNewPerformancesSettings{
				Query:    "ParseError",
				MaxPages: 1,
			},
			mockResponses: map[string]string{
				// 필수 태그(.title_box)는 있지만, 내부 필수 태그(.name)가 누락된 HTML
				// 그래야 parsePerformancesFromHTML 루프에 진입하고 에러를 반환함
				"u7=1": makeJSONResponse(`<ul><li><div class="item"><div class="title_box">NO_NAME</div></div></li></ul>`),
			},
			// fetchPerformances에서 parse error를 그대로 반환하거나 wrapping함
			expectedError: "HTML 구조 변경",
		},
		{
			name:     "성공: 통합 필터링 (키워드 매칭으로 일부 항목 제외)",
			settings: settingsWithFilters,
			mockResponses: map[string]string{
				"u7=1": makeJSONResponse(fmt.Sprintf("<ul>%s%s%s</ul>",
					makePerformanceHTML("Keep Item", "Seoul"),      // Match
					makePerformanceHTML("Keep Drop Item", "Seoul"), // Exclude (Contains 'Drop')
					makePerformanceHTML("Other Item", "Seoul"),     // Exclude (No 'Keep')
				)),
			},
			expectedMessage: []string{"Keep Item"},
			validate: func(t *testing.T, snapshot *watchNewPerformancesSnapshot) {
				require.Equal(t, 1, len(snapshot.Performances))
				assert.Equal(t, "Keep Item", snapshot.Performances[0].Title)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Mock Fetcher 설정
			mockFetcher := mocks.NewMockHTTPFetcher()
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
			baseTask := provider.NewBase("NAVER", "WATCH", "INSTANCE", "NOTI", contract.TaskRunByScheduler, nil)
			naverTask := &task{
				Base: baseTask,
			}
			naverTask.SetScraper(scraper.New(mockFetcher))
			// SetFetcher call removed

			// 실행
			// prevSnapshot은 nil로 가정 (수집 테스트이므로)
			msg, resultData, err := naverTask.executeWatchNewPerformances(context.Background(), tt.settings, nil, false)

			// 검증
			if tt.expectedError != "" {
				require.Error(t, err) // Error가 nil이면 여기서 멈춤 (Prevents panic)
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

// TestTask_FetchPerformances_Cancellation 작업 취소 시나리오를 검증합니다. (Concurrency)
func TestTask_FetchPerformances_Cancellation(t *testing.T) {
	t.Parallel()

	// 1. Setup
	mockFetcher := mocks.NewMockHTTPFetcher()

	// 첫 번째 페이지 요청에 500ms 지연을 설정합니다.
	// 이는 별도 고루틴에서 Cancel()을 호출할 충분한 시간을 벌어줍니다.
	delayedURL := buildSearchAPIURL("CancelTest", 1)
	mockFetcher.SetDelay(delayedURL, 500*time.Millisecond)
	mockFetcher.SetResponse(delayedURL, []byte(`{"html": "<ul><li>Delayed Item</li></ul>"}`))

	baseTask := provider.NewBase("NAVER", "WATCH", "INSTANCE", "NOTI", contract.TaskRunByUser, nil)
	naverTask := &task{
		Base: baseTask,
	}
	naverTask.SetScraper(scraper.New(mockFetcher))
	// SetFetcher call removed

	settings := &watchNewPerformancesSettings{
		Query:          "CancelTest",
		MaxPages:       5,
		PageFetchDelay: 10,
	}

	// 2. Execution (Async Cancel)
	errCh := make(chan error, 1)
	go func() {
		_, err := naverTask.fetchPerformances(context.Background(), settings)
		errCh <- err
	}()

	// 지연 시간(500ms)보다 짧은 시간(100ms) 후에 취소 요청을 보냅니다.
	time.Sleep(100 * time.Millisecond)
	naverTask.Cancel()

	// 3. Validation
	err := <-errCh
	assert.NoError(t, err, "취소 시 에러가 반환되지 않고 nil이어야 합니다 (Graceful Shutdown)")
	assert.True(t, naverTask.IsCanceled(), "Task 상태가 Canceled여야 합니다")

	// 요청이 실제로 취소되었는지 확인 (결과가 nil이어야 함)
	// fetchPerformances는 취소 시 nil, nil을 반환하도록 구현되어 있음
}

// TestTask_FetchPerformances_PaginationLimits 페이지네이션 한계 및 종료 조건을 검증합니다.
func TestTask_FetchPerformances_PaginationLimits(t *testing.T) {
	t.Parallel()

	makePageHTML := func(startIndex int, itemsCount int) string {
		var sb strings.Builder
		sb.WriteString("<ul>")
		for i := 0; i < itemsCount; i++ {
			idx := startIndex + i
			sb.WriteString(fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">Item %d</strong><span class="sub_text">Place %d</span></div><div class="thumb"><img src="t.jpg"></div></div></li>`, idx, idx))
		}
		sb.WriteString("</ul>")
		m := map[string]string{"html": sb.String()}
		b, _ := json.Marshal(m)
		return string(b)
	}

	tests := []struct {
		name            string
		maxPages        int
		mockResponses   []string // 순서대로 Page 1, 2, 3... 응답 본문
		expectedCallCnt int      // 예상되는 API 호출 횟수
		expectedItems   int      // 최종 수집된 아이템 수
	}{
		{
			name:            "MaxPages 도달 시 중단",
			maxPages:        2,
			mockResponses:   []string{makePageHTML(0, 1), makePageHTML(1, 1), makePageHTML(2, 1)}, // Item 0, Item 1, Item 2
			expectedCallCnt: 2,                                                                    // 2페이지까지만 호출하고 멈춰야 함 (loop 조건: pageIndex > maxPages break)
			expectedItems:   2,
		},
		{
			name:            "데이터 없는 페이지(RawCount=0) 도달 시 중단",
			maxPages:        10,
			mockResponses:   []string{makePageHTML(0, 1), makePageHTML(1, 0), makePageHTML(2, 1)}, // 2페이지가 비었음
			expectedCallCnt: 2,                                                                    // 2페이지(빈 결과)까지 확인하고 루프 종료
			expectedItems:   1,
		},
		{
			name:            "첫 페이지부터 비어있음",
			maxPages:        5,
			mockResponses:   []string{makePageHTML(0, 0)},
			expectedCallCnt: 1,
			expectedItems:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()
			for i, body := range tt.mockResponses {
				page := i + 1
				u := buildSearchAPIURL("LimitTest", page)
				mockFetcher.SetResponse(u, []byte(body))
			}

			// executeFlow
			baseTask := provider.NewBase("NAVER", "WATCH", "INSTANCE", "NOTI", contract.TaskRunByUser, nil)
			naverTask := &task{
				Base: baseTask,
			}
			naverTask.SetScraper(scraper.New(mockFetcher))
			// SetFetcher call removed

			settings := &watchNewPerformancesSettings{
				Query:          "LimitTest",
				MaxPages:       tt.maxPages,
				PageFetchDelay: 0, // No delay
			}

			items, err := naverTask.fetchPerformances(context.Background(), settings)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedItems, len(items))

			// 호출 횟수 검증 (RequestedURLs 이용) e.g. "u7=1", "u7=2" 포함 여부 확인
			// MockHttpFetcher의 GetRequestedURLs() 사용
			requested := mockFetcher.GetRequestedURLs()
			assert.Equal(t, tt.expectedCallCnt, len(requested), "API 호출 횟수가 예상과 달라야 합니다")
		})
	}
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

// BenchmarkTask_ParsePerformances 대량의 HTML 데이터에 대한 파싱 성능을 측정합니다.
func BenchmarkTask_ParsePerformances(b *testing.B) {
	// 50개의 아이템이 있는 HTML 생성
	var sb strings.Builder
	sb.WriteString("<ul>")
	for i := 0; i < 50; i++ {
		sb.WriteString(fmt.Sprintf(`<li><div class="item"><div class="title_box"><strong class="name">Performance %d</strong><span class="sub_text">Place %d</span></div><div class="thumb"><img src="thumb.jpg"></div></div></li>`, i, i))
	}
	sb.WriteString("</ul>")
	html := sb.String()

	filters := &keywordMatchers{
		TitleMatcher: strutil.NewKeywordMatcher(nil, nil),
		PlaceMatcher: strutil.NewKeywordMatcher(nil, nil),
	}

	b.ResetTimer()
	taskInstance := &task{
		Base: provider.NewBase("T", "C", "I", "N", contract.TaskRunByScheduler, nil),
	}
	taskInstance.SetScraper(scraper.New(mocks.NewMockHTTPFetcher()))

	for i := 0; i < b.N; i++ {
		_, _, _ = taskInstance.parsePerformancesFromHTML(context.Background(), html, filters)
	}
}

// BenchmarkTask_DiffAndNotify_Large 대량의 공연 데이터 비교 성능을 측정합니다.
func BenchmarkTask_DiffAndNotify_Large(b *testing.B) {
	count := 500
	prevItems := make([]*performance, count)
	currItems := make([]*performance, count)

	for i := 0; i < count; i++ {
		prevItems[i] = &performance{Title: fmt.Sprintf("Title%d", i), Place: "Place"}

		// 50%는 신규 아이템으로 교체
		if i >= count/2 {
			currItems[i] = &performance{Title: fmt.Sprintf("NewTitle%d", i), Place: "Place"}
		} else {
			currItems[i] = prevItems[i]
		}
	}

	baseTask := provider.NewBase("NAVER", "WATCH", "INSTANCE", "NOTI", contract.TaskRunByScheduler, nil)
	testTask := &task{
		Base: baseTask,
	}
	testTask.SetScraper(scraper.New(nil))

	prevSnap := &watchNewPerformancesSnapshot{Performances: prevItems}
	currSnap := &watchNewPerformancesSnapshot{Performances: currItems}

	prevPerformancesSet := make(map[string]bool)
	for _, p := range prevSnap.Performances {
		prevPerformancesSet[p.Key()] = true
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = testTask.analyzeAndReport(currSnap, prevPerformancesSet, false)
	}
}
