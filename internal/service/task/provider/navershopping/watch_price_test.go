package navershopping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
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
			err := s.Validate()
			if tt.wantError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				assert.NoError(t, err)
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
		mockSetup   func(*mocks.MockHTTPFetcher)
		checkResult func(*testing.T, []*product, error)
	}{
		{
			name:     "성공: 정상적인 데이터 수집 및 키워드 매칭",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 2, Items: []*productSearchResponseItem{
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "Comma", LowPrice: "1,500", ProductID: "1"}}}
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 0, Items: []*productSearchResponseItem{}}
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "BadPrice", LowPrice: "Free", ProductID: "1"}}}
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 2, Items: []*productSearchResponseItem{
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// 예상되는 인코딩된 URL
				encodedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%EC%95%84%EC%9D%B4%ED%8F%B0+%26+%EC%BC%80%EC%9D%B4%EC%8A%A4&sort=sim&start=1"
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "Case", LowPrice: "5000", ProductID: "1"}}}
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 4, Items: []*productSearchResponseItem{
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
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
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

			mockFetcher := mocks.NewMockHTTPFetcher()
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			tsk := &task{
				Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
				clientID:     "id",
				clientSecret: "secret",
			}
			// SetFetcher call removed as it's deprecated

			got, err := tsk.fetchProducts(context.Background(), &tt.settings)
			tt.checkResult(t, got, err)
		})
	}
}

// TestTask_FetchProducts_URLVerification fetchProducts 메서드 호출 시
// 내부적으로 생성되는 URL이 파라미터(쿼리, 페이징 등)에 따라 올바른지 집중적으로 검증합니다.
func TestTask_FetchProducts_URLVerification(t *testing.T) {
	t.Parallel()

	defaultResponse := mustMarshal(productSearchResponse{Total: 0, Items: []*productSearchResponseItem{}})

	tests := []struct {
		name        string
		settings    watchPriceSettings
		expectedURL string
	}{
		{
			name:        "기본 URL 생성 확인",
			settings:    NewSettingsBuilder().WithQuery("macbook").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=macbook&sort=sim&start=1",
		},
		{
			name:        "특수문자 쿼리 인코딩 확인",
			settings:    NewSettingsBuilder().WithQuery("A&B").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=A%26B&sort=sim&start=1",
		},
		{
			name:        "공백 쿼리 인코딩 확인",
			settings:    NewSettingsBuilder().WithQuery("macbook air").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=macbook+air&sort=sim&start=1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()
			// 어떤 URL이든 성공 응답을 주도록 설정 (URL 검증이 목적이므로 내용은 무관)
			mockFetcher.SetResponse(tt.expectedURL, defaultResponse)

			tsk := &task{
				Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
				clientID:     "id",
				clientSecret: "secret",
			}
			// SetFetcher call removed as it's deprecated

			_, err := tsk.fetchProducts(context.Background(), &tt.settings)

			// URL 불일치 시 SetResponse에 없는 URL을 요청하게 되므로 에러 발생 ("no mock response found")
			// 따라서 에러가 없으면 URL이 정확하다는 뜻입니다.
			assert.NoError(t, err, "요청된 URL이 기대값과 다릅니다")
		})
	}
}

