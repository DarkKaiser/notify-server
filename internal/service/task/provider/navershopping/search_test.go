package navershopping

import (
	"context"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 테스트 헬퍼
// =============================================================================

// newSearchTestTask search.go 함수 테스트에 사용할 최소 task를 생성합니다.
func newSearchTestTask(fetcher ...*mocks.MockHTTPFetcher) *task {
	var f *mocks.MockHTTPFetcher
	if len(fetcher) > 0 {
		f = fetcher[0]
	} else {
		f = mocks.NewMockHTTPFetcher()
	}
	return &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     TaskID,
				CommandID:  WatchPriceAnyCommand,
				NotifierID: "test-notifier",
				RunBy:      contract.TaskRunByUser,
			},
			InstanceID:  "test-instance",
			Fetcher:     f,
			NewSnapshot: func() interface{} { return &watchPriceSnapshot{} },
		}, true),
	}
}

// =============================================================================
// parseProduct 검증
// =============================================================================

// TestParseProduct_Success 정상적인 상품 데이터를 변환할 때 모든 필드가 올바르게 매핑되는지 검증합니다.
func TestParseProduct_Success(t *testing.T) {
	t.Parallel()

	tsk := newSearchTestTask()
	item := &productSearchResponseItem{
		ProductID:   "123456",
		ProductType: "1",
		Title:       "Samsung Galaxy S24",
		Link:        "https://shopping.naver.com/products/123456",
		LowPrice:    "1200000",
		MallName:    "Samsung Official Store",
	}

	got := tsk.parseProduct(item)

	require.NotNil(t, got)
	assert.Equal(t, "123456", got.ProductID)
	assert.Equal(t, "1", got.ProductType)
	assert.Equal(t, "Samsung Galaxy S24", got.Title)
	assert.Equal(t, "https://shopping.naver.com/products/123456", got.Link)
	assert.Equal(t, 1200000, got.LowPrice)
	assert.Equal(t, "Samsung Official Store", got.MallName)
}

// TestParseProduct_HTMLStripping 상품명에 포함된 HTML 태그를 제거하는지 검증합니다.
//
// 네이버 API는 검색어 매칭 부분에 <b> 태그를 삽입하므로 반드시 제거가 필요합니다.
func TestParseProduct_HTMLStripping(t *testing.T) {
	t.Parallel()

	tsk := newSearchTestTask()

	tests := []struct {
		name      string
		rawTitle  string
		wantTitle string
	}{
		{
			name:      "<b> 태그 제거",
			rawTitle:  "<b>Apple</b> iPad <b>Pro</b>",
			wantTitle: "Apple iPad Pro",
		},
		{
			name:      "HTML 엔티티 디코딩 (&amp; → &)",
			rawTitle:  "MacBook Pro &amp; Air",
			wantTitle: "MacBook Pro & Air",
		},
		{
			name:      "태그 없는 일반 문자열은 그대로 반환",
			rawTitle:  "iPhone 15 Pro Max",
			wantTitle: "iPhone 15 Pro Max",
		},
		{
			name:      "유니코드 및 특수문자는 그대로 유지",
			rawTitle:  "특가! ★Galaxy★ S25 Ultra",
			wantTitle: "특가! ★Galaxy★ S25 Ultra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := &productSearchResponseItem{
				ProductID: "1", Title: tt.rawTitle, LowPrice: "100000", Link: "http://link", MallName: "mall",
			}
			got := tsk.parseProduct(item)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantTitle, got.Title)
		})
	}
}

// TestParseProduct_PriceCommaRemoval 쉼표가 포함된 가격 문자열을 올바르게 파싱하는지 검증합니다.
func TestParseProduct_PriceCommaRemoval(t *testing.T) {
	t.Parallel()

	tsk := newSearchTestTask()

	tests := []struct {
		name      string
		rawPrice  string
		wantPrice int
	}{
		{name: "쉼표 없는 가격", rawPrice: "50000", wantPrice: 50000},
		{name: "쉼표 포함 4자리", rawPrice: "1,000", wantPrice: 1000},
		{name: "쉼표 포함 7자리", rawPrice: "1,000,000", wantPrice: 1000000},
		{name: "0원", rawPrice: "0", wantPrice: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := &productSearchResponseItem{
				ProductID: "1", Title: "Test", LowPrice: tt.rawPrice, Link: "http://link", MallName: "mall",
			}
			got := tsk.parseProduct(item)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantPrice, got.LowPrice)
		})
	}
}

