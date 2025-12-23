package navershopping

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Unit Tests: Settings & Domain Models
// -----------------------------------------------------------------------------

func TestWatchPriceSettings_Validate_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		settings  func() watchPriceSettings
		wantError string
	}{
		{
			name: "성공: 정상적인 설정",
			settings: func() watchPriceSettings {
				return NewSettingsBuilder().WithQuery("valid").WithPriceLessThan(10000).Build()
			},
			wantError: "",
		},
		{
			name: "실패: Query 누락",
			settings: func() watchPriceSettings {
				return NewSettingsBuilder().WithQuery("").WithPriceLessThan(10000).Build()
			},
			wantError: "query",
		},
		{
			name: "실패: Query 공백",
			settings: func() watchPriceSettings {
				return NewSettingsBuilder().WithQuery("   ").WithPriceLessThan(10000).Build()
			},
			wantError: "query",
		},
		{
			name: "실패: PriceLessThan 0 이하",
			settings: func() watchPriceSettings {
				return NewSettingsBuilder().WithQuery("valid").WithPriceLessThan(0).Build()
			},
			wantError: "price_less_than",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := tt.settings()
			err := s.validate()
			if tt.wantError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProduct_String_TableDriven(t *testing.T) {
	t.Parallel()

	p := NewProductBuilder().
		WithTitle("Test Product").
		WithLink("http://example.com").
		WithPrice(10000).
		WithMallName("Test Mall").
		Build()

	tests := []struct {
		name         string
		supportsHTML bool
		mark         string
		wants        []string
		unwants      []string
	}{
		{
			name:         "HTML - No Mark",
			supportsHTML: true,
			mark:         "",
			wants:        []string{"<a href=\"http://example.com\"><b>Test Product</b></a>", "(Test Mall)", "10,000원"},
			unwants:      []string{"Test Product (Test Mall) 10,000원 🆕"},
		},
		{
			name:         "HTML - With Mark",
			supportsHTML: true,
			mark:         " 🆕",
			wants:        []string{"<a href=\"http://example.com\"><b>Test Product</b></a>", "(Test Mall)", "10,000원 🆕"},
		},
		{
			name:         "Text - No Mark",
			supportsHTML: false,
			mark:         "",
			wants:        []string{"☞ Test Product (Test Mall) 10,000원", "http://example.com"},
			unwants:      []string{"<a href"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := p.String(tt.supportsHTML, tt.mark)
			for _, want := range tt.wants {
				assert.Contains(t, got, want)
			}
			for _, unwant := range tt.unwants {
				assert.NotContains(t, got, unwant)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration Tests: Fetch & Notify Logic
// -----------------------------------------------------------------------------

func TestTask_FetchProducts_TableDriven(t *testing.T) {
	t.Parallel()

	// 공통 설정
	defaultSettings := NewSettingsBuilder().
		WithQuery("test").
		WithPriceLessThan(20000).
		Build()

	// 예상되는 호출 URL (Key 정렬: display, query, sort, start)
	expectedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

	tests := []struct {
		name        string
		settings    watchPriceSettings
		mockSetup   func(*testutil.MockHTTPFetcher)
		checkResult func(*testing.T, []*product, error)
	}{
		{
			name:     "성공: 정상적인 데이터 수집 및 키워드 매칭",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{
					Total: 3, Items: []*searchResponseItem{
						{Title: "Keep", Link: "L1", LowPrice: "10000", ProductID: "1"},
						{Title: "FilterPrice", Link: "L2", LowPrice: "30000", ProductID: "2"},   // 20000 초과
						{Title: "FilterKeyword", Link: "L3", LowPrice: "10000", ProductID: "3"}, // 제외 키워드 시나리오
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				// defaultSettings에는 제외 키워드가 없으므로 가격 필터만 적용됨. (3개 중 1개 제외 -> 2개 남음)
				require.Len(t, p, 2)
				assert.Equal(t, "Keep", p[0].Title)
				assert.Equal(t, "FilterKeyword", p[1].Title)
			},
		},
		{
			name:     "성공: 제외 키워드 적용",
			settings: NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).WithExcludedKeywords("Exclude").Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{
					Total: 2, Items: []*searchResponseItem{
						{Title: "Keep", Link: "L1", LowPrice: "10000", ProductID: "1"},
						{Title: "Exclude Me", Link: "L2", LowPrice: "10000", ProductID: "2"},
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, "Keep", p[0].Title)
			},
		},
		{
			name:     "성공: 가격 쉼표 파싱",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{Total: 1, Items: []*searchResponseItem{{Title: "Comma", LowPrice: "1,500", ProductID: "1"}}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, 1500, p[0].LowPrice)
			},
		},
		{
			name:     "성공: 빈 결과",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{Total: 0, Items: []*searchResponseItem{}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				assert.Empty(t, p)
			},
		},
		{
			name:     "실패: API 호출 에러",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				m.SetError(expectedURL, errors.New("network fail"))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "network fail")
			},
		},
		{
			name:     "성공: 잘못된 가격 형식 무시",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{Total: 1, Items: []*searchResponseItem{{Title: "BadPrice", LowPrice: "Free", ProductID: "1"}}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				assert.Empty(t, p, "가격 파싱에 실패한 항목은 제외되어야 함")
			},
		},
		{
			name:     "성공: HTML 태그가 포함된 로우 데이터 키워드 매칭",
			settings: NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).WithExcludedKeywords("S25 FE").Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				resp := searchResponse{
					Total: 2, Items: []*searchResponseItem{
						{Title: "Galaxy <b>S25</b> <b>FE</b>", Link: "L1", LowPrice: "10000", ProductID: "1"}, // 제외 대상
						{Title: "Galaxy S25 Plus", Link: "L2", LowPrice: "10000", ProductID: "2"},             // 수집 대상
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1, "제외 키워드 'S25 FE'가 HTML 태그를 무시하고 적용되어야 함")
				assert.Equal(t, "Galaxy S25 Plus", p[0].Title)
			},
		},
		{
			name:     "실패: 잘못된 JSON 응답 (Malformed)",
			settings: defaultSettings,
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				m.SetResponse(expectedURL, []byte(`{invalid_json`))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "JSON")
			},
		},
		{
			name:     "성공: URL 인코딩 검증 (특수문자 쿼리)",
			settings: NewSettingsBuilder().WithQuery("아이폰 & 케이스").WithPriceLessThan(20000).Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				// 예상되는 인코딩된 URL
				encodedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%EC%95%84%EC%9D%B4%ED%8F%B0+%26+%EC%BC%80%EC%9D%B4%EC%8A%A4&sort=sim&start=1"
				resp := searchResponse{Total: 1, Items: []*searchResponseItem{{Title: "Case", LowPrice: "5000", ProductID: "1"}}}
				m.SetResponse(encodedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
			},
		},
		{
			name:     "성공: 키워드 매칭 (OR 조건 - A 또는 B 포함)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords("Galaxy|iPhone").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := searchResponse{
					Total: 3, Items: []*searchResponseItem{
						{Title: "Galaxy S25", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (Galaxy)
						{Title: "iPhone 16", Link: "L2", LowPrice: "1000", ProductID: "2"},  // 매칭 (iPhone)
						{Title: "Pixel 9", Link: "L3", LowPrice: "1000", ProductID: "3"},    // 미매칭
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 2)
				assert.Equal(t, "Galaxy S25", p[0].Title)
				assert.Equal(t, "iPhone 16", p[1].Title)
			},
		},
		{
			name:     "성공: 키워드 매칭 (복합 조건 - 포함 AND 제외)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords("Case").WithExcludedKeywords("Silicon,Hard").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := searchResponse{
					Total: 4, Items: []*searchResponseItem{
						{Title: "Leather Case", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (Case 포함, 제외어 없음)
						{Title: "Silicon Case", Link: "L2", LowPrice: "1000", ProductID: "2"}, // 제외 (Silicon)
						{Title: "Hard Case", Link: "L3", LowPrice: "1000", ProductID: "3"},    // 제외 (Hard)
						{Title: "Metal Bumper", Link: "L4", LowPrice: "1000", ProductID: "4"}, // 미매칭 (Case 미포함)
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, "Leather Case", p[0].Title)
			},
		},
		{
			name:     "성공: 키워드 매칭 (대소문자 혼합 및 공백 처리)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords(" apple watch | galaxy TAB ").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *testutil.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := searchResponse{
					Total: 3, Items: []*searchResponseItem{
						{Title: "Apple Watch Series 9", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (apple watch)
						{Title: "Galaxy Tab S9", Link: "L2", LowPrice: "1000", ProductID: "2"},        // 매칭 (galaxy TAB)
						{Title: "Galaxy Watch 6", Link: "L3", LowPrice: "1000", ProductID: "3"},       // 미매칭
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 2)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := testutil.NewMockHTTPFetcher()
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			tsk := &task{clientID: "id", clientSecret: "secret"}
			tsk.SetFetcher(mockFetcher)

			got, err := tsk.fetchProducts(&tt.settings)
			tt.checkResult(t, got, err)
		})
	}
}

func TestTask_DiffAndNotify_TableDriven(t *testing.T) {
	t.Parallel()

	// Base settings
	settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).Build()

	// Fixtures
	p1 := NewProductBuilder().WithID("1").WithPrice(10000).WithTitle("P1").Build()
	p1Same := NewProductBuilder().WithID("1").WithPrice(10000).WithTitle("P1").Build()
	p1Cheap := NewProductBuilder().WithID("1").WithPrice(9000).WithLink("L_NEW").WithTitle("P1").Build() // Price Drop + Link Change
	p1Expensive := NewProductBuilder().WithID("1").WithPrice(11000).WithTitle("P1").Build()
	p2 := NewProductBuilder().WithID("2").WithPrice(5000).WithTitle("P2").Build()

	tests := []struct {
		name         string
		runBy        tasksvc.RunBy
		currentItems []*product
		prevItems    []*product
		checkMsg     func(*testing.T, string, interface{}, error)
	}{
		{
			name:         "신규 상품 (New)",
			runBy:        tasksvc.RunByScheduler,
			currentItems: []*product{p1, p2},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "상품의 정보가 변경되었습니다")
				assert.Contains(t, msg, "P2")
				assert.Contains(t, msg, "🆕")
				assert.NotNil(t, data)
			},
		},
		{
			name:         "가격 하락 & Stale Link (Change)",
			runBy:        tasksvc.RunByScheduler,
			currentItems: []*product{p1Cheap},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "변경되었습니다")
				assert.Contains(t, msg, "9,000원")
				assert.Contains(t, msg, "(이전: 10,000원)")
				assert.Contains(t, msg, "L_NEW") // Stale Link Check: 최신 링크 사용 여부
				assert.NotNil(t, data)
			},
		},
		{
			name:         "가격 상승",
			runBy:        tasksvc.RunByScheduler,
			currentItems: []*product{p1Expensive},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "11,000원")
				assert.NotNil(t, data)
			},
		},
		{
			name:         "변경 없음 (Scheduler)",
			runBy:        tasksvc.RunByScheduler,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Empty(t, msg)
				assert.Nil(t, data)
			},
		},
		{
			name:         "변경 없음 (User)",
			runBy:        tasksvc.RunByUser,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "변경된 정보가 없습니다")
				assert.Nil(t, data)
			},
		},
		{
			name:         "결과 없음 (User)",
			runBy:        tasksvc.RunByUser,
			currentItems: []*product{},
			prevItems:    []*product{},
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "상품이 존재하지 않습니다")
			},
		},
		{
			name:         "최초 실행 (Prev is Nil)",
			runBy:        tasksvc.RunByScheduler,
			currentItems: []*product{p1},
			prevItems:    nil,
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.Contains(t, msg, "변경되었습니다")
			},
		},
		{
			name:  "정렬 검증 (가격 오름차순 -> 이름 오름차순)",
			runBy: tasksvc.RunByUser, // 결과 목록을 보기 위해 User 실행 모드 사용
			currentItems: []*product{
				NewProductBuilder().WithPrice(20000).WithTitle("B").Build(),
				NewProductBuilder().WithPrice(10000).WithTitle("A").Build(),
				NewProductBuilder().WithPrice(10000).WithTitle("C").Build(),
			},
			prevItems: nil,
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				// 메시지에 순서대로 나타나는지 확인 (10000원 A -> 10000원 C -> 20000원 B)
				// strings.Index로 위치 비교
				idxA := strings.Index(msg, "A")
				idxB := strings.Index(msg, "B")
				idxC := strings.Index(msg, "C")

				assert.Greater(t, idxA, -1)
				assert.Greater(t, idxB, -1)
				assert.Greater(t, idxC, -1)

				assert.Less(t, idxA, idxC, "같은 가격일 때 이름순(A->C)이어야 함")
				assert.Less(t, idxC, idxB, "가격 낮은 순(10000->20000)이어야 함")
			},
		},
		{
			name:  "대량 데이터 처리 (Benchmarks Memory Safety)",
			runBy: tasksvc.RunByScheduler,
			currentItems: func() []*product {
				items := make([]*product, 1000)
				for i := 0; i < 1000; i++ {
					items[i] = NewProductBuilder().WithID(fmt.Sprintf("%d", i)).WithPrice(1000 + i).WithTitle("Item").Build()
				}
				return items
			}(),
			prevItems: nil,
			checkMsg: func(t *testing.T, msg string, data interface{}, err error) {
				require.NoError(t, err)
				assert.NotEmpty(t, msg) // Panic 없이 메시지 생성 여부만 확인
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Task 생성 및 RunBy 설정
			tsk := &task{}
			tsk.Task = tasksvc.NewBaseTask("NS", "CMD", "INS", "NOTI", tt.runBy)

			current := &watchPriceSnapshot{Products: tt.currentItems}
			var prev *watchPriceSnapshot
			if tt.prevItems != nil {
				prev = &watchPriceSnapshot{Products: tt.prevItems}
			}

			msg, data, err := tsk.diffAndNotify(&settings, current, prev, false)
			tt.checkMsg(t, msg, data, err)
		})
	}
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type SettingsBuilder struct {
	settings watchPriceSettings
}