// TestTask_AnalyzeAndReport_TableDriven 네이버 쇼핑 알림 로직의 핵심인 analyzeAndReport 메서드를
// 다양한 시나리오(Table-Driven)를 통해 철저하게 검증합니다.
func TestTask_AnalyzeAndReport_TableDriven(t *testing.T) {
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
		runBy        contract.TaskRunBy
		currentItems []*product
		prevItems    []*product
		checkMsg     func(*testing.T, string, bool)
	}{
		{
			name:         "신규 상품 (New)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1, p2},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "상품 정보가 변경되었습니다")
				assert.Contains(t, msg, "P2")
				assert.Contains(t, msg, mark.New.WithSpace())
				assert.True(t, shouldSave)
			},
		},
		{
			name:         "가격 하락 & Stale Link (Change)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1Cheap},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "변경되었습니다")
				assert.Contains(t, msg, "9,000원")
				assert.Contains(t, msg, "(이전: 10,000원)")
				assert.Contains(t, msg, "L_NEW") // Stale Link Check: 최신 링크 사용 여부
				assert.True(t, shouldSave)
			},
		},
		{
			name:         "가격 상승",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1Expensive},
			prevItems:    []*product{p1},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "11,000원")
				assert.True(t, shouldSave)
			},
		},
		{
			name:         "변경 없음 (Scheduler)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Empty(t, msg)
				assert.False(t, shouldSave)
			},
		},
		{
			name:         "변경 없음 (User)",
			runBy:        contract.TaskRunByUser,
			currentItems: []*product{p1},
			prevItems:    []*product{p1Same},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "변경된 정보가 없습니다")
				assert.False(t, shouldSave)
			},
		},
		{
			name:         "결과 없음 (User)",
			runBy:        contract.TaskRunByUser,
			currentItems: []*product{},
			prevItems:    []*product{},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "상품이 존재하지 않습니다")
			},
		},
		{
			name:         "최초 실행 (Prev is Nil)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: []*product{p1},
			prevItems:    nil,
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.Contains(t, msg, "변경되었습니다")
			},
		},
		{
			name:  "정렬 검증 (가격 오름차순 -> 이름 오름차순)",
			runBy: contract.TaskRunByUser, // 결과 목록을 보기 위해 User 실행 모드 사용
			currentItems: []*product{
				NewProductBuilder().WithPrice(20000).WithTitle("B").Build(),
				NewProductBuilder().WithPrice(10000).WithTitle("A").Build(),
				NewProductBuilder().WithPrice(10000).WithTitle("C").Build(),
			},
			prevItems: nil,
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				// 메시지에 순서대로 나타나는지 확인 (10000원 A -> 10000원 C -> 20000원 B)
				// strings.Index로 위치 비교
				idxA := strings.Index(msg, "A")
				idxB := strings.Index(msg, "B")
				idxC := strings.Index(msg, "C")

				assert.Greater(t, idxA, -1)
				assert.Greater(t, idxB, -1)
				assert.Greater(t, idxC, -1)

				assert.Less(t, idxA, idxC, "같은 가격일 때 이름순(A->C)이어야 함")
				assert.Less(t, idxC, idxB, "가격 오름차순(10000->20000)이어야 함")
			},
		},
		{
			name:         "대량 데이터 처리 (Benchmarks Memory Safety)",
			runBy:        contract.TaskRunByScheduler,
			currentItems: makeMockProducts(100), // 100개 동시 변경
			prevItems:    []*product{},
			checkMsg: func(t *testing.T, msg string, shouldSave bool) {
				assert.True(t, shouldSave)
				assert.Contains(t, msg, "Product 99")
				// 너무 긴 메시지 생성을 제한하는 로직이 있다면 여기서 검증 가능
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Task 생성 및 RunBy 설정
			tsk := &task{
				Base: provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "T", CommandID: "C", NotifierID: "N", RunBy: tt.runBy}, InstanceID: "I", Fetcher: mocks.NewMockHTTPFetcher(), NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			}

			current := &watchPriceSnapshot{Products: tt.currentItems}
			var prev *watchPriceSnapshot
			if tt.prevItems != nil {
				prev = &watchPriceSnapshot{Products: tt.prevItems}
			}

			msg, shouldSave := tsk.analyzeAndReport(&settings, current, prev, false)
			tt.checkMsg(t, msg, shouldSave)

			// [Invariant Check] 전문가 수준의 방어적 테스트
			// "변경 사항을 저장해야 한다면(shouldSave=true), 반드시 알림 메시지가 존재해야 한다(msg != "")"
			// 이는 시스템의 데이터 무결성을 보장하는 핵심 불변식입니다.
			if shouldSave {
				assert.NotEmpty(t, msg, "Invariant Violation: shouldSave is true but message is empty")
			}
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
	item := func(title, price string) *productSearchResponseItem {
		return &productSearchResponseItem{
			Title:     title,
			LowPrice:  price,
			ProductID: "1",
			Link:      "http://link",
			MallName:  "mall",
		}
	}

	tests := []struct {
		name          string
		item          *productSearchResponseItem
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
			expectedTitle: "   ", // 현재 로직상 Trim은 수행하지 않음 (strutil.StripHTML에 의존)
		},
		{
			name:          "성공: HTML 엔티티 디코딩 (ex: &amp; -> &)",
			item:          item("MacBook Pro &amp; Air", "20000"),
			wantProduct:   true,
			expectedTitle: "MacBook Pro & Air",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tsk := &task{
				Base: provider.NewBase(provider.NewTaskParams{
					Request: &contract.TaskSubmitRequest{
						TaskID:     TaskID,
						CommandID:  WatchPriceAnyCommand,
						NotifierID: "test-notifier",
						RunBy:      contract.TaskRunByUser,
					},
					InstanceID: "test-instance",
					Fetcher:    mocks.NewMockHTTPFetcher(),
					NewSnapshot: func() interface{} {
						return &watchPriceSnapshot{}
					},
				}, true),
			}
			got := tsk.parseProduct(tt.item)

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

func TestProduct_IsPriceEligible_TableDriven(t *testing.T) {
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
			p := &product{LowPrice: tt.price}
			got := p.isPriceEligible(tt.priceLessThan)

			assert.Equal(t, tt.want, got)
		})
	}
}

