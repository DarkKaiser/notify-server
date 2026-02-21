package navershopping

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 통합 테스트 헬퍼 (Integration Test Helpers)
// =============================================================================

// integrationTask HTTP 목업 응답과 함께 사용할 통합테스트 전용 task를 생성합니다.
//
// newTask 팩토리를 통해 실제 초기화 경로(AppConfig → taskSettings 파싱 → clientID/Secret 바인딩)를
// 거치므로 단위테스트용 직접 구성 방식보다 실제 환경에 더 가깝습니다.
func integrationTask(t *testing.T, fetcher *mocks.MockHTTPFetcher, runBy contract.TaskRunBy) *task {
	t.Helper()

	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Data: map[string]interface{}{
					"client_id":     "test-client-id",
					"client_secret": "test-client-secret",
				},
				Commands: []config.CommandConfig{
					{
						ID: string(WatchPriceAnyCommand),
						Data: map[string]interface{}{
							"query": "placeholder",
							"filters": map[string]interface{}{
								"price_less_than": float64(9999999),
							},
						},
					},
				},
			},
		},
	}

	handler, err := newTask(provider.NewTaskParams{
		InstanceID: "integration-test",
		Request: &contract.TaskSubmitRequest{
			TaskID:     TaskID,
			CommandID:  WatchPriceAnyCommand,
			NotifierID: "test-notifier",
			RunBy:      runBy,
		},
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     fetcher,
		NewSnapshot: func() any { return &watchPriceSnapshot{} },
	})
	require.NoError(t, err)

	tsk, ok := handler.(*task)
	require.True(t, ok)
	return tsk
}

// makeItemJSON 단일 상품 JSON 문자열을 생성하는 헬퍼입니다.
func makeItemJSON(id, title, price, link, mallName string) string {
	return fmt.Sprintf(`{"productId":%q,"productType":"1","title":%q,"lprice":%q,"link":%q,"mallName":%q}`,
		id, title, price, link, mallName)
}

// makeSearchResponseJSON 상품 목록을 감싸는 검색 응답 JSON을 생성합니다.
func makeSearchResponseJSON(items ...string) string {
	var joined string
	for i, item := range items {
		if i > 0 {
			joined += ","
		}
		joined += item
	}
	return fmt.Sprintf(`{"total":%d,"start":1,"display":%d,"items":[%s]}`, len(items), len(items), joined)
}

// apiURL 검색어를 이용해 첫 페이지 URL을 반환합니다.
func apiURL(query string) string {
	base, err := url.Parse(productSearchEndpoint)
	if err != nil {
		panic(err)
	}
	return buildProductSearchURL(base, query, 1, defaultDisplayCount)
}

// =============================================================================
// 통합 시나리오 테스트
// =============================================================================

// TestIntegration_FirstRun_NewProducts 최초 실행(prev 스냅샷 없음) 시나리오를 검증합니다.
//
// 기대 동작:
//   - API 응답의 상품이 모두 신규(🆕)로 인식되어 알림 메시지가 생성됩니다.
//   - 가격 필터 미달 상품은 결과에서 제외됩니다.
//   - 반환된 스냅샷에 결과가 올바르게 저장됩니다.
func TestIntegration_FirstRun_NewProducts(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("1", "테스트 상품", "10000", "https://link/1", "TestMall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot, "신규 상품이 있으면 스냅샷을 반환해야 합니다")

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.Equal(t, "테스트 상품", typed.Products[0].Title)
	assert.Equal(t, 10000, typed.Products[0].LowPrice)

	assert.Contains(t, msg, "상품 정보가 변경되었습니다")
	assert.Contains(t, msg, "테스트 상품")
	assert.Contains(t, msg, mark.New.String(), "신규 상품은 🆕 마크가 포함되어야 합니다")
}

// TestIntegration_FirstRun_FromJSONFile testdata JSON 파일을 이용한 최초 실행 시나리오입니다.
//
// 실제 API 응답 형태의 파일을 로드하여 파싱이 올바르게 동작하는지 검증합니다.
func TestIntegration_FirstRun_FromJSONFile(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	jsonContent := testutil.LoadTestDataAsString(t, "shopping_search_result.json")

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(jsonContent))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.GreaterOrEqual(t, len(typed.Products), 1, "JSON 파일에 최소 1개 이상의 상품이 있어야 합니다")
	assert.Contains(t, msg, mark.New.String())
}

