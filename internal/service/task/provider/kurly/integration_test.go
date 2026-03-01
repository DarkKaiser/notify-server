package kurly

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 통합 테스트용 픽스처 헬퍼
// =============================================================================

// integrationHTMLFixture 통합 테스트용 HTML 픽스처 빌더입니다.
type integrationHTMLFixture struct {
	productID     string
	productName   string
	discountRate  string // 빈 문자열이면 할인 없음
	salePrice     string // 할인가 (할인 있을 때)
	originalPrice string // 정가
}

// htmlUnavailable 판매 불가(product: null) HTML을 반환합니다.
func htmlUnavailable() string {
	return `<html><body><script id="__NEXT_DATA__">{"props":{"pageProps":{"product":null}}}</script></body></html>`
}

// htmlStructureInvalid props.pageProps 자체가 없는 구조 결함 HTML을 반환합니다.
func htmlStructureInvalid() string {
	return `<html><body><script id="__NEXT_DATA__">{"props":{}}</script></body></html>`
}

// htmlOnSale 할인 중인 상품 HTML을 반환합니다.
func htmlOnSale(productID, productName, discountRate, salePrice, originalPrice string) string {
	return fmt.Sprintf(`
<html>
<body>
	<script id="__NEXT_DATA__">{"props":{"pageProps":{"product":{"no":%s}}}}</script>
	<div id="product-atf">
		<section class="css-1ua1wyk">
			<div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"><h1>%s</h1></div></div></div>
			<h2 class="css-xrp7wx">
				<span class="css-8h3us8">%s%%</span>
				<div class="css-o2nlqt"><span>%s</span><span>원</span></div>
			</h2>
			<span class="css-1s96j0s"><span>%s원</span></span>
		</section>
	</div>
</body>
</html>`,
		productID, productName, discountRate, salePrice, originalPrice)
}

// htmlFullPrice 할인 없는 정가 판매 상품 HTML을 반환합니다.
func htmlFullPrice(productID, productName, price string) string {
	return fmt.Sprintf(`
<html>
<body>
	<script id="__NEXT_DATA__">{"props":{"pageProps":{"product":{"no":%s}}}}</script>
	<div id="product-atf">
		<section class="css-1ua1wyk">
			<div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"><h1>%s</h1></div></div></div>
			<h2 class="css-xrp7wx">
				<div class="css-o2nlqt"><span>%s</span><span>원</span></div>
			</h2>
		</section>
	</div>
</body>
</html>`,
		productID, productName, price)
}

// newIntegrationTask 통합 테스트용 task를 생성하는 헬퍼입니다.
// 반복되는 config 설정 및 task 초기화 코드를 공통화합니다.
func newIntegrationTask(t *testing.T, fetcher *mocks.MockHTTPFetcher, runBy contract.TaskRunBy) *task {
	t.Helper()

	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "placeholder.csv",
						},
					},
				},
			},
		},
	}

	handler, err := newTask(provider.NewTaskParams{
		InstanceID: "integration_instance",
		Request: &contract.TaskSubmitRequest{
			TaskID:     TaskID,
			CommandID:  WatchProductPriceCommand,
			NotifierID: "test-notifier",
			RunBy:      runBy,
		},
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     fetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)

	tsk, ok := handler.(*task)
	require.True(t, ok)
	return tsk
}

// newIntegrationLoader CSV 내용을 임시 파일에 쓴 뒤 loader를 반환하는 헬퍼입니다.
func newIntegrationLoader(t *testing.T, csvContent string) *csvWatchListLoader {
	t.Helper()
	csvFile := testutil.CreateTestFile(t, "integration_products.csv", csvContent)
	return &csvWatchListLoader{filePath: csvFile}
}

// =============================================================================
// 통합 테스트 — executeWatchProductPrice 전체 흐름 E2E 검증
// =============================================================================

// TestIntegration_NewProduct 신규 상품 등록 시 🆕 알림 메시지가 생성되는 전체 흐름을 검증합니다.
func TestIntegration_NewProduct(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlOnSale(productID, productName, "20", "8,000", "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	// 이전 스냅샷 없음 → 최초 실행
	prevSnapshot := &watchProductPriceSnapshot{Products: make([]*product, 0)}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchProductPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.Equal(t, productName, typed.Products[0].Name)
	assert.Equal(t, 10000, typed.Products[0].Price)
	assert.Equal(t, 8000, typed.Products[0].DiscountedPrice)
	assert.Equal(t, 20, typed.Products[0].DiscountRate)

	// 신규 상품 알림 검증
	assert.Contains(t, message, "상품 정보가 변경되었습니다")
	assert.Contains(t, message, productName)
	assert.Contains(t, message, "🆕")
}

// TestIntegration_NoChange_Scheduler 변경 없을 때 스케줄러 실행은 메시지와 스냅샷을 반환하지 않음을 검증합니다.
func TestIntegration_NoChange_Scheduler(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlOnSale(productID, productName, "20", "8,000", "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID: 12345, Name: productName,
				Price: 10000, DiscountedPrice: 8000, DiscountRate: 20,
				LowestPrice: 8000, LowestPriceTimeUTC: fixedTime,
			},
		},
	}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	assert.Empty(t, message, "변경 없음 + 스케줄러 실행 시에는 메시지가 없어야 합니다 (침묵)")
	assert.Nil(t, newSnapshot, "변경 없음 시에는 새 스냅샷이 nil이어야 합니다")
}