// -----------------------------------------------------------------------------
// Unit Tests: Core Logic (Diff, Render, Summary)
// -----------------------------------------------------------------------------

func TestCalculateProductDiffs(t *testing.T) {
	t.Parallel()

	// Helper to make products
	makeProd := func(id, title string, price int) *product {
		return &product{ProductID: id, Title: title, LowPrice: price}
	}

	tests := []struct {
		name          string
		current       []*product
		prev          []*product
		expectedDiffs []productDiff
		checkSorting  func(*testing.T, []*product) // Side-effect(정렬) 검증
	}{
		{
			name: "신규 상품 감지 및 정렬 (가격 오름차순)",
			current: []*product{
				makeProd("1", "A", 2000),
				makeProd("2", "B", 1000),
			},
			prev: []*product{},
			expectedDiffs: []productDiff{
				{Type: productEventNew, Product: makeProd("2", "B", 1000), Prev: nil},
				{Type: productEventNew, Product: makeProd("1", "A", 2000), Prev: nil},
			},
			checkSorting: func(t *testing.T, curr []*product) {
				assert.Equal(t, "B", curr[0].Title) // 1000원이 먼저 오름
				assert.Equal(t, "A", curr[1].Title)
			},
		},
		{
			name: "가격 변동 감지",
			current: []*product{
				makeProd("1", "A", 1500),
			},
			prev: []*product{
				makeProd("1", "A", 2000),
			},
			expectedDiffs: []productDiff{
				{Type: productEventPriceChanged, Product: makeProd("1", "A", 1500), Prev: makeProd("1", "A", 2000)},
			},
			checkSorting: func(t *testing.T, curr []*product) {},
		},
		{
			name: "가격 동일 시 이름 오름차순 정렬",
			current: []*product{
				makeProd("2", "B_Title", 1000),
				makeProd("1", "A_Title", 1000),
			},
			prev: []*product{},
			expectedDiffs: []productDiff{
				{Type: productEventNew, Product: makeProd("1", "A_Title", 1000), Prev: nil},
				{Type: productEventNew, Product: makeProd("2", "B_Title", 1000), Prev: nil},
			},
			checkSorting: func(t *testing.T, curr []*product) {
				assert.Equal(t, "A_Title", curr[0].Title)
				assert.Equal(t, "B_Title", curr[1].Title)
			},
		},
		{
			name: "변경 없음 (Diffs Empty)",
			current: []*product{
				makeProd("1", "A", 1000),
			},
			prev: []*product{
				makeProd("1", "A", 1000),
			},
			expectedDiffs: nil,
			checkSorting:  func(t *testing.T, curr []*product) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			currSnap := &watchPriceSnapshot{Products: tt.current}
			var prevSnap *watchPriceSnapshot
			if tt.prev != nil {
				prevSnap = &watchPriceSnapshot{Products: tt.prev}
			}

			// Execute
			diffs := currSnap.Compare(prevSnap)

			// Verify Diffs
			assert.Len(t, diffs, len(tt.expectedDiffs))
			for i, expect := range tt.expectedDiffs {
				got := diffs[i]
				assert.Equal(t, expect.Type, got.Type)
				assert.Equal(t, expect.Product.ProductID, got.Product.ProductID)
				if expect.Prev != nil {
					assert.Equal(t, expect.Prev.LowPrice, got.Prev.LowPrice)
				}
			}

			// Verify Sorting (Side Effect)
			if tt.checkSorting != nil && len(currSnap.Products) > 0 {
				tt.checkSorting(t, currSnap.Products)
			}
		})
	}
}