func NewSettingsBuilder() *SettingsBuilder {
	return &SettingsBuilder{}
}

func (b *SettingsBuilder) WithQuery(q string) *SettingsBuilder {
	b.settings.Query = q
	return b
}
func (b *SettingsBuilder) WithPriceLessThan(p int) *SettingsBuilder {
	b.settings.Filters.PriceLessThan = p
	return b
}
func (b *SettingsBuilder) WithIncludedKeywords(k string) *SettingsBuilder {
	b.settings.Filters.IncludedKeywords = k
	return b
}
func (b *SettingsBuilder) WithExcludedKeywords(k string) *SettingsBuilder {
	b.settings.Filters.ExcludedKeywords = k
	return b
}
func (b *SettingsBuilder) Build() watchPriceSettings {
	return b.settings
}

type ProductBuilder struct {
	product product
}

func NewProductBuilder() *ProductBuilder {
	return &ProductBuilder{
		product: product{
			Title:     "Default Title",
			Link:      "http://default.com",
			LowPrice:  1000,
			MallName:  "Naver",
			ProductID: "12345",
		},
	}
}

func (b *ProductBuilder) WithID(id string) *ProductBuilder {
	b.product.ProductID = id
	return b
}
func (b *ProductBuilder) WithTitle(t string) *ProductBuilder {
	b.product.Title = t
	return b
}
func (b *ProductBuilder) WithPrice(p int) *ProductBuilder {
	b.product.LowPrice = p
	return b
}
func (b *ProductBuilder) WithLink(l string) *ProductBuilder {
	b.product.Link = l
	return b
}
func (b *ProductBuilder) WithMallName(m string) *ProductBuilder {
	b.product.MallName = m
	return b
}
func (b *ProductBuilder) Build() *product {
	return &b.product
}