// TestIntegration_NoChange_UserRun 변경 없을 때 사용자 실행은 현재 상태 보고 메시지를 반환함을 검증합니다.
func TestIntegration_NoChange_UserRun(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlOnSale(productID, productName, "20", "8,000", "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByUser)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	fixedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID: 12345, Name: productName,
				Price: 10000, DiscountedPrice: 8000, DiscountRate: 20,
				LowestPrice: 8000, LowestPriceTimeUTC: fixedTime,
			},
		},
	}

	message, _, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	// 사용자 실행 시에는 변경 없어도 현재 상태 보고 메시지 반환
	assert.Contains(t, message, "변경된 상품 정보가 없습니다")
	assert.Contains(t, message, "현재 등록된 상품 정보는 아래와 같습니다")
}

// TestIntegration_PriceChanged_LowestPriceAchieved 가격 하락으로 역대 최저가 갱신 시 🔥 알림 메시지를 검증합니다.
func TestIntegration_PriceChanged_LowestPriceAchieved(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	// 이전 가격: 8,000원 → 현재 가격: 5,000원 (역대 최저가 갱신)
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlOnSale(productID, productName, "50", "5,000", "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID: 12345, Name: productName,
				Price: 10000, DiscountedPrice: 8000, DiscountRate: 20,
				LowestPrice: 8000,
			},
		},
	}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchProductPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.Equal(t, 5000, typed.Products[0].DiscountedPrice)

	assert.Contains(t, message, "상품 정보가 변경되었습니다")
	assert.Contains(t, message, "🔥", "최저가 갱신 시 🔥 마크가 포함되어야 합니다")
	assert.Contains(t, message, "5,000원")
}

// TestIntegration_PriceChanged_Regular 가격 변동(최저가 갱신 아님) 시 일반 변경 알림을 검증합니다.
func TestIntegration_PriceChanged_Regular(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	// 이전 가격: 8,000원(역대 최저 5,000원) → 현재 가격: 9,000원 (가격 상승)
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlOnSale(productID, productName, "10", "9,000", "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID: 12345, Name: productName,
				Price: 10000, DiscountedPrice: 8000, DiscountRate: 20,
				LowestPrice: 5000, // 역대 최저가는 5,000원
			},
		},
	}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)
	assert.Contains(t, message, "상품 정보가 변경되었습니다")
	// 최저가 갱신이 아니면 🔥가 없어야 함
	assert.NotContains(t, message, "🔥")
}

// TestIntegration_Reappeared 판매 중지 → 재입고 전이 시 알림 메시지를 검증합니다.
func TestIntegration_Reappeared(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	// 현재: 다시 판매 중
	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlFullPrice(productID, productName, "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	// 이전: 판매 중지 상태
	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{ID: 12345, Name: productName, IsUnavailable: true},
		},
	}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)
	assert.Contains(t, message, "상품 정보가 변경되었습니다")
	assert.Contains(t, message, "🆕", "재입고 알림은 신규와 동일한 🆕 이모지를 사용해야 합니다")
	assert.Contains(t, message, productName)
}

// TestIntegration_SoldOut 판매 중 → 판매 중지 전이 시 판매 불가 알림 메시지를 검증합니다.
func TestIntegration_SoldOut(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlUnavailable()),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	// 이전: 정상 판매 중
	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{ID: 12345, Name: productName, Price: 10000, DiscountedPrice: 8000, DiscountRate: 20},
		},
	}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	require.NotNil(t, newSnapshot)

	typed := newSnapshot.(*watchProductPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.True(t, typed.Products[0].IsUnavailable)

	assert.Contains(t, message, "알 수 없는 상품 목록")
	assert.Contains(t, message, productName)
}

// TestIntegration_SoldOut_NoRepeat 이미 판매 중지 상태인 상품은 재알림하지 않음을 검증합니다(스팸 방지).
func TestIntegration_SoldOut_NoRepeat(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlUnavailable()),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName))

	// 이전에도 이미 판매 중지 상태
	prevSnapshot := &watchProductPriceSnapshot{
		Products: []*product{
			{ID: 12345, Name: productName, IsUnavailable: true},
		},
	}

	message, _, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	assert.Empty(t, message, "이미 판매 중지 상태인 상품은 재알림하지 않아야 합니다")
}

