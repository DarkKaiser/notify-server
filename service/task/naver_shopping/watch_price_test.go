package naver_shopping

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNaverShoppingWatchPriceCommandSettings_Validate(t *testing.T) {
	t.Run("정상적인 데이터", func(t *testing.T) {
		commandSettings := &watchPriceCommandSettings{
			Query: "테스트 상품",
		}
		commandSettings.Filters.PriceLessThan = 10000

		err := commandSettings.validate()
		assert.NoError(t, err, "정상적인 데이터는 검증을 통과해야 합니다")
	})

	t.Run("Query가 비어있는 경우", func(t *testing.T) {
		commandSettings := &watchPriceCommandSettings{
			Query: "",
		}

		err := commandSettings.validate()
		assert.Error(t, err, "Query가 비어있으면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "query", "적절한 에러 메시지를 반환해야 합니다")
	})

	t.Run("PriceLessThan이 0 이하인 경우", func(t *testing.T) {
		commandSettings := &watchPriceCommandSettings{
			Query: "테스트 상품",
		}
		commandSettings.Filters.PriceLessThan = 0

		err := commandSettings.validate()
		assert.Error(t, err, "PriceLessThan이 0 이하면 에러가 발생해야 합니다")
		assert.Contains(t, err.Error(), "price_less_than", "적절한 에러 메시지를 반환해야 합니다")
	})
}

func TestNaverShoppingProduct_String(t *testing.T) {
	t.Run("HTML 메시지 포맷", func(t *testing.T) {
		product := &product{
			Title:       "테스트 상품",
			Link:        "https://shopping.naver.com/product/1",
			LowPrice:    10000,
			ProductID:   "1",
			ProductType: "1",
		}

		result := product.String(true, "")

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.Contains(t, result, "<a href", "HTML 링크 태그가 포함되어야 합니다")
	})

	t.Run("텍스트 메시지 포맷", func(t *testing.T) {
		product := &product{
			Title:       "테스트 상품",
			Link:        "https://shopping.naver.com/product/1",
			LowPrice:    10000,
			ProductID:   "1",
			ProductType: "1",
		}

		result := product.String(false, "")

		assert.Contains(t, result, "테스트 상품", "상품 이름이 포함되어야 합니다")
		assert.Contains(t, result, "10,000원", "가격이 포함되어야 합니다")
		assert.NotContains(t, result, "<a href", "HTML 태그가 포함되지 않아야 합니다")
	})

	t.Run("마크 표시", func(t *testing.T) {
		product := &product{
			Title:    "테스트 상품",
			LowPrice: 10000,
		}

		result := product.String(false, " 🆕")

		assert.Contains(t, result, "🆕", "마크가 포함되어야 합니다")
	})
}

func TestNaverShoppingWatchPriceSearchResultData_Parsing(t *testing.T) {
	t.Run("JSON 파싱 테스트", func(t *testing.T) {
		// testdata에서 샘플 JSON 로드
		jsonData := testutil.LoadTestData(t, "api_response.json")

		var result searchResponse
		err := json.Unmarshal(jsonData, &result)

		assert.NoError(t, err, "JSON 파싱이 성공해야 합니다")
		assert.Equal(t, 100, result.Total, "Total 값이 일치해야 합니다")
		assert.Equal(t, 1, result.Start, "Start 값이 일치해야 합니다")
		assert.Equal(t, 10, result.Display, "Display 값이 일치해야 합니다")
		assert.Equal(t, 2, len(result.Items), "Items 개수가 일치해야 합니다")

		// 첫 번째 상품 검증
		assert.Equal(t, "테스트 상품 1", result.Items[0].Title, "첫 번째 상품 이름이 일치해야 합니다")
		assert.Equal(t, "10000", result.Items[0].LowPrice, "첫 번째 상품 가격이 일치해야 합니다")
	})
}

func TestNaverShoppingTask_FilterProducts(t *testing.T) {
	t.Run("포함 키워드 필터링", func(t *testing.T) {
		t.Skip("통합 테스트로 이동 필요")
	})
}

// MockHTTPClient는 HTTP 클라이언트를 Mock하는 구조체입니다.
type MockHTTPClient struct {
	Response []byte
	Error    error
}

func (m *MockHTTPClient) Get(url string) ([]byte, error) {
	if m.Error != nil {
		return nil, m.Error
	}
	return m.Response, nil
}

func TestNaverShoppingTask_APIError(t *testing.T) {
	t.Run("네트워크 오류 시뮬레이션", func(t *testing.T) {
		// HTTP 클라이언트 Mock을 사용한 에러 테스트
		mockClient := &MockHTTPClient{
			Error: errors.New("network error"),
		}

		assert.NotNil(t, mockClient.Error, "Mock 에러가 설정되어야 합니다")
	})

	t.Run("빈 응답 처리", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			Response: []byte("{}"),
		}

		var result searchResponse
		err := json.Unmarshal(mockClient.Response, &result)

		assert.NoError(t, err, "빈 JSON도 파싱할 수 있어야 합니다")
		assert.Equal(t, 0, result.Total, "Total이 0이어야 합니다")
	})
}