func TestRenderProductDiffs(t *testing.T) {
	t.Parallel()

	p1 := &product{Title: "Item1", LowPrice: 10000, Link: "http://link1"}
	p2 := &product{Title: "Item2", LowPrice: 20000, Link: "http://link2"}
	p1Prev := &product{Title: "Item1", LowPrice: 15000} // Price dropped

	diffs := []productDiff{
		{Type: productEventNew, Product: p2},
		{Type: productEventPriceChanged, Product: p1, Prev: p1Prev},
	}

	t.Run("HTML Mode", func(t *testing.T) {
		msg := renderProductDiffs(diffs, true)
		assert.Contains(t, msg, mark.New.WithSpace())
		assert.Contains(t, msg, mark.Modified.WithSpace())
		assert.Contains(t, msg, "<a href=")
		assert.Contains(t, msg, "(이전: 15,000원)") // 15000 -> 10000 Drop
	})

	t.Run("Text Mode", func(t *testing.T) {
		msg := renderProductDiffs(diffs, false)
		assert.Contains(t, msg, mark.New.WithSpace())
		assert.Contains(t, msg, mark.Modified.WithSpace())
		assert.NotContains(t, msg, "<a href=")
		assert.Contains(t, msg, "http://link1") // Link explicitly shown
	})

	t.Run("Empty Diffs", func(t *testing.T) {
		msg := renderProductDiffs(nil, false)
		assert.Empty(t, msg)
	})
}

func TestRenderSearchConditionsSummary(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().
		WithQuery("MyQuery").
		WithIncludedKeywords("In Key").
		WithExcludedKeywords("Ex Key").
		WithPriceLessThan(50000).
		Build()

	summary := renderSearchConditionsSummary(&settings)

	assert.Contains(t, summary, "MyQuery")
	assert.Contains(t, summary, "In Key")
	assert.Contains(t, summary, "Ex Key")
	assert.Contains(t, summary, "50,000")
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

	mockFetcher := mocks.NewMockHTTPFetcher()

	// Page 1 Setup
	page1URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=1"
	page1Items := make([]*productSearchResponseItem, 100)
	for i := 0; i < 100; i++ {
		page1Items[i] = &productSearchResponseItem{Title: "P1", LowPrice: "100", ProductID: "P1"}
	}
	mockFetcher.SetResponse(page1URL, mustMarshal(productSearchResponse{
		Total: 150, Start: 1, Display: 100, Items: page1Items,
	}))

	// Page 2 Setup
	page2URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=101"
	page2Items := make([]*productSearchResponseItem, 50)
	for i := 0; i < 50; i++ {
		page2Items[i] = &productSearchResponseItem{Title: "P2", LowPrice: "100", ProductID: "P2"}
	}
	mockFetcher.SetResponse(page2URL, mustMarshal(productSearchResponse{
		Total: 150, Start: 101, Display: 50, Items: page2Items,
	}))

	tsk := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     "T",
				CommandID:  "C",
				NotifierID: "N",
				RunBy:      contract.TaskRunByUser,
			},
			InstanceID: "I",
			Fetcher:    mockFetcher,
			NewSnapshot: func() interface{} {
				return &watchPriceSnapshot{}
			},
		}, true),
		clientID:     "id",
		clientSecret: "secret",
	}
	// SetFetcher call removed as it's deprecated

	products, err := tsk.fetchProducts(context.Background(), &settings)

	require.NoError(t, err)
	assert.Len(t, products, 150, "총 150개의 상품이 수집되어야 합니다")
}