// TestIntegration_DuplicateCSVRecord 중복 상품 레코드 감지 시 알림 메시지를 검증합니다.
func TestIntegration_DuplicateCSVRecord(t *testing.T) {
	t.Parallel()

	const (
		productID   = "12345"
		productName = "맛있는 사과"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(htmlFullPrice(productID, productName, "10,000")),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)

	// 동일 ID가 두 번 등장하는 CSV → 중복 감지
	csvContent := fmt.Sprintf(
		"No,Name,Status\n%s,%s,1\n%s,%s(중복),1\n",
		productID, productName, productID, productName,
	)
	loader := newIntegrationLoader(t, csvContent)

	prevSnapshot := &watchProductPriceSnapshot{Products: make([]*product, 0)}

	message, _, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.NoError(t, err)
	assert.Contains(t, message, "중복으로 등록된 상품 목록", "중복 상품 감지 시 중복 알림이 포함되어야 합니다")
}

// TestIntegration_NetworkError 네트워크 오류 시 FetchFailedCount가 증가하고 에러 없이 스냅샷을 반환함을 검증합니다.
func TestIntegration_NetworkError(t *testing.T) {
	t.Parallel()

	const productID = "12345"

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetError(
		fmt.Sprintf(productPageURLFormat, productID),
		fmt.Errorf("network timeout"),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,사과,1\n", productID))

	prevSnapshot := &watchProductPriceSnapshot{}

	message, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	// 네트워크 에러는 FetchFailedCount 카운팅을 위해 스냅샷을 저장해야 하므로 err는 nil
	require.NoError(t, err)
	assert.Empty(t, message)

	require.NotNil(t, newSnapshot)
	typed := newSnapshot.(*watchProductPriceSnapshot)
	require.Len(t, typed.Products, 1)
	assert.Equal(t, 1, typed.Products[0].FetchFailedCount, "네트워크 에러 시 FetchFailedCount가 1 증가해야 합니다")
}

// TestIntegration_JSONStructureInvalid JSON 구조 결함(props.pageProps 누락) 시 에러를 반환함을 검증합니다.
// 이는 단순 단종이 아니라 마켓컬리 페이지 구조 변경으로 판단해야 하는 핵심 케이스입니다.
func TestIntegration_JSONStructureInvalid(t *testing.T) {
	t.Parallel()

	const productID = "12345"

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		// props.pageProps 자체가 없는 구조 결함 HTML
		[]byte(htmlStructureInvalid()),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,사과,1\n", productID))

	prevSnapshot := &watchProductPriceSnapshot{}

	_, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	// 구조 결함은 단종이 아닌 시스템 에러이므로 에러가 반환되어야 합니다.
	// 이를 무시하면 모든 상품이 단종 처리되어 대량 오알림이 발생합니다.
	require.Error(t, err)
	assert.Nil(t, newSnapshot)
}

// TestIntegration_ParsingError HTML에 __NEXT_DATA__ 없음 시 에러를 반환함을 검증합니다.
func TestIntegration_ParsingError(t *testing.T) {
	t.Parallel()

	const productID = "12345"

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID),
		[]byte(`<html><body><h1>No Product Info</h1></body></html>`),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	loader := newIntegrationLoader(t, fmt.Sprintf("No,Name,Status\n%s,사과,1\n", productID))

	prevSnapshot := &watchProductPriceSnapshot{}

	_, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	require.Error(t, err)
	assert.Nil(t, newSnapshot)
}

// TestIntegration_MultipleProducts_PartialFailure 다중 상품 중 일부만 성공할 때 전체 동작을 검증합니다.
// 구조 결함 에러 발생 시 전체 수집이 abort되어야 합니다 (Fail-fast 원칙).
func TestIntegration_MultipleProducts_FailFast(t *testing.T) {
	t.Parallel()

	const (
		productID1 = "11111"
		productID2 = "22222"
	)

	mockFetcher := mocks.NewMockHTTPFetcher()
	// 첫 번째 상품: 정상
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID1),
		[]byte(htmlFullPrice(productID1, "상품1", "10,000")),
	)
	// 두 번째 상품: 구조 결함 → Fail-fast 중단
	mockFetcher.SetResponse(
		fmt.Sprintf(productPageURLFormat, productID2),
		[]byte(htmlStructureInvalid()),
	)

	tsk := newIntegrationTask(t, mockFetcher, contract.TaskRunByScheduler)
	csvContent := fmt.Sprintf(
		"No,Name,Status\n%s,상품1,1\n%s,상품2,1\n",
		productID1, productID2,
	)
	loader := newIntegrationLoader(t, csvContent)

	prevSnapshot := &watchProductPriceSnapshot{}

	_, newSnapshot, err := tsk.executeWatchProductPrice(context.Background(), loader, prevSnapshot, false)

	// 구조 결함은 Fail-fast → 전체 중단, 에러 반환
	require.Error(t, err)
	assert.Nil(t, newSnapshot)
}
