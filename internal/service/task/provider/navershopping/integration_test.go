package navershopping

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/darkkaiser/notify-server/internal/service/task/provider/testutil"
	"github.com/darkkaiser/notify-server/internal/service/task/scraper"
	"github.com/stretchr/testify/require"
)

func TestNaverShoppingTask_RunWatchPrice_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := mocks.NewMockHTTPFetcher()

	// 테스트용 JSON 응답 생성
	productTitle := "테스트 상품"
	productLink := "https://example.com/product/123"

	// "shopping_search_result.json"은 service/task/navershopping/testdata에 있어야 함
	// 하지만 list_dir 결과 "shopping_search_result.json"은 "naver" 폴더에 있었음.
	// We will assume I move it to "service/task/navershopping/testdata".
	jsonContent := testutil.LoadTestDataAsString(t, "shopping_search_result.json")

	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	// 2. Task 초기화
	tTask := &task{
		Base:         provider.NewBase(TaskID, WatchPriceAnyCommand, "test_instance", "test-notifier", contract.TaskRunByUnknown),
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}
	tTask.SetScraper(scraper.New(mockFetcher))
	// SetFetcher call removed as it's deprecated

	// 1. 초기 상태 설정
	commandSettings := &watchPriceSettings{
		Query: "맥북 에어",
	}
	commandSettings.Filters.PriceLessThan = 1500000
	commandSettingsMap := make(map[string]interface{})
	refStruct, _ := json.Marshal(commandSettings)
	_ = json.Unmarshal(refStruct, &commandSettingsMap)

	// 3. 테스트 데이터 준비
	commandConfig := &watchPriceSettings{
		Query: "테스트",
	}
	commandConfig.Filters.IncludedKeywords = ""
	commandConfig.Filters.ExcludedKeywords = ""
	commandConfig.Filters.PriceLessThan = 100000

	// 초기 결과 데이터 (비어있음)
	resultData := &watchPriceSnapshot{
		Products: make([]*product, 0),
	}

	// 4. 실행
	message, newResultData, err := tTask.executeWatchPrice(context.Background(), commandConfig, resultData, true)

	// 5. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*watchPriceSnapshot)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))

	product := typedResultData.Products[0]
	require.Equal(t, productTitle, product.Title)
	require.Equal(t, 10000, product.LowPrice)
	require.Equal(t, productLink, product.Link)

	// 메시지 검증 (신규 상품 알림)
	require.Contains(t, message, "조회 조건에 해당되는 상품 정보가 변경되었습니다")
	require.Contains(t, message, productTitle)
	require.Contains(t, message, "🆕")
}

func TestNaverShoppingTask_RunWatchPrice_NetworkError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	tTask := &task{
		Base:         provider.NewBase(TaskID, WatchPriceAnyCommand, "test_instance", "test-notifier", contract.TaskRunByUnknown),
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}
	tTask.SetScraper(scraper.New(mockFetcher))
	// SetFetcher call removed as it's deprecated

	// 3. 테스트 데이터 준비
	commandConfig := &watchPriceSettings{
		Query: "테스트",
	}
	resultData := &watchPriceSnapshot{}

	// 4. 실행
	_, _, err := tTask.executeWatchPrice(context.Background(), commandConfig, resultData, true)

	// 5. 검증
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestNaverShoppingTask_RunWatchPrice_InvalidJSON(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := mocks.NewMockHTTPFetcher()
	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetResponse(url, []byte(`{invalid json`))

	// 2. Task 초기화
	tTask := &task{
		Base:         provider.NewBase(TaskID, WatchPriceAnyCommand, "test_instance", "test-notifier", contract.TaskRunByUnknown),
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}
	tTask.SetScraper(scraper.New(mockFetcher))
	// SetFetcher call removed as it's deprecated

	// 3. 테스트 데이터 준비
	commandConfig := &watchPriceSettings{
		Query: "테스트",
	}
	resultData := &watchPriceSnapshot{}

	// 4. 실행
	_, _, err := tTask.executeWatchPrice(context.Background(), commandConfig, resultData, true)

	// 5. 검증
	require.Error(t, err)
	// unmarshalFromResponseJSONData 함수에서 발생하는 에러 메시지 확인
	// "응답 데이터(JSON) 파싱이 실패하였습니다" 같은 메시지가 포함되어야 함
	require.Contains(t, err.Error(), "JSON")
}

