package task

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNaverShoppingTask_RunWatchPrice_Integration(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := NewMockHTTPFetcher()

	// 테스트용 JSON 응답 생성
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
			"mallName": "%s"
		}]
	}`, productTitle, productLprice, productLink, productImage, productMallName)

	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	// 2. Task 초기화
	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	// 3. 테스트 데이터 준비
	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}
	commandData.Filters.IncludedKeywords = ""
	commandData.Filters.ExcludedKeywords = ""
	commandData.Filters.PriceLessThan = 100000

	// 초기 결과 데이터 (비어있음)
	resultData := &naverShoppingWatchPriceResultData{
		Products: make([]*naverShoppingProduct, 0),
	}

	// 4. 실행
	message, newResultData, err := task.runWatchPrice(commandData, resultData, true)

	// 5. 검증
	require.NoError(t, err)
	require.NotNil(t, newResultData)

	// 결과 데이터 타입 변환
	typedResultData, ok := newResultData.(*naverShoppingWatchPriceResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))

	product := typedResultData.Products[0]
	require.Equal(t, productTitle, product.Title)
	require.Equal(t, 10000, product.LowPrice)
	require.Equal(t, productLink, product.Link)

	// 메시지 검증 (신규 상품 알림)
	require.Contains(t, message, "조회 조건에 해당되는 상품의 정보가 변경되었습니다")
	require.Contains(t, message, productTitle)
	require.Contains(t, message, "🆕")
}

func TestNaverShoppingTask_RunWatchPrice_NetworkError(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := NewMockHTTPFetcher()
	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetError(url, fmt.Errorf("network error"))

	// 2. Task 초기화
	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	// 3. 테스트 데이터 준비
	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}
	resultData := &naverShoppingWatchPriceResultData{}

	// 4. 실행
	_, _, err := task.runWatchPrice(commandData, resultData, true)

	// 5. 검증
	require.Error(t, err)
	require.Contains(t, err.Error(), "network error")
}

func TestNaverShoppingTask_RunWatchPrice_InvalidJSON(t *testing.T) {
	// 1. Mock 설정
	mockFetcher := NewMockHTTPFetcher()
	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(`{invalid json`))

	// 2. Task 초기화
	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	// 3. 테스트 데이터 준비
	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}
	resultData := &naverShoppingWatchPriceResultData{}

	// 4. 실행
	_, _, err := task.runWatchPrice(commandData, resultData, true)

	// 5. 검증
	require.Error(t, err)
	// unmarshalFromResponseJSONData 함수에서 발생하는 에러 메시지 확인
	// "응답 데이터(JSON) 파싱이 실패하였습니다" 같은 메시지가 포함되어야 함
	require.Contains(t, err.Error(), "JSON")
}

func TestNaverShoppingTask_RunWatchPrice_NoChange(t *testing.T) {
	// 데이터 변화 없음 시나리오 (스케줄러 실행)
	mockFetcher := NewMockHTTPFetcher()

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
			"mallName": "%s"
		}]
	}`, productTitle, productLprice, productLink, productImage, productMallName)

	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
			runBy:      TaskRunByScheduler, // 스케줄러 실행으로 설정
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}

	// 기존 결과 데이터 (이미 동일한 상품이 있음)
	resultData := &naverShoppingWatchPriceResultData{
		Products: []*naverShoppingProduct{
			{
				Title:    productTitle,
				LowPrice: 10000,
				Link:     productLink,
			},
		},
	}

	// 실행
	message, newResultData, err := task.runWatchPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.Empty(t, message)     // 스케줄러 실행 시 변화 없으면 메시지 없음
	require.Nil(t, newResultData) // 변화 없으면 nil 반환
}

func TestNaverShoppingTask_RunWatchPrice_PriceChange(t *testing.T) {
	// 가격 변경 시나리오
	mockFetcher := NewMockHTTPFetcher()

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
			"mallName": "%s"
		}]
	}`, productTitle, newPrice, productLink, productImage, productMallName)

	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}
	commandData.Filters.PriceLessThan = 100000 // 가격 필터 설정

	// 기존 결과 데이터 (이전 가격)
	resultData := &naverShoppingWatchPriceResultData{
		Products: []*naverShoppingProduct{
			{
				Title:    productTitle,
				LowPrice: 10000,
				Link:     productLink,
			},
		},
	}

	// 실행
	message, newResultData, err := task.runWatchPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message) // 가격 변경 시 메시지 있음
	// 가격 변경 시 메시지에 상품 정보 포함 확인
	require.Contains(t, message, productTitle)

	typedResultData, ok := newResultData.(*naverShoppingWatchPriceResultData)
	require.True(t, ok)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, 8000, typedResultData.Products[0].LowPrice)
}

func TestNaverShoppingTask_RunWatchPrice_WithFiltering(t *testing.T) {
	// 필터링 적용 시나리오
	mockFetcher := NewMockHTTPFetcher()

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
				"mallName": "테스트몰1"
			},
			{
				"title": "일반 테스트 상품",
				"lprice": "15000",
				"link": "https://example.com/product/2",
				"image": "https://example.com/image2.jpg",
				"mallName": "테스트몰2"
			},
			{
				"title": "저렴한 상품",
				"lprice": "5000",
				"link": "https://example.com/product/3",
				"image": "https://example.com/image3.jpg",
				"mallName": "테스트몰3"
			}
		]
	}`

	url := "https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim"
	mockFetcher.SetResponse(url, []byte(jsonContent))

	task := &naverShoppingTask{
		task: task{
			id:         TidNaverShopping,
			commandID:  TcidNaverShoppingWatchPriceAny,
			notifierID: "test-notifier",
			fetcher:    mockFetcher,
		},
		clientID:     "test-client-id",
		clientSecret: "test-client-secret",
	}

	commandData := &naverShoppingWatchPriceTaskCommandData{
		Query: "테스트",
	}
	// 가격 필터: 20000원 미만만
	commandData.Filters.PriceLessThan = 20000
	// 포함 키워드: "테스트"
	commandData.Filters.IncludedKeywords = "테스트"

	resultData := &naverShoppingWatchPriceResultData{
		Products: make([]*naverShoppingProduct, 0),
	}

	// 실행
	message, newResultData, err := task.runWatchPrice(commandData, resultData, true)

	// 검증
	require.NoError(t, err)
	require.NotEmpty(t, message)

	typedResultData, ok := newResultData.(*naverShoppingWatchPriceResultData)
	require.True(t, ok)
	// 필터링 결과: "일반 테스트 상품"만 포함 (가격 15000원, "테스트" 포함)
	require.Equal(t, 1, len(typedResultData.Products))
	require.Equal(t, "일반 테스트 상품", typedResultData.Products[0].Title)
	require.Equal(t, 15000, typedResultData.Products[0].LowPrice)
}