// -----------------------------------------------------------------------------
// Component Tests: MapToProduct (Granular Logic)
// -----------------------------------------------------------------------------

func TestTask_MapToProduct_TableDriven(t *testing.T) {
	t.Parallel()

	// Helper for clean tests
	item := func(title, price string) *searchResponseItem {
		return &searchResponseItem{
			Title:     title,
			LowPrice:  price,
			ProductID: "1",
			Link:      "http://link",
			MallName:  "mall",
		}
	}

	tests := []struct {
		name          string
		item          *searchResponseItem
		wantProduct   bool
		expectedTitle string // 변환 후 기대되는 Title (plain text)
	}{
		{
			name:          "성공: 정상적인 상품 데이터 변환",
			item:          item("Apple iPad", "50000"),
			wantProduct:   true,
			expectedTitle: "Apple iPad",
		},
		{
			name:          "성공: 가격 쉼표 처리",
			item:          item("Apple iPad", "50,000"),
			wantProduct:   true,
			expectedTitle: "Apple iPad",
		},
		{
			name:          "성공: HTML 태그 제거 (Sanitization)",
			item:          item("<b>Apple</b> iPad <b>Pro</b>", "100000"),
			wantProduct:   true,
			expectedTitle: "Apple iPad Pro",
		},
		{
			name:          "실패: 가격 파싱 오류 (Invalid Number)",
			item:          item("Apple iPad", "Call for Price"),
			wantProduct:   false,
			expectedTitle: "",
		},
		{
			name:          "실패: 가격 파싱 오류 (Empty String)",
			item:          item("Free Item", ""),
			wantProduct:   false,
			expectedTitle: "",
		},
		{
			name:          "성공: 유니코드 및 특수문자 처리",
			item:          item("특가! ★Galaxy★ S25 Ultra", "1500000"),
			wantProduct:   true,
			expectedTitle: "특가! ★Galaxy★ S25 Ultra",
		},
		{
			name:          "성공: 공백만 있는 필드 처리 (Trimmed result check needed if validation existed, but parser allows currently)",
			item:          item("   ", "100"),
			wantProduct:   true,
			expectedTitle: "   ", // 현재 로직상 Trim은 수행하지 않음 (strutil.StripHTMLTags에 의존)
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tsk := &task{}
			got := tsk.mapToProduct(tt.item)

			if tt.wantProduct {
				require.NotNil(t, got)
				assert.Equal(t, tt.expectedTitle, got.Title, "HTML 태그가 제거된 Plain Title이어야 합니다")
				// 추가적인 필드 검증
				assert.Equal(t, tt.item.Link, got.Link)
				assert.Equal(t, tt.item.MallName, got.MallName)
			} else {
				assert.Nil(t, got)
			}
		})
	}
}