// TestIntegration_NoChange_Scheduler 스케줄러 실행 시 변경 없으면 메시지가 빈 문자열임을 검증합니다.
func TestIntegration_NoChange_Scheduler(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	const productID = "123"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON(productID, "테스트 상품", "10000", "https://link/1", "TestMall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	// 이전 스냅샷에 동일한 상품 존재
	prevSnapshot := &watchPriceSnapshot{
		Products: []*product{
			{ProductID: productID, Title: "테스트 상품", LowPrice: 10000, Link: "https://link/1", MallName: "TestMall", ProductType: "1"},
		},
	}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	assert.Empty(t, msg, "Scheduler: 변경 없음 → 빈 메시지")
	assert.Nil(t, newSnapshot, "변경 없음 → 스냅샷 갱신 불필요")
}

// TestIntegration_NoChange_User 사용자 실행 시 변경 없어도 현재 목록을 알림으로 전송합니다.
func TestIntegration_NoChange_User(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	const productID = "456"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON(productID, "테스트 상품", "20000", "https://link/2", "UserMall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByUser)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{
		Products: []*product{
			{ProductID: productID, Title: "테스트 상품", LowPrice: 20000, Link: "https://link/2", MallName: "UserMall", ProductType: "1"},
		},
	}

	msg, _, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	assert.Contains(t, msg, "변경된 정보가 없습니다", "User 실행: 현재 상품 목록을 표시해야 합니다")
	assert.Contains(t, msg, "테스트 상품")
}

// TestIntegration_PriceChanged 가격 변동 시 🔄 마크와 이전 가격이 메시지에 포함되는지 검증합니다.
func TestIntegration_PriceChanged(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	const productID = "789"
	mockFetcher := mocks.NewMockHTTPFetcher()
	// 가격이 10000 → 8000으로 하락
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON(productID, "테스트 상품", "8000", "https://link/3", "PriceMall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{
		Products: []*product{
			{ProductID: productID, Title: "테스트 상품", LowPrice: 10000, Link: "https://link/3", MallName: "PriceMall", ProductType: "1"},
		},
	}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.Equal(t, 8000, typed.Products[0].LowPrice, "스냅샷에 새 가격이 반영되어야 합니다")

	assert.Contains(t, msg, "8,000원", "현재 가격이 포함되어야 합니다")
	assert.Contains(t, msg, "(이전: 10,000원)", "이전 가격 비교가 포함되어야 합니다")
	assert.Contains(t, msg, mark.Modified.String(), "가격 변동은 🔄 마크가 포함되어야 합니다")
}

// TestIntegration_PriceFilter 가격 필터(price_less_than)가 올바르게 작동하는지 검증합니다.
//
// 필터 기준: 20000원 미만
//   - 5000원 상품  → 포함
//   - 15000원 상품 → 포함
//   - 50000원 상품 → 제외
func TestIntegration_PriceFilter(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("1", "저렴한 상품", "5000", "https://link/1", "Mall1"),
		makeItemJSON("2", "보통 상품", "15000", "https://link/2", "Mall2"),
		makeItemJSON("3", "비싼 상품", "50000", "https://link/3", "Mall3"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(20000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 2, "20000원 미만 상품만 2개 포함되어야 합니다")

	titles := []string{typed.Products[0].Title, typed.Products[1].Title}
	assert.Contains(t, titles, "저렴한 상품")
	assert.Contains(t, titles, "보통 상품")
	assert.NotContains(t, titles, "비싼 상품")
}

// TestIntegration_IncludedKeywordFilter 포함 키워드 필터가 올바르게 작동하는지 검증합니다.
//
// 포함 키워드: "프로" (AND 조건)
func TestIntegration_IncludedKeywordFilter(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("1", "맥북 프로 14인치", "2000000", "https://link/1", "Mall"),
		makeItemJSON("2", "맥북 에어 15인치", "1500000", "https://link/2", "Mall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().
		WithQuery(query).
		WithPriceLessThan(9999999).
		WithIncludedKeywords("프로").
		Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 1, "포함 키워드 '프로'에 매칭되는 상품만 수집되어야 합니다")
	assert.Equal(t, "맥북 프로 14인치", typed.Products[0].Title)
}

// TestIntegration_ExcludedKeywordFilter 제외 키워드 필터가 올바르게 작동하는지 검증합니다.
//
// 제외 키워드: "중고" (OR 조건)
func TestIntegration_ExcludedKeywordFilter(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("1", "새 상품 A", "10000", "https://link/1", "Mall"),
		makeItemJSON("2", "중고 상품 B", "5000", "https://link/2", "Mall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().
		WithQuery(query).
		WithPriceLessThan(100000).
		WithExcludedKeywords("중고").
		Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 1, "제외 키워드 '중고' 상품은 수집되지 않아야 합니다")
	assert.Equal(t, "새 상품 A", typed.Products[0].Title)
}

// TestIntegration_CombinedFilters 포함+제외 키워드와 가격 필터가 복합 적용되는지 검증합니다.
func TestIntegration_CombinedFilters(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("1", "프리미엄 테스트 상품", "50000", "https://link/1", "Mall1"), // 가격 초과 → 제외
		makeItemJSON("2", "일반 테스트 상품", "15000", "https://link/2", "Mall2"),   // 조건 충족 → 포함
		makeItemJSON("3", "저렴한 상품", "5000", "https://link/3", "Mall3"),       // 포함 키워드 없음 → 제외
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().
		WithQuery(query).
		WithPriceLessThan(20000).    // 20000원 미만
		WithIncludedKeywords("테스트"). // "테스트" 포함
		Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 1, "복합 필터 결과: '일반 테스트 상품'만 통과해야 합니다")
	assert.Equal(t, "일반 테스트 상품", typed.Products[0].Title)
	assert.Equal(t, 15000, typed.Products[0].LowPrice)
}

// TestIntegration_NetworkError 네트워크 오류 시 에러를 반환하는지 검증합니다.
func TestIntegration_NetworkError(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetError(apiURL(query), fmt.Errorf("connection refused"))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, _, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

// TestIntegration_InvalidJSON 유효하지 않은 JSON 응답 시 파싱 에러를 반환하는지 검증합니다.
func TestIntegration_InvalidJSON(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(apiURL(query), []byte(`{invalid json`))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	_, _, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "JSON")
}

// TestIntegration_EmptyResult_ZeroSpamProtection 결과가 0건이면 스팸 방지로 스냅샷이 갱신되지 않습니다.
func TestIntegration_EmptyResult_ZeroSpamProtection(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	// 0건 응답
	mockFetcher.SetResponse(apiURL(query), []byte(`{"total":0,"start":1,"display":0,"items":[]}`))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	// 이전에 상품이 있었음
	prevSnapshot := &watchPriceSnapshot{
		Products: []*product{
			{ProductID: "1", Title: "기존 상품", LowPrice: 10000, Link: "https://link/1", MallName: "Mall", ProductType: "1"},
		},
	}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	assert.Empty(t, msg, "0건 방어: 스팸 방지로 알림을 보내지 않아야 합니다")
	assert.Nil(t, newSnapshot, "0건 방어: 스냅샷을 갱신하지 않아야 합니다")
}

// TestIntegration_SortOrder 결과 상품이 가격 오름차순으로 정렬되어 메시지에 표시되는지 검증합니다.
func TestIntegration_SortOrder(t *testing.T) {
	t.Parallel()

	const query = "테스트"
	mockFetcher := mocks.NewMockHTTPFetcher()
	// 역순으로 응답 (30000 → 10000 → 20000)
	mockFetcher.SetResponse(apiURL(query), []byte(makeSearchResponseJSON(
		makeItemJSON("3", "비싼 상품", "30000", "https://link/3", "Mall"),
		makeItemJSON("1", "저렴한 상품", "10000", "https://link/1", "Mall"),
		makeItemJSON("2", "중간 상품", "20000", "https://link/2", "Mall"),
	)))

	tsk := integrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(100000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: []*product{}}

	msg, newSnapshot, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	// 스냅샷 내부 정렬 확인
	typed := newSnapshot.(*watchPriceSnapshot)
	require.Len(t, typed.Products, 3)
	assert.Equal(t, 10000, typed.Products[0].LowPrice, "첫 번째 상품은 최저가여야 합니다")
	assert.Equal(t, 20000, typed.Products[1].LowPrice)
	assert.Equal(t, 30000, typed.Products[2].LowPrice)

	// 메시지 내 순서: "저렴한 상품"이 "비싼 상품"보다 먼저 등장해야 함
	idxCheap := indexInString(msg, "저렴한 상품")
	idxExpensive := indexInString(msg, "비싼 상품")
	assert.Greater(t, idxCheap, -1)
	assert.Greater(t, idxExpensive, -1)
	assert.Less(t, idxCheap, idxExpensive, "가격 오름차순으로 정렬된 순서로 메시지가 구성되어야 합니다")
}

// indexInString msg 내에서 sub 문자열의 바이트 위치를 반환합니다. 없으면 -1.
func indexInString(msg, sub string) int {
	for i := 0; i <= len(msg)-len(sub); i++ {
		if msg[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
