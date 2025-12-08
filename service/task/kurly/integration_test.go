package kurly

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/require"
)

func TestKurlyTask_RunWatchProductPrice_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()

	// 테스트용 HTML 응답 생성
	productID := "12345"
	productName := "Test Product"
	originalPrice := "10,000"
	discountedPrice := "8,000"
	discountRate := "20"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":{"no":%s}}}}</script>
			<div id="product-atf">
				<section class="css-1ua1wyk">
					<div class="css-84rb3h">
						<div class="css-6zfm8o">
							<div class="css-o3fjh7">
								<h1>%s</h1>
							</div>
						</div>
					</div>
					<h2 class="css-xrp7wx">
						<span class="css-8h3us8">%s%%</span>
						<div class="css-o2nlqt">
							<span>%s</span>
							<span>원</span>
						</div>
					</h2>
					<span class="css-1s96j0s">
						<span>%s원</span>
					</span>
				</section>
			</div>
		</body>
		</html>
	`, productID, productName, discountRate, discountedPrice, originalPrice)

	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	// 2. Task 초기화
	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	// 3. 테스트 데이터 준비
	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}

	// CSV 파일 생성 (테스트용 임시 파일)
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	// 초기 결과 데이터 (비어있음)
	resultData := &kurlyWatchProductPriceResultData{
		Products: make([]*kurlyProduct, 0),
	}

	// 4. 실행
	message, newResultData, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 5. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*kurlyWatchProductPriceResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))

	product := typedResultData.Products[0]
	require.Equal(t, productName, product.Name)
	require.Equal(t, 10000, product.Price)
	require.Equal(t, 8000, product.DiscountedPrice)
	require.Equal(t, 20, product.DiscountRate)

	// 메시지 검증 (신규 상품 알림)
	require.Contains(t, message, "상품 정보가 변경되었습니다")
	require.Contains(t, message, productName)
	require.Contains(t, message, "🆕")
}

func TestKurlyTask_RunWatchProductPrice_NetworkError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	productID := "12345"
	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	// 3. 테스트 데이터 준비
	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,Test Product,1\n", productID)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	resultData := &kurlyWatchProductPriceResultData{}

	// 4. 실행
	_, _, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 5. 검증
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestKurlyTask_RunWatchProductPrice_ParsingError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := task.NewMockHTTPFetcher()
	productID := "12345"
	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	// 필수 요소가 누락된 HTML
	mockFetcher.SetResponse(url, []byte(`<html><body><h1>No Product Info</h1></body></html>`))

	// 2. Task 초기화
	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	// 3. 테스트 데이터 준비
	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,Test Product,1\n", productID)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	resultData := &kurlyWatchProductPriceResultData{}

	// 4. 실행
	_, _, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 5. 검증
	require.Error(t, err)
	// Kurly는 HTML에서 JSON 데이터를 추출하므로 다른 에러 메시지가 발생
	// "JSON 데이터 추출이 실패하였습니다" 메시지 예상
	require.Contains(t, err.Error(), "JSON 데이터 추출이 실패하였습니다")
}

func TestKurlyTask_RunWatchProductPrice_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := task.NewMockHTTPFetcher()
	productID := "12345"
	productName := "Test Product"
	price := "10,000"
	discountedPrice := "8,000"
	discountRate := "20"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":{"no":%s}}}}</script>
			<div id="product-atf">
				<section class="css-1ua1wyk">
					<div class="css-84rb3h">
						<div class="css-6zfm8o">
							<div class="css-o3fjh7">
								<h1>%s</h1>
							</div>
						</div>
					</div>
					<h2 class="css-xrp7wx">
						<span class="css-8h3us8">%s%%</span>
						<div class="css-o2nlqt">
							<span>%s</span>
							<span>원</span>
						</div>
					</h2>
					<span class="css-1s96j0s">
						<span>%s원</span>
					</span>
				</section>
			</div>
		</body>
		</html>
	`, productID, productName, discountRate, discountedPrice, price)

	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
			RunBy:      task.TaskRunByScheduler, // 스케줄러 실행으로 설정
		},
	}

	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	// 기존 결과 데이터 (동일한 데이터)
	resultData := &kurlyWatchProductPriceResultData{
		Products: []*kurlyProduct{
			{
				No:              12345,
				Name:            productName,
				Price:           10000,
				DiscountedPrice: 8000,
				DiscountRate:    20,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestKurlyTask_RunWatchProductPrice_PriceChange(t *testing.T) {
	// 가격 변경 시나리오
	mockFetcher := task.NewMockHTTPFetcher()
	productID := "12345"
	productName := "Test Product"
	price := "10,000"
	newDiscountedPrice := "5,000" // 가격 하락
	newDiscountRate := "50"

	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":{"no":%s}}}}</script>
			<div id="product-atf">
				<section class="css-1ua1wyk">
					<div class="css-84rb3h">
						<div class="css-6zfm8o">
							<div class="css-o3fjh7">
								<h1>%s</h1>
							</div>
						</div>
					</div>
					<h2 class="css-xrp7wx">
						<span class="css-8h3us8">%s%%</span>
						<div class="css-o2nlqt">
							<span>%s</span>
							<span>원</span>
						</div>
					</h2>
					<span class="css-1s96j0s">
						<span>%s원</span>
					</span>
				</section>
			</div>
		</body>
		</html>
	`, productID, productName, newDiscountRate, newDiscountedPrice, price)

	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	// 기존 결과 데이터 (이전 가격)
	resultData := &kurlyWatchProductPriceResultData{
		Products: []*kurlyProduct{
			{
				No:              12345,
				Name:            productName,
				Price:           10000,
				DiscountedPrice: 8000,
				DiscountRate:    20,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "상품 정보가 변경되었습니다")
	require.Contains(t, message, "🔁")      // 변경 마크
	require.Contains(t, message, "5,000원") // 새로운 가격

	typedResultData, ok := newResultData.(*kurlyWatchProductPriceResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, 5000, typedResultData.Products[0].DiscountedPrice)
}

func TestKurlyTask_RunWatchProductPrice_SoldOut(t *testing.T) {
	// 품절(알 수 없는 상품) 시나리오
	mockFetcher := task.NewMockHTTPFetcher()
	productID := "12345"
	productName := "Test Product"

	// product: null 로 설정하여 알 수 없는 상품 시뮬레이션
	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":null}}}</script>
		</body>
		</html>
	`)

	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	tTask := &kurlyTask{
		Task: task.Task{
			ID:         TidKurly,
			CommandID:  TcidKurlyWatchProductPrice,
			NotifierID: "test-notifier",
			Fetcher:    mockFetcher,
		},
	}

	commandData := &kurlyWatchProductPriceTaskCommandData{
		WatchProductsFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := task.CreateTestCSVFile(t, "test_products.csv", csvContent)
	commandData.WatchProductsFile = csvFile

	// 기존 결과 데이터 (정상 판매 중)
	resultData := &kurlyWatchProductPriceResultData{
		Products: []*kurlyProduct{
			{
				No:               12345,
				Name:             productName,
				Price:            10000,
				DiscountedPrice:  8000,
				DiscountRate:     20,
				IsUnknownProduct: false,
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.runWatchProductPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "알 수 없는 상품 목록")
	require.Contains(t, message, productName)

	typedResultData, ok := newResultData.(*kurlyWatchProductPriceResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.True(t, typedResultData.Products[0].IsUnknownProduct)
}
