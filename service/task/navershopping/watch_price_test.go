package navershopping

import (
	"encoding/json"
	"errors"
	"testing"

	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatchPriceSettings_Validate(t *testing.T) {
	tests := []struct {
		name        string
		settings    watchPriceSettings
		expectedErr string
	}{
		{
			name: "정상적인 설정",
			settings: watchPriceSettings{
				Query: "test_query",
				Filters: struct {
					IncludedKeywords string `json:"included_keywords"`
					ExcludedKeywords string `json:"excluded_keywords"`
					PriceLessThan    int    `json:"price_less_than"`
				}{
					PriceLessThan: 10000,
				},
			},
			expectedErr: "",
		},
		{
			name: "Query 누락",
			settings: watchPriceSettings{
				Query: "",
				Filters: struct {
					IncludedKeywords string `json:"included_keywords"`
					ExcludedKeywords string `json:"excluded_keywords"`
					PriceLessThan    int    `json:"price_less_than"`
				}{
					PriceLessThan: 10000,
				},
			},
			expectedErr: "query",
		},
		{
			name: "Query 공백",
			settings: watchPriceSettings{
				Query: "   ",
				Filters: struct {
					IncludedKeywords string `json:"included_keywords"`
					ExcludedKeywords string `json:"excluded_keywords"`
					PriceLessThan    int    `json:"price_less_than"`
				}{
					PriceLessThan: 10000,
				},
			},
			expectedErr: "query",
		},
		{
			name: "PriceLessThan 0 이하",
			settings: watchPriceSettings{
				Query: "test_query",
				Filters: struct {
					IncludedKeywords string `json:"included_keywords"`
					ExcludedKeywords string `json:"excluded_keywords"`
					PriceLessThan    int    `json:"price_less_than"`
				}{
					PriceLessThan: 0,
				},
			},
			expectedErr: "price_less_than",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.validate()
			if tt.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProduct_String(t *testing.T) {
	p := &product{
		Title:       "Test Product",
		Link:        "http://example.com",
		LowPrice:    10000,
		MallName:    "Test Mall",
		ProductID:   "123456",
		ProductType: "1",
	}

	tests := []struct {
		name         string
		supportsHTML bool
		mark         string
		expected     []string
		notExpected  []string
	}{
		{
			name:         "HTML - No Mark",
			supportsHTML: true,
			mark:         "",
			expected:     []string{"<a href=\"http://example.com\"><b>Test Product</b></a>", "(Test Mall)", "10,000원"},
			notExpected:  []string{"Test Product (Test Mall) 10,000원 🆕"},
		},
		{
			name:         "HTML - With Mark",
			supportsHTML: true,
			mark:         " 🆕",
			expected:     []string{"<a href=\"http://example.com\"><b>Test Product</b></a>", "(Test Mall)", "10,000원 🆕"},
		},
		{
			name:         "Text - No Mark",
			supportsHTML: false,
			mark:         "",
			expected:     []string{"☞ Test Product (Test Mall) 10,000원", "http://example.com"},
			notExpected:  []string{"<a href"},
		},
		{
			name:         "Text - With Mark",
			supportsHTML: false,
			mark:         " 🆕",
			expected:     []string{"☞ Test Product (Test Mall) 10,000원 🆕"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.String(tt.supportsHTML, tt.mark)
			for _, exp := range tt.expected {
				assert.Contains(t, result, exp)
			}
			for _, nexp := range tt.notExpected {
				assert.NotContains(t, result, nexp)
			}
		})
	}
}

func TestTask_FetchProducts(t *testing.T) {
	// Setup
	mockFetcher := testutil.NewMockHTTPFetcher()
	tsk := &task{
		clientID:     "test_id",
		clientSecret: "test_secret",
	}
	tsk.SetFetcher(mockFetcher)

	baseSettings := &watchPriceSettings{
		Query: "test",
		Filters: struct {
			IncludedKeywords string `json:"included_keywords"`
			ExcludedKeywords string `json:"excluded_keywords"`
			PriceLessThan    int    `json:"price_less_than"`
		}{
			PriceLessThan: 20000,
		},
	}

	t.Run("성공: 데이터 수집 및 필터링", func(t *testing.T) {
		// Mock Response
		response := searchResponse{
			Total:   2,
			Start:   1,
			Display: 2,
			Items: []*searchResponseItem{
				{
					Title:     "Test Product 1",
					Link:      "http://example.com/1",
					LowPrice:  "10000",
					ProductID: "111",
				},
				{
					Title:     "Test Product 2 (Excluded)",
					Link:      "http://example.com/2",
					LowPrice:  "15000",
					ProductID: "222",
				},
				{
					Title:     "Test Product 3 (Expensive)",
					Link:      "http://example.com/3",
					LowPrice:  "30000",
					ProductID: "333",
				},
			},
		}
		responseJSON, _ := json.Marshal(response)
		// URL 매칭을 위해 Query와 Encode 로직을 고려해야 함.
		// 테스트 편의상 mockFetcher가 모든 URL에 대해 동일 응답을 주도록 설정하거나,
		// 정확한 URL을 예측해야 함. 여기서는 정확한 매칭을 시도.
		// searchAPIURL + query params.
		// query params 순서는 map iteration 순서에 따르므로 예측이 어려울 수 있음.
		// 하지만 FetchJSON 호출 시 u.String()을 사용.
		// mockFetcher.SetResponse(...)는 URL이 정확해야 함.
		// 하지만 testutil.MockHTTPFetcher 구현을 보면, URL을 키로 맵에 저장하지 않고,
		// SetResponse 호출 시 저장해두고 Get 호출 시 반환하거나,
		// 좀 더 유연하게 동작할 필요가 있음.
		// testutil.NewMockHTTPFetcher 구현 확인 결과 (추정): 보통 SetResponse(url, body) 형태임.
		// URL 예측이 힘들다면 MockFetcher를 수정하거나, FetchProducts 내부 URL 생성 로직을 검증하는 별도 방법을 써야 함.
		// 일단 가장 일반적인 Happy Path URL을 구성.
		// query=test&display=100&start=1&sort=sim
		// url.Values Encode는 키 정렬을 보장함.
		expectedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"
		mockFetcher.SetResponse(expectedURL, responseJSON)

		// 제외 키워드 설정
		settings := *baseSettings
		settings.Filters.ExcludedKeywords = "Excluded"

		products, err := tsk.fetchProducts(&settings)
		require.NoError(t, err)

		// Product 2는 ExcludedKeywords 포함으로 제외
		// Product 3는 PriceLessThan(20000) 초과로 제외
		// Product 1만 남아야 함
		require.Len(t, products, 1)
		assert.Equal(t, "Test Product 1", products[0].Title)
	})

	t.Run("성공: 가격 쉼표 파싱", func(t *testing.T) {
		response := searchResponse{
			Total: 1,
			Items: []*searchResponseItem{
				{
					Title:    "Comma Price",
					LowPrice: "1,500", // 쉼표 포함
				},
			},
		}
		responseJSON, _ := json.Marshal(response)
		mockFetcher.SetResponse("https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1", responseJSON)

		products, err := tsk.fetchProducts(baseSettings)
		require.NoError(t, err)
		require.Len(t, products, 1)
		assert.Equal(t, 1500, products[0].LowPrice)
	})

	t.Run("실패: API 호출 에러", func(t *testing.T) {
		// SetError requires URL
		mockFetcher.SetError("https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1", errors.New("network error"))
		_, err := tsk.fetchProducts(baseSettings)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "network error")
	})

	t.Run("성공: 빈 결과", func(t *testing.T) {
		mockFetcher.Reset() // Clear previous errors and responses
		response := searchResponse{Total: 0, Items: []*searchResponseItem{}}
		responseJSON, _ := json.Marshal(response)
		mockFetcher.SetResponse("https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1", responseJSON)

		products, err := tsk.fetchProducts(baseSettings)
		require.NoError(t, err)
		assert.Empty(t, products)
	})
}

func TestTask_DiffAndNotify(t *testing.T) {
	tsk := &task{}
	// RunBy 설정 (tasksvc.Task 내장 필드 설정 필요)
	// 하지만 Task 구조체 내장 필드는 private일 수 있음 -> NewBaseTask로 생성된 Task를 임베딩했으므로,
	// t.RunBy 접근이 가능하거나 Set 메서드가 있는지 확인 필요.
	// Task 구조체는 tasksvc.BaseTask를 임베딩하고 있음.
	tsk.Task = tasksvc.NewBaseTask("taskID", "commandID", "instanceID", "notifierID", tasksvc.RunByScheduler)

	baseSettings := &watchPriceSettings{
		Query: "test",
		Filters: struct {
			IncludedKeywords string `json:"included_keywords"`
			ExcludedKeywords string `json:"excluded_keywords"`
			PriceLessThan    int    `json:"price_less_than"`
		}{
			PriceLessThan: 20000,
		},
	}

	p1 := &product{Title: "P1", Link: "L1", LowPrice: 10000, ProductID: "PID_1"}
	p2 := &product{Title: "P2", Link: "L2", LowPrice: 10000, ProductID: "PID_2"}

	t.Run("신규 상품 발견 (New)", func(t *testing.T) {
		current := &watchPriceSnapshot{Products: []*product{p1, p2}}
		prev := &watchPriceSnapshot{Products: []*product{p1}} // p2가 신규

		msg, _, err := tsk.diffAndNotify(baseSettings, current, prev, false)
		require.NoError(t, err)
		assert.Contains(t, msg, "상품의 정보가 변경되었습니다")
		assert.Contains(t, msg, "P2")
		assert.Contains(t, msg, "🆕")
	})

	t.Run("가격 변동 (Change)", func(t *testing.T) {
		p1Reduced := &product{Title: "P1", Link: "L1", LowPrice: 9000, ProductID: "PID_1"}
		current := &watchPriceSnapshot{Products: []*product{p1Reduced}}
		prev := &watchPriceSnapshot{Products: []*product{p1}} // 10000 -> 9000

		msg, _, err := tsk.diffAndNotify(baseSettings, current, prev, false)
		require.NoError(t, err)
		assert.Contains(t, msg, "변경되었습니다")
		assert.Contains(t, msg, "🔁")
		assert.Contains(t, msg, "9,000원")
	})

	t.Run("변경 사항 없음 (No Change - Scheduler)", func(t *testing.T) {
		tsk.Task = tasksvc.NewBaseTask("taskID", "commandID", "instanceID", "notifierID", tasksvc.RunByScheduler)
		current := &watchPriceSnapshot{Products: []*product{p1}}
		prev := &watchPriceSnapshot{Products: []*product{p1}}

		msg, _, err := tsk.diffAndNotify(baseSettings, current, prev, false)
		require.NoError(t, err)
		assert.Empty(t, msg, "스케줄러 실행 시 변경 없으면 빈 메시지여야 함")
	})

	t.Run("변경 사항 없음 (No Change - User)", func(t *testing.T) {
		tsk.Task = tasksvc.NewBaseTask("taskID", "commandID", "instanceID", "notifierID", tasksvc.RunByUser)
		current := &watchPriceSnapshot{Products: []*product{p1}}
		prev := &watchPriceSnapshot{Products: []*product{p1}}

		msg, _, err := tsk.diffAndNotify(baseSettings, current, prev, false)
		require.NoError(t, err)
		assert.NotEmpty(t, msg, "사용자 실행 시 변경 없어도 메시지 반환해야 함")
		assert.Contains(t, msg, "변경된 정보가 없습니다")
		assert.Contains(t, msg, "조회 조건은 아래와 같습니다")
	})

	t.Run("최초 실행 (Prev is Nil)", func(t *testing.T) {
		current := &watchPriceSnapshot{Products: []*product{p1}}

		msg, _, err := tsk.diffAndNotify(baseSettings, current, nil, false)
		require.NoError(t, err)
		assert.Contains(t, msg, "변경되었습니다")
		assert.Contains(t, msg, "🆕")
	})
}