func TestTask_FetchProducts_Cancellation(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("cancel").WithPriceLessThan(999999).Build()
	mockFetcher := mocks.NewMockHTTPFetcher()

	// 1페이지 응답 설정 (Total이 많아서 다음 페이지가 필요하도록 설정)
	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=cancel&sort=sim&start=1"
	mockFetcher.SetResponse(url, mustMarshal(productSearchResponse{
		Total: 1000, Start: 1, Display: 1, Items: []*productSearchResponseItem{{Title: "A", LowPrice: "100", ProductID: "1"}},
	}))

	// Task 생성 및 취소 상태로 설정
	tsk := &task{clientID: "id", clientSecret: "secret"}
	tsk.Base = provider.NewBase(provider.NewTaskParams{
		Request: &contract.TaskSubmitRequest{
			TaskID:     "NS",
			CommandID:  "CMD",
			NotifierID: "NOTI",
			RunBy:      contract.TaskRunByScheduler,
		},
		InstanceID: "INS",
		Fetcher:    mockFetcher,
		NewSnapshot: func() interface{} {
			return &watchPriceSnapshot{}
		},
	}, true)
	// SetFetcher call removed as it's deprecated

	// 강제로 취소 상태 주입 (Context Cancel)
	tsk.Cancel()

	products, err := tsk.fetchProducts(context.Background(), &settings)

	// 취소되었으므로 nil 반환 체크
	require.NoError(t, err)
	assert.Nil(t, products, "작업 취소 시 nil을 반환해야 합니다")
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------

// BenchmarkTask_DiffAndNotify 대량의 상품 데이터에 대한 Diff 및 정렬 로직 성능을 측정합니다.
// 시나리오: 1000개의 기존 상품 vs 1000개의 신규 상품 (50% 변경)
func BenchmarkTask_DiffAndNotify(b *testing.B) {
	tsk := &task{}
	tsk.Base = provider.NewBase(provider.NewTaskParams{
		Request: &contract.TaskSubmitRequest{
			TaskID:     "NS",
			CommandID:  "CMD",
			NotifierID: "NOTI",
			RunBy:      contract.TaskRunByScheduler,
		},
		InstanceID: "INS",
		Fetcher:    mocks.NewMockHTTPFetcher(),
		NewSnapshot: func() interface{} {
			return &watchPriceSnapshot{}
		},
	}, true)
	settings := NewSettingsBuilder().WithQuery("bench").WithPriceLessThan(999999).Build()

	// Setup Large Data
	count := 1000
	prevItems := make([]*product, count)
	currItems := make([]*product, count)

	for i := 0; i < count; i++ {
		// Prev: Base Price
		prevItems[i] = NewProductBuilder().WithID(fmt.Sprintf("%d", i)).WithPrice(10000).WithTitle(fmt.Sprintf("Item %d", i)).Build()

		// Curr:
		// - 0~499: No Change
		// - 500~999: Price Change
		price := 10000
		if i >= count/2 {
			price = 9000 // Price Drop
		}
		currItems[i] = NewProductBuilder().WithID(fmt.Sprintf("%d", i)).WithPrice(price).WithTitle(fmt.Sprintf("Item %d", i)).Build()
	}

	prevSnapshot := &watchPriceSnapshot{Products: prevItems}
	currSnapshot := &watchPriceSnapshot{Products: currItems}

	prevProductsMap := make(map[string]*product)
	if prevSnapshot != nil {
		for _, p := range prevSnapshot.Products {
			prevProductsMap[p.key()] = p
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tsk.analyzeAndReport(&settings, currSnapshot, prevSnapshot, false)
	}
}

// BenchmarkTask_MapToProduct 핫 패스(Hot Path)인 상품 매핑 로직의 성능을 측정합니다.
func BenchmarkTask_MapToProduct(b *testing.B) {
	tsk := &task{}
	item := &productSearchResponseItem{
		Title:     "<b>Benchmark</b> Product",
		LowPrice:  "50,000",
		ProductID: "123456",
		Link:      "http://example.com",
		MallName:  "Benchmark Mall",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tsk.parseProduct(item)
	}
}

// makeMockProducts 테스트용 상품 목록을 대량으로 생성하는 헬퍼 함수
func makeMockProducts(count int) []*product {
	products := make([]*product, count)
	for i := 0; i < count; i++ {
		id := fmt.Sprintf("%d", i)
		products[i] = NewProductBuilder().
			WithID(id).
			WithTitle(fmt.Sprintf("Product %d", i)).
			WithPrice(1000 + i).
			Build()
	}
	return products
}

// TestTask_AnalyzeAndReport_Sorting executeWatchPrice 결과 리포트 생성 시,
// 신규 상품과 변경 상품이 '가격 오름차순'으로 올바르게 정렬되는지 검증합니다.
// (동일 가격일 경우 기존 순서를 유지하거나 이름순 등으로 정렬될 수 있음 - 현재 구현 기준 검증)
func TestTask_AnalyzeAndReport_Sorting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		currentProducts []*product
		prevProducts    []*product
		wantOrder       []string // 기대되는 ProductID 순서 (가격 오름차순)
	}{
		{
			name: "신규 상품만 - 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 30000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 10000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 20000},
			},
			prevProducts: []*product{},
			wantOrder:    []string{"B", "C", "A"}, // 10000 → 20000 → 30000
		},
		{
			name: "가격 변경 상품만 - 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 25000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 15000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 35000},
			},
			prevProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 30000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 20000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 40000},
			},
			wantOrder: []string{"B", "A", "C"}, // 15000 → 25000 → 35000
		},
		{
			name: "신규 + 가격 변경 혼합 - 전체 가격 오름차순 정렬",
			currentProducts: []*product{
				{ProductID: "NEW1", ProductType: "1", Title: "New 1", LowPrice: 50000},
				{ProductID: "CHANGED1", ProductType: "1", Title: "Changed 1", LowPrice: 12000},
				{ProductID: "NEW2", ProductType: "1", Title: "New 2", LowPrice: 8000},
				{ProductID: "CHANGED2", ProductType: "1", Title: "Changed 2", LowPrice: 18000},
				{ProductID: "UNCHANGED", ProductType: "1", Title: "Unchanged", LowPrice: 5000}, // 가격 변경 없음 (메시지 미생성)
			},
			prevProducts: []*product{
				{ProductID: "CHANGED1", ProductType: "1", Title: "Changed 1", LowPrice: 15000},
				{ProductID: "CHANGED2", ProductType: "1", Title: "Changed 2", LowPrice: 20000},
				{ProductID: "UNCHANGED", ProductType: "1", Title: "Unchanged", LowPrice: 5000},
			},
			wantOrder: []string{"NEW2", "CHANGED1", "CHANGED2", "NEW1"}, // 8000 → 12000 → 18000 → 50000 (UNCHANGED 제외)
		},
		{
			name: "동일 가격 상품 - 원본 순서 유지",
			currentProducts: []*product{
				{ProductID: "A", ProductType: "1", Title: "Product A", LowPrice: 10000},
				{ProductID: "B", ProductType: "1", Title: "Product B", LowPrice: 10000},
				{ProductID: "C", ProductType: "1", Title: "Product C", LowPrice: 10000},
			},
			prevProducts: []*product{},
			wantOrder:    []string{"A", "B", "C"}, // 동일 가격이므로 원본 순서 유지
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Setup
			tsk := &task{}
			currentSnapshot := &watchPriceSnapshot{Products: tt.currentProducts}
			prevSnapshot := &watchPriceSnapshot{Products: tt.prevProducts}
			settings := &watchPriceSettings{
				Query: "test",
			}
			settings.Filters.PriceLessThan = 100000

			// Execute
			message, _ := tsk.analyzeAndReport(settings, currentSnapshot, prevSnapshot, false)

			if len(tt.wantOrder) == 0 {
				assert.Empty(t, message, "변경 사항이 없으면 메시지가 비어야 합니다")
				return
			}

			require.NotEmpty(t, message, "변경 사항이 있으면 메시지가 생성되어야 합니다")

			// 메시지에서 ProductID 출현 순서 추출
			// (참고: 실제 구현체는 메시지 생성 시 상품명/가격 등을 출력하므로, Title이나 Price로 역추적해야 함.
			// 여기서는 Title에 ProductID 정보가 암묵적으로 포함되어 있다고 가정하거나, Title 자체를 확인)
			// -> Test data setup에서 Title을 'Product A' 등으로 했으므로 Title로 찾습니다.
			//    ProductID와 Title 매핑: A -> Product A, B -> Product B ...
			lines := strings.Split(message, "\n")
			var actualOrder []string
			for _, line := range lines {
				for _, p := range tt.currentProducts {
					if strings.Contains(line, p.Title) {
						// 중복 방지 (한 라인에 여러 번 매칭될 수도 있으나, 여기선 단순화)
						// 이미 리스트에 있으면 스킵
						alreadyAdded := false
						for _, id := range actualOrder {
							if id == p.ProductID {
								alreadyAdded = true
								break
							}
						}
						if !alreadyAdded {
							actualOrder = append(actualOrder, p.ProductID)
						}
						break // 한 라인에 하나의 상품만 매칭된다고 가정
					}
				}
			}

			// 기대 순서와 실제 순서 비교
			assert.Equal(t, tt.wantOrder, actualOrder, "상품이 가격 오름차순으로 정렬되어야 합니다")
		})
	}
}
