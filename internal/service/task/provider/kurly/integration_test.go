package kurly

import (
	"context"
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"
	"github.com/stretchr/testify/require"
)

func TestKurlyTask_RunWatchProductPrice_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := mocks.NewMockHTTPFetcher()

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

	// url := fmt.Sprintf("%sgoods/%s", baseURL, productID) -> fmt.Sprintf(productPageURLFormat, productID)
	url := fmt.Sprintf(productPageURLFormat, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	// 2. Task 초기화
	// 2. Task 초기화
	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}

	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	// 3. 테스트 데이터 준비
	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}

	// CSV 파일 생성 (테스트용 임시 파일)
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	// 초기 결과 데이터 (비어있음)
	resultData := &watchProductPriceSnapshot{
		Products: make([]*product, 0),
	}

	// CSV Provider 생성
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// 4. 실행
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, true)

	// 5. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*watchProductPriceSnapshot)
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
	mockFetcher := mocks.NewMockHTTPFetcher()
	productID := "12345"
	url := fmt.Sprintf(productPageURLFormat, productID)
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	// 2. Task 초기화
	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}

	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	// 3. 테스트 데이터 준비
	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,Test Product,1\n", productID)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	resultData := &watchProductPriceSnapshot{}

	// CSV Loader 생성
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// 4. 실행
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, true)

	// 5. 검증: 부분 실패는 에러로 간주되지 않고 해당 상품이 무시됩니다.
	// 따라서 전체 실행은 성공(err == nil)하고, 결과에는 수집된 상품이 0개이므로
	// 이전 상태와의 차이가 없어서 변경 알림(message)과 신규 저장 스냅샷(newResultData)이 생성되지 않습니다.
	require.NoError(t, err)
	require.Empty(t, message)
	require.Nil(t, newResultData)
}

func TestKurlyTask_RunWatchProductPrice_ParsingError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := mocks.NewMockHTTPFetcher()
	productID := "12345"
	url := fmt.Sprintf(productPageURLFormat, productID)
	// 필수 요소가 누락된 HTML
	mockFetcher.SetResponse(url, []byte(`<html><body><h1>No Product Info</h1></body></html>`))

	// 2. Task 초기화
	// 2. Task 초기화
	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}

	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	// 3. 테스트 데이터 준비
	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,Test Product,1\n", productID)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	resultData := &watchProductPriceSnapshot{}

	// [Dependency Injection]
	// 테스트 환경에서도 실제 CSVLoader를 사용하여 E2E	// [Dependency Injection]
	// 테스트 환경에서도 실제 CSVLoader를 사용하여 E2E 시나리오를 검증합니다.
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// -------------------------------------------------------------------------
	// 5. Execute Task Logic
	// -------------------------------------------------------------------------
	// 상품 코드가 숫자가 아니므로, 파싱 전 단계에서 에러가 반환되어야 합니다.
	// 변경된 스펙: 부분 실패는 격리되며 에러를 반환하지 않습니다.
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, false)

	// 5. 검증
	require.NoError(t, err)
	require.Empty(t, message)
	require.Nil(t, newResultData)
}

func TestKurlyTask_RunWatchProductPrice_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := mocks.NewMockHTTPFetcher()
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

	url := fmt.Sprintf(productPageURLFormat, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByScheduler,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}
	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	// 기존 결과 데이터 (동일한 데이터)
	resultData := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID:              12345,
				Name:            productName,
				Price:           10000,
				DiscountedPrice: 8000,
				DiscountRate:    20,
			},
		},
	}

	// CSV Loader 생성
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// 실행
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestKurlyTask_RunWatchProductPrice_PriceChange(t *testing.T) {
	// 가격 변경 시나리오
	mockFetcher := mocks.NewMockHTTPFetcher()
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

	url := fmt.Sprintf(productPageURLFormat, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}
	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	// 기존 결과 데이터 (이전 가격)
	resultData := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID:              12345,
				Name:            productName,
				Price:           10000,
				DiscountedPrice: 8000,
				DiscountRate:    20,
			},
		},
	}

	// CSV Loader 생성
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// 실행
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "상품 정보가 변경되었습니다")
	require.Contains(t, message, "🔥")      // 최저가 갱신 마크
	require.Contains(t, message, "5,000원") // 새로운 가격

	typedResultData, ok := newResultData.(*watchProductPriceSnapshot)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, 5000, typedResultData.Products[0].DiscountedPrice)
}

func TestKurlyTask_RunWatchProductPrice_SoldOut(t *testing.T) {
	// 품절(알 수 없는 상품) 시나리오
	mockFetcher := mocks.NewMockHTTPFetcher()
	productID := "12345"
	productName := "Test Product"

	// product: null 로 설정하여 알 수 없는 상품 시뮬레이션
	htmlContent := `
		<html>
		<body>
			<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":null}}}</script>
		</body>
		</html>
	`

	url := fmt.Sprintf(productPageURLFormat, productID)
	mockFetcher.SetResponse(url, []byte(htmlContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchProductPriceCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
	appConfig := &config.AppConfig{
		Tasks: []config.TaskConfig{
			{
				ID: string(TaskID),
				Commands: []config.CommandConfig{
					{
						ID: string(WatchProductPriceCommand),
						Data: map[string]interface{}{
							"watch_list_file": "test_products.csv",
						},
					},
				},
			},
		},
	}
	handler, err := newTask(provider.NewTaskParams{
		InstanceID:  "test_instance",
		Request:     req,
		AppConfig:   appConfig,
		Storage:     nil,
		Fetcher:     mockFetcher,
		NewSnapshot: func() any { return &watchProductPriceSnapshot{} },
	})
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandConfig := &watchProductPriceSettings{
		WatchListFile: "test_products.csv",
	}
	csvContent := fmt.Sprintf("No,Name,Status\n%s,%s,1\n", productID, productName)
	csvFile := testutil.CreateTestFile(t, "test_products.csv", csvContent)
	commandConfig.WatchListFile = csvFile

	// 기존 결과 데이터 (정상 판매 중)
	resultData := &watchProductPriceSnapshot{
		Products: []*product{
			{
				ID:              12345,
				Name:            productName,
				Price:           10000,
				DiscountedPrice: 8000,
				DiscountRate:    20,
				IsUnavailable:   false,
			},
		},
	}

	// CSV Loader 생성
	loader := &csvWatchListLoader{filePath: commandConfig.WatchListFile}

	// 실행
	message, newResultData, err := tTask.executeWatchProductPrice(context.Background(), loader, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)
	require.Contains(t, message, "알 수 없는 상품 목록")
	require.Contains(t, message, productName)

	typedResultData, ok := newResultData.(*watchProductPriceSnapshot)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.True(t, typedResultData.Products[0].IsUnavailable)
}