func TestNaverShoppingTask_RunWatchPrice_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := mocks.NewMockHTTPFetcher()

	productTitle := "테스트 상품"
	productLprice := "10000"
	productLink := "https://example.com/product/123"
	productImage := "https://example.com/image.jpg"
	productMallName := "테스트몰"

	jsonContent := fmt.Sprintf(`{
		"total": 1,
		"start": 1,
		"display": 1,
		"items": [{
			"title": "%s",
			"lprice": "%s",
			"link": "%s",
			"image": "%s",
			"mallName": "%s",
			"productId": "123",
			"productType": "1"
		}]
	}`, productTitle, productLprice, productLink, productImage, productMallName)

	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchPriceAnyCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByScheduler,
	}
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
							"query": "dummy",
							"filters": map[string]interface{}{
								"price_less_than": 10000,
							},
						},
					},
				},
			},
		},
	}

	handler, err := createTask("test_instance", req, appConfig, mockFetcher)
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandSettings := &watchPriceSettings{
		Query: "맥북 프로",
	}
	commandSettings.Filters.PriceLessThan = 2000000
	commandSettingsMap := make(map[string]interface{})
	refStruct, _ := json.Marshal(commandSettings)
	_ = json.Unmarshal(refStruct, &commandSettingsMap)

	commandConfig := &watchPriceSettings{
		Query: "테스트",
	}

	// 기존 결과 데이터 (이미 동일한 상품이 있음)
	resultData := &watchPriceSnapshot{
		Products: []*product{
			{
				Title:     productTitle,
				LowPrice:  10000,
				Link:      productLink,
				ProductID: "123",
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.executeWatchPrice(context.Background(), commandConfig, resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 스케줄러 실행 시 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestNaverShoppingTask_RunWatchPrice_PriceChange(t *testing.T) {
	// 가격 변경 시나리오
	mockFetcher := mocks.NewMockHTTPFetcher()

	productTitle := "테스트 상품"
	newPrice := "8000" // 가격 하락
	productLink := "https://example.com/product/123"
	productImage := "https://example.com/image.jpg"
	productMallName := "테스트몰"

	jsonContent := fmt.Sprintf(`{
		"total": 1,
		"start": 1,
		"display": 1,
		"items": [{
			"title": "%s",
			"lprice": "%s",
			"link": "%s",
			"image": "%s",
			"mallName": "%s",
			"productId": "123",
			"productType": "1"
		}]
	}`, productTitle, newPrice, productLink, productImage, productMallName)

	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchPriceAnyCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
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
							"query": "dummy",
							"filters": map[string]interface{}{
								"price_less_than": 10000,
							},
						},
					},
				},
			},
		},
	}

	handler, err := createTask("test_instance", req, appConfig, mockFetcher)
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandConfig := &watchPriceSettings{
		Query: "테스트",
	}
	commandConfig.Filters.PriceLessThan = 100000 // 가격 필터 설정

	// 기존 결과 데이터 (이전 가격)
	resultData := &watchPriceSnapshot{
		Products: []*product{
			{
				Title:     productTitle,
				LowPrice:  10000,
				Link:      productLink,
				ProductID: "123",
			},
		},
	}

	// 실행
	message, newResultData, err := tTask.executeWatchPrice(context.Background(), commandConfig, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message) // 가격 변경 시 메시지 있음
	// 가격 변경 시 메시지에 상품 정보 포함 확인
	require.Contains(t, message, productTitle)

	typedResultData, ok := newResultData.(*watchPriceSnapshot)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, 8000, typedResultData.Products[0].LowPrice)
}

func TestNaverShoppingTask_RunWatchPrice_WithFiltering(t *testing.T) {
	// 키워드 매칭 적용 시나리오
	mockFetcher := mocks.NewMockHTTPFetcher()

	jsonContent := `{
		"total": 3,
		"start": 1,
		"display": 3,
		"items": [
			{
				"title": "프리미엄 테스트 상품",
				"lprice": "50000",
				"link": "https://example.com/product/1",
				"image": "https://example.com/image1.jpg",
				"mallName": "테스트몰1",
				"productId": "1",
				"productType": "1"
			},
			{
				"title": "일반 테스트 상품",
				"lprice": "15000",
				"link": "https://example.com/product/2",
				"image": "https://example.com/image2.jpg",
				"mallName": "테스트몰2",
				"productId": "2",
				"productType": "1"
			},
			{
				"title": "저렴한 상품",
				"lprice": "5000",
				"link": "https://example.com/product/3",
				"image": "https://example.com/image3.jpg",
				"mallName": "테스트몰3",
				"productId": "3",
				"productType": "1"
			}
		]
	}`

	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%ED%85%8C%EC%8A%A4%ED%8A%B8&sort=sim&start=1"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	req := &contract.TaskSubmitRequest{
		TaskID:     TaskID,
		CommandID:  WatchPriceAnyCommand,
		NotifierID: "test-notifier",
		RunBy:      contract.TaskRunByUnknown,
	}
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
							"query": "dummy",
							"filters": map[string]interface{}{
								"price_less_than": 10000,
							},
						},
					},
				},
			},
		},
	}

	handler, err := createTask("test_instance", req, appConfig, mockFetcher)
	require.NoError(t, err)
	tTask, ok := handler.(*task)
	require.True(t, ok)

	commandSettings := &watchPriceSettings{
		Query: "테스트",
	}
	// 가격 필터: 20000원 미만만
	commandSettings.Filters.PriceLessThan = 20000
	// 포함 키워드: "테스트"
	commandSettings.Filters.IncludedKeywords = "테스트"

	resultData := &watchPriceSnapshot{
		Products: make([]*product, 0),
	}

	// 실행
	message, newResultData, err := tTask.executeWatchPrice(context.Background(), commandSettings, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)

	typedResultData, ok := newResultData.(*watchPriceSnapshot)
	require.True(t, ok)
	// 키워드 매칭 결과: "일반 테스트 상품"만 포함 (가격 15000원, "테스트" 포함)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, "일반 테스트 상품", typedResultData.Products[0].Title)
	require.Equal(t, 15000, typedResultData.Products[0].LowPrice)
}
