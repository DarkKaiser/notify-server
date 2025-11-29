package task

import (
	"testing"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNaverShoppingTask_RunWatchPrice(t *testing.T) {
	t.Run("정상적인 상품 검색 및 필터링", func(t *testing.T) {
		mockFetcher := NewMockHTTPFetcher()

		// Mock Naver Shopping API response
		apiResponse := `{
			"total": 2,
			"start": 1,
			"display": 2,
			"items": [
				{
					"title": "테스트 상품1",
					"link": "https://shopping.naver.com/product/1",
					"lprice": "5000",
					"mallName": "테스트몰1",
					"productId": "1",
					"productType": "1"
				},
				{
					"title": "테스트 상품2",
					"link": "https://shopping.naver.com/product/2",
					"lprice": "8000",
					"mallName": "테스트몰2",
					"productId": "2",
					"productType": "1"
				}
			]
		}`
		mockFetcher.SetResponse("https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim", []byte(apiResponse))

		// Task setup
		task := &naverShoppingTask{
			task: task{
				id:        TidNaverShopping,
				commandID: TaskCommandID("WatchPrice_Test"),
				fetcher:   mockFetcher,
			},
			config:       &g.AppConfig{},
			clientID:     "test_client_id",
			clientSecret: "test_client_secret",
		}

		taskCommandData := &naverShoppingWatchPriceTaskCommandData{
			Query: "테스트",
		}
		taskCommandData.Filters.IncludedKeywords = ""
		taskCommandData.Filters.ExcludedKeywords = ""
		taskCommandData.Filters.PriceLessThan = 10000

		taskResultData := &naverShoppingWatchPriceResultData{}
		message, changedData, err := task.runWatchPrice(taskCommandData, taskResultData, false)

		require.NoError(t, err)
		assert.Contains(t, message, "테스트 상품1", "상품1이 메시지에 포함되어야 합니다")
		assert.Contains(t, message, "테스트 상품2", "상품2가 메시지에 포함되어야 합니다")

		require.NotNil(t, changedData)
		resultData := changedData.(*naverShoppingWatchPriceResultData)
		assert.Equal(t, 2, len(resultData.Products), "2개의 상품이 추출되어야 합니다")
		assert.Equal(t, "테스트 상품1", resultData.Products[0].Title)
		assert.Equal(t, 5000, resultData.Products[0].LowPrice)
		assert.Equal(t, "테스트 상품2", resultData.Products[1].Title)
		assert.Equal(t, 8000, resultData.Products[1].LowPrice)
	})

	t.Run("가격 필터링 테스트", func(t *testing.T) {
		mockFetcher := NewMockHTTPFetcher()

		// Mock API response with products above and below price threshold
		apiResponse := `{
			"total": 3,
			"start": 1,
			"display": 3,
			"items": [
				{
					"title": "저가 상품",
					"link": "https://shopping.naver.com/product/1",
					"lprice": "5000",
					"mallName": "테스트몰1",
					"productId": "1",
					"productType": "1"
				},
				{
					"title": "고가 상품",
					"link": "https://shopping.naver.com/product/2",
					"lprice": "15000",
					"mallName": "테스트몰2",
					"productId": "2",
					"productType": "1"
				}
			]
		}`
		mockFetcher.SetResponse("https://openapi.naver.com/v1/search/shop.json?query=%ED%85%8C%EC%8A%A4%ED%8A%B8&display=100&start=1&sort=sim", []byte(apiResponse))

		task := &naverShoppingTask{
			task: task{
				id:        TidNaverShopping,
				commandID: TaskCommandID("WatchPrice_Test"),
				fetcher:   mockFetcher,
			},
			config:       &g.AppConfig{},
			clientID:     "test_client_id",
			clientSecret: "test_client_secret",
		}

		taskCommandData := &naverShoppingWatchPriceTaskCommandData{
			Query: "테스트",
		}
		taskCommandData.Filters.IncludedKeywords = ""
		taskCommandData.Filters.ExcludedKeywords = ""
		taskCommandData.Filters.PriceLessThan = 10000

		taskResultData := &naverShoppingWatchPriceResultData{}
		message, changedData, err := task.runWatchPrice(taskCommandData, taskResultData, false)

		require.NoError(t, err)
		assert.Contains(t, message, "저가 상품", "저가 상품만 포함되어야 합니다")
		assert.NotContains(t, message, "고가 상품", "고가 상품은 제외되어야 합니다")

		require.NotNil(t, changedData)
		resultData := changedData.(*naverShoppingWatchPriceResultData)
		assert.Equal(t, 1, len(resultData.Products), "1개의 상품만 추출되어야 합니다")
		assert.Equal(t, "저가 상품", resultData.Products[0].Title)
	})
}