// TestParseProduct_InvalidPrice 가격 파싱에 실패하면 nil을 반환하는지 검증합니다.
//
// 유효하지 않은 가격 데이터는 경고 로그를 남기고 해당 상품을 건너뛰어야 합니다.
func TestParseProduct_InvalidPrice(t *testing.T) {
	t.Parallel()

	tsk := newSearchTestTask()

	tests := []struct {
		name     string
		rawPrice string
	}{
		{name: "숫자가 아닌 문자열", rawPrice: "Call for Price"},
		{name: "빈 문자열", rawPrice: ""},
		{name: "특수문자", rawPrice: "N/A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			item := &productSearchResponseItem{
				ProductID: "1", Title: "Test", LowPrice: tt.rawPrice, Link: "http://link", MallName: "mall",
			}
			got := tsk.parseProduct(item)
			assert.Nil(t, got, "유효하지 않은 가격은 nil을 반환해야 합니다")
		})
	}
}

// =============================================================================
// buildProductSearchURL 검증
// =============================================================================

// TestBuildProductSearchURL 검색 URL이 올바른 쿼리 파라미터를 가지는지 검증합니다.
func TestBuildProductSearchURL(t *testing.T) {
	t.Parallel()

	base, err := url.Parse("https://openapi.naver.com/v1/search/shop.json")
	require.NoError(t, err)

	t.Run("기본 파라미터 적용 검증", func(t *testing.T) {
		t.Parallel()

		got := buildProductSearchURL(base, "갤럭시 S24", 1, 100)

		parsed, err := url.Parse(got)
		require.NoError(t, err)

		q := parsed.Query()
		assert.Equal(t, "갤럭시 S24", q.Get("query"))
		assert.Equal(t, "100", q.Get("display"))
		assert.Equal(t, "1", q.Get("start"))
		assert.Equal(t, defaultSortOption, q.Get("sort"))
	})

	t.Run("페이지네이션 start 파라미터 변경 검증", func(t *testing.T) {
		t.Parallel()

		got := buildProductSearchURL(base, "MacBook", 101, 100)

		parsed, err := url.Parse(got)
		require.NoError(t, err)

		assert.Equal(t, "101", parsed.Query().Get("start"))
	})

	t.Run("원본 baseURL이 변경되지 않음 (불변성 보장)", func(t *testing.T) {
		t.Parallel()

		// baseURL의 RawQuery가 변경되지 않아야 함
		originalQuery := base.RawQuery
		_ = buildProductSearchURL(base, "test", 1, 100)
		assert.Equal(t, originalQuery, base.RawQuery, "buildProductSearchURL 호출 후 baseURL이 변경되지 않아야 합니다")
	})

	t.Run("특수문자 쿼리가 URL 인코딩됨", func(t *testing.T) {
		t.Parallel()

		got := buildProductSearchURL(base, "iPhone & case", 1, 100)

		parsed, err := url.Parse(got)
		require.NoError(t, err)

		// url.Parse 후 Query()로 꺼내면 자동으로 디코딩되어 원본 값이 나와야 함
		assert.Equal(t, "iPhone & case", parsed.Query().Get("query"))
	})
}

// =============================================================================
// fetchPageProducts 검증
// =============================================================================

// TestFetchPageProducts_Success 정상 응답 시 상품 데이터를 올바르게 파싱하는지 검증합니다.
func TestFetchPageProducts_Success(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	apiURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

	resp := productSearchResponse{
		Total:   2,
		Start:   1,
		Display: 2,
		Items: []*productSearchResponseItem{
			{ProductID: "1", Title: "Product A", LowPrice: "10000", Link: "http://link/1", MallName: "ShopA"},
			{ProductID: "2", Title: "Product B", LowPrice: "20000", Link: "http://link/2", MallName: "ShopB"},
		},
	}
	mockFetcher.SetResponse(apiURL, mustMarshal(resp))

	tsk := newSearchTestTask(mockFetcher)
	tsk.clientID = "test-id"
	tsk.clientSecret = "test-secret"

	got, err := tsk.fetchPageProducts(context.Background(), apiURL)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2, got.Total)
	assert.Len(t, got.Items, 2)
	assert.Equal(t, "Product A", got.Items[0].Title)
}

// TestFetchPageProducts_NetworkError 네트워크 오류 시 에러를 반환하는지 검증합니다.
func TestFetchPageProducts_NetworkError(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	apiURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

	networkErr := assert.AnError
	mockFetcher.SetError(apiURL, networkErr)

	tsk := newSearchTestTask(mockFetcher)
	tsk.clientID = "id"
	tsk.clientSecret = "secret"

	got, err := tsk.fetchPageProducts(context.Background(), apiURL)

	require.Error(t, err)
	assert.Nil(t, got)
}

// TestFetchPageProducts_ContextCanceled context가 취소된 상태에서 호출하면 에러를 반환합니다.
func TestFetchPageProducts_ContextCanceled(t *testing.T) {
	t.Parallel()

	mockFetcher := mocks.NewMockHTTPFetcher()
	apiURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"
	mockFetcher.SetError(apiURL, context.Canceled)

	tsk := newSearchTestTask(mockFetcher)
	tsk.clientID = "id"
	tsk.clientSecret = "secret"

	got, err := tsk.fetchPageProducts(context.Background(), apiURL)

	require.Error(t, err)
	assert.Nil(t, got)
}