func TestTask_IsPriceEligible_TableDriven(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		price         int
		priceLessThan int
		want          bool
	}{
		{
			name:          "성공: 가격 조건 만족",
			price:         50000,
			priceLessThan: 100000,
			want:          true,
		},
		{
			name:          "실패: 가격 초과 (Price Limit)",
			price:         150000,
			priceLessThan: 100000,
			want:          false,
		},
		{
			name:          "실패: 가격 상한가와 동일 (Boundary)",
			price:         100000,
			priceLessThan: 100000,
			want:          false, // '<' 조건이므로 false
		},
		{
			name:          "실패: 유효하지 않은 가격 (Zero)",
			price:         0,
			priceLessThan: 100000,
			want:          false,
		},
		{
			name:          "실패: 유효하지 않은 가격 (Negative)",
			price:         -100,
			priceLessThan: 100000,
			want:          false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tsk := &task{}
			got := tsk.isPriceEligible(tt.price, tt.priceLessThan)

			assert.Equal(t, tt.want, got)
		})
	}
}

// -----------------------------------------------------------------------------
// Advanced Scenarios: Pagination & Cancellation
// -----------------------------------------------------------------------------

func TestTask_FetchProducts_Pagination(t *testing.T) {
	t.Parallel()

	// 시나리오: 총 150개 상품, 1 페이지당 100개 요청.
	// 1번 요청: Start=1, Display=100 -> 100개 반환 (Next Start=101)
	// 2번 요청: Start=101, Display=100 -> 50개 반환 (Total=150 달성)

	settings := NewSettingsBuilder().WithQuery("paging").WithPriceLessThan(999999).Build()

	mockFetcher := testutil.NewMockHTTPFetcher()

	// Page 1 Setup
	page1URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=1"
	page1Items := make([]*searchResponseItem, 100)
	for i := 0; i < 100; i++ {
		page1Items[i] = &searchResponseItem{Title: "P1", LowPrice: "100", ProductID: "P1"}
	}
	mockFetcher.SetResponse(page1URL, mustMarshal(searchResponse{
		Total: 150, Start: 1, Display: 100, Items: page1Items,
	}))

	// Page 2 Setup
	page2URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=101"
	page2Items := make([]*searchResponseItem, 50)
	for i := 0; i < 50; i++ {
		page2Items[i] = &searchResponseItem{Title: "P2", LowPrice: "100", ProductID: "P2"}
	}
	mockFetcher.SetResponse(page2URL, mustMarshal(searchResponse{
		Total: 150, Start: 101, Display: 50, Items: page2Items,
	}))

	tsk := &task{clientID: "id", clientSecret: "secret"}
	tsk.SetFetcher(mockFetcher)

	products, err := tsk.fetchProducts(&settings)

	require.NoError(t, err)
	assert.Len(t, products, 150, "총 150개의 상품이 수집되어야 합니다")
}

func TestTask_FetchProducts_Cancellation(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("cancel").WithPriceLessThan(999999).Build()
	mockFetcher := testutil.NewMockHTTPFetcher()

	// 1페이지 응답 설정 (Total이 많아서 다음 페이지가 필요하도록 설정)
	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=cancel&sort=sim&start=1"
	mockFetcher.SetResponse(url, mustMarshal(searchResponse{
		Total: 1000, Start: 1, Display: 1, Items: []*searchResponseItem{{Title: "A", LowPrice: "100", ProductID: "1"}},
	}))

	// Task 생성 및 취소 상태로 설정
	tsk := &task{clientID: "id", clientSecret: "secret"}
	tsk.Task = tasksvc.NewBaseTask("NS", "CMD", "INS", "NOTI", tasksvc.RunByScheduler)
	tsk.SetFetcher(mockFetcher)

	// 강제로 취소 상태 주입 (Context Cancel)
	tsk.Cancel()

	products, err := tsk.fetchProducts(&settings)

	// 취소되었으므로 nil 반환 체크
	require.NoError(t, err)
	assert.Nil(t, products, "작업 취소 시 nil을 반환해야 합니다")
}
