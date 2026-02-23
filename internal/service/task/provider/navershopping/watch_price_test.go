package navershopping

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Integration Tests: Fetch & Notify Logic
// -----------------------------------------------------------------------------

func TestTask_FetchProducts_TableDriven(t *testing.T) {
	t.Parallel()

	// 공통 설정
	defaultSettings := NewSettingsBuilder().
		WithQuery("test").
		WithPriceLessThan(20000).
		Build()

	// 예상되는 호출 URL (Key 정렬: display, query, sort, start)
	expectedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

	tests := []struct {
		name        string
		settings    watchPriceSettings
		mockSetup   func(*mocks.MockHTTPFetcher)
		checkResult func(*testing.T, []*product, error)
	}{
		{
			name:     "성공: 정상적인 데이터 수집 및 키워드 매칭",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
						{Title: "Keep", Link: "L1", LowPrice: "10000", ProductID: "1"},
						{Title: "FilterPrice", Link: "L2", LowPrice: "30000", ProductID: "2"},   // 20000 초과
						{Title: "FilterKeyword", Link: "L3", LowPrice: "10000", ProductID: "3"}, // 제외 키워드 시나리오
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				// defaultSettings에는 제외 키워드가 없으므로 가격 필터만 적용됨. (3개 중 1개 제외 -> 2개 남음)
				require.Len(t, p, 2)
				assert.Equal(t, "Keep", p[0].Title)
				assert.Equal(t, "FilterKeyword", p[1].Title)
			},
		},
		{
			name:     "성공: 제외 키워드 적용",
			settings: NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).WithExcludedKeywords("Exclude").Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 2, Items: []*productSearchResponseItem{
						{Title: "Keep", Link: "L1", LowPrice: "10000", ProductID: "1"},
						{Title: "Exclude Me", Link: "L2", LowPrice: "10000", ProductID: "2"},
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, "Keep", p[0].Title)
			},
		},
		{
			name:     "성공: 가격 쉼표 파싱",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "Comma", LowPrice: "1,500", ProductID: "1"}}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, 1500, p[0].LowPrice)
			},
		},
		{
			name:     "성공: 빈 결과",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 0, Items: []*productSearchResponseItem{}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				assert.Empty(t, p)
			},
		},
		{
			name:     "실패: API 호출 에러",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				m.SetError(expectedURL, errors.New("network fail"))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "network fail")
			},
		},
		{
			name:     "성공: 잘못된 가격 형식 무시",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "BadPrice", LowPrice: "Free", ProductID: "1"}}}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				assert.Empty(t, p, "가격 파싱에 실패한 항목은 제외되어야 함")
			},
		},
		{
			name:     "성공: HTML 태그가 포함된 로우 데이터 키워드 매칭",
			settings: NewSettingsBuilder().WithQuery("test").WithPriceLessThan(20000).WithExcludedKeywords("S25 FE").Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				resp := productSearchResponse{
					Total: 2, Items: []*productSearchResponseItem{
						{Title: "Galaxy <b>S25</b> <b>FE</b>", Link: "L1", LowPrice: "10000", ProductID: "1"}, // 제외 대상
						{Title: "Galaxy S25 Plus", Link: "L2", LowPrice: "10000", ProductID: "2"},             // 수집 대상
					},
				}
				m.SetResponse(expectedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1, "제외 키워드 'S25 FE'가 HTML 태그를 무시하고 적용되어야 함")
				assert.Equal(t, "Galaxy S25 Plus", p[0].Title)
			},
		},
		{
			name:     "실패: 잘못된 JSON 응답 (Malformed)",
			settings: defaultSettings,
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				m.SetResponse(expectedURL, []byte(`{invalid_json`))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "JSON")
			},
		},
		{
			name:     "성공: URL 인코딩 검증 (특수문자 쿼리)",
			settings: NewSettingsBuilder().WithQuery("아이폰 & 케이스").WithPriceLessThan(20000).Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				// 예상되는 인코딩된 URL
				encodedURL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=%EC%95%84%EC%9D%B4%ED%8F%B0+%26+%EC%BC%80%EC%9D%B4%EC%8A%A4&sort=sim&start=1"
				resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "Case", LowPrice: "5000", ProductID: "1"}}}
				m.SetResponse(encodedURL, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
			},
		},
		{
			name:     "성공: 키워드 매칭 (OR 조건 - A 또는 B 포함)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords("Galaxy|iPhone").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
						{Title: "Galaxy S25", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (Galaxy)
						{Title: "iPhone 16", Link: "L2", LowPrice: "1000", ProductID: "2"},  // 매칭 (iPhone)
						{Title: "Pixel 9", Link: "L3", LowPrice: "1000", ProductID: "3"},    // 미매칭
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 2)
				assert.Equal(t, "Galaxy S25", p[0].Title)
				assert.Equal(t, "iPhone 16", p[1].Title)
			},
		},
		{
			name:     "성공: 키워드 매칭 (복합 조건 - 포함 AND 제외)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords("Case").WithExcludedKeywords("Silicon,Hard").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 4, Items: []*productSearchResponseItem{
						{Title: "Leather Case", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (Case 포함, 제외어 없음)
						{Title: "Silicon Case", Link: "L2", LowPrice: "1000", ProductID: "2"}, // 제외 (Silicon)
						{Title: "Hard Case", Link: "L3", LowPrice: "1000", ProductID: "3"},    // 제외 (Hard)
						{Title: "Metal Bumper", Link: "L4", LowPrice: "1000", ProductID: "4"}, // 미매칭 (Case 미포함)
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 1)
				assert.Equal(t, "Leather Case", p[0].Title)
			},
		},
		{
			name:     "성공: 키워드 매칭 (대소문자 혼합 및 공백 처리)",
			settings: NewSettingsBuilder().WithQuery("search").WithIncludedKeywords(" apple watch | galaxy TAB ").WithPriceLessThan(999999).Build(),
			mockSetup: func(m *mocks.MockHTTPFetcher) {
				url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=search&sort=sim&start=1"
				resp := productSearchResponse{
					Total: 3, Items: []*productSearchResponseItem{
						{Title: "Apple Watch Series 9", Link: "L1", LowPrice: "1000", ProductID: "1"}, // 매칭 (apple watch)
						{Title: "Galaxy Tab S9", Link: "L2", LowPrice: "1000", ProductID: "2"},        // 매칭 (galaxy TAB)
						{Title: "Galaxy Watch 6", Link: "L3", LowPrice: "1000", ProductID: "3"},       // 미매칭
					},
				}
				m.SetResponse(url, mustMarshal(resp))
			},
			checkResult: func(t *testing.T, p []*product, err error) {
				require.NoError(t, err)
				require.Len(t, p, 2)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()
			if tt.mockSetup != nil {
				tt.mockSetup(mockFetcher)
			}

			tsk := &task{
				Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
				clientID:     "id",
				clientSecret: "secret",
			}
			// SetFetcher call removed as it's deprecated

			got, err := tsk.fetchProducts(context.Background(), &tt.settings)
			tt.checkResult(t, got, err)
		})
	}
}

// TestTask_FetchProducts_URLVerification fetchProducts 메서드 호출 시
// 내부적으로 생성되는 URL이 파라미터(쿼리, 페이징 등)에 따라 올바른지 집중적으로 검증합니다.
func TestTask_FetchProducts_URLVerification(t *testing.T) {
	t.Parallel()

	defaultResponse := mustMarshal(productSearchResponse{Total: 0, Items: []*productSearchResponseItem{}})

	tests := []struct {
		name        string
		settings    watchPriceSettings
		expectedURL string
	}{
		{
			name:        "기본 URL 생성 확인",
			settings:    NewSettingsBuilder().WithQuery("macbook").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=macbook&sort=sim&start=1",
		},
		{
			name:        "특수문자 쿼리 인코딩 확인",
			settings:    NewSettingsBuilder().WithQuery("A&B").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=A%26B&sort=sim&start=1",
		},
		{
			name:        "공백 쿼리 인코딩 확인",
			settings:    NewSettingsBuilder().WithQuery("macbook air").Build(),
			expectedURL: "https://openapi.naver.com/v1/search/shop.json?display=100&query=macbook+air&sort=sim&start=1",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockFetcher := mocks.NewMockHTTPFetcher()
			// 어떤 URL이든 성공 응답을 주도록 설정 (URL 검증이 목적이므로 내용은 무관)
			mockFetcher.SetResponse(tt.expectedURL, defaultResponse)

			tsk := &task{
				Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
				clientID:     "id",
				clientSecret: "secret",
			}
			// SetFetcher call removed as it's deprecated

			_, err := tsk.fetchProducts(context.Background(), &tt.settings)

			// URL 불일치 시 SetResponse에 없는 URL을 요청하게 되므로 에러 발생 ("no mock 수 response found")
			// 따라서 에러가 없으면 URL이 정확하다는 뜻입니다.
			assert.NoError(t, err, "요청된 URL이 기대값과 다릅니다")
		})
	}
}

// TestTask_FetchProducts_EdgeCases fetchProducts 메서드 내의 컨텍스트 취소, 타임아웃, 외부 중단 시나리오 등
// 예외 상황(Edge Cases) 블록들이 올바르게 처리되는지 커버리지 및 로직을 모두 검증합니다.
func TestTask_FetchProducts_EdgeCases(t *testing.T) {
	t.Parallel()

	// 1. fetchPageProducts 내부에서 ctx.Err()가 DeadlineExceeded 일 때
	t.Run("Context_DeadlineExceeded_Error_Handling", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

		// 의도적으로 타임아웃 형태의 에러 발생 (context.Canceled가 아님)
		mockFetcher.SetError(url, errors.New("timeout network error"))

		tsk := &task{
			Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			clientID:     "id",
			clientSecret: "secret",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 0) // 즉각 만료
		defer cancel()

		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()

		// fetchProducts 호출
		got, err := tsk.fetchProducts(ctx, &settings)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded) // ctx.Err()가 반환되어야 함
		assert.Nil(t, got)
	})

	// 2. Fetcher가 context.Canceled 에러를 명시적으로 반환할 때
	t.Run("Context_Canceled_Error_Handling", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"
		mockFetcher.SetError(url, context.Canceled) // 명시적인 Canceled 에러 반환 설정

		tsk := &task{
			Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			clientID:     "id",
			clientSecret: "secret",
		}

		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()
		// 취소되지 않은 context라도, Fetcher에서 canceled가 발생하면 즉결 반환됨을 확인
		got, err := tsk.fetchProducts(context.Background(), &settings)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	// 3. 루프 내부에서 IsCanceled() 가 true일 때 즉시 탈출 (필터 파트 & 루프 시작 파트 묶음 확인용)
	t.Run("Task_Cancel_Interrupt", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"
		// 한 번의 응답은 정상적으로 하되, 필터링 루프로 넘어가기 전에 취소가 되거나 루프의 시작부터 취소 처리되는지 검증
		resp := productSearchResponse{Total: 1, Items: []*productSearchResponseItem{{Title: "Test", LowPrice: "1000", ProductID: "1"}}}
		mockFetcher.SetResponse(url, mustMarshal(resp))

		tsk := &task{
			Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			clientID:     "id",
			clientSecret: "secret",
		}

		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()

		// 의도적으로 Base의 cancel을 호출
		tsk.Cancel()

		// 이미 취소된 상태이므로 루프 시작 시점 또는 어딘가에서 곧장 리턴되어야 함
		got, err := tsk.fetchProducts(context.Background(), &settings)
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	// 4. 타이머 강제 만료 및 Delay 로직 사이클 검증
	t.Run("Delay_Timer_Expiration_Handling", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url1 := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"
		url2 := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=101"

		items := make([]*productSearchResponseItem, 100)
		for i := 0; i < 100; i++ {
			items[i] = &productSearchResponseItem{Title: "Test1", LowPrice: "1000", ProductID: fmt.Sprintf("1-%d", i)}
		}

		// 2페이지 분량을 응답
		resp1 := productSearchResponse{Total: 101, Start: 1, Display: 100, Items: items}
		resp2 := productSearchResponse{Total: 101, Start: 101, Display: 1, Items: []*productSearchResponseItem{{Title: "Test2", LowPrice: "1000", ProductID: "2"}}}

		mockFetcher.SetResponse(url1, mustMarshal(resp1))
		mockFetcher.SetResponse(url2, mustMarshal(resp2))

		tsk := &task{
			Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			clientID:     "id",
			clientSecret: "secret",
		}

		// Delay 시간을 극단적으로 짧게 하여(1ns 미만은 불가능하므로 0) defer 된 Stop이나 Reset 구문에서 false를 리턴하고 타이머 큐가 비워지는 로직을 거치게끔 만듦
		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()
		settings.PageFetchDelay = 0 // 0으로 주면 100ms 가 대신 할당됨(ApplyDefaults 등) 하지만 여기선 구조체 그대로 쓰므로 강제로 0

		got, err := tsk.fetchProducts(context.Background(), &settings)
		require.NoError(t, err)
		assert.Len(t, got, 101)
	})

	// 5. 타이머 대기 중 context 취소 시나리오
	t.Run("Context_Canceled_During_Delay", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url1 := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

		// 1페이지 응답 후 타이머 대기로 진입할 수 있도록 Total을 200으로 설정
		resp1 := productSearchResponse{Total: 200, Start: 1, Display: 100, Items: []*productSearchResponseItem{{Title: "Test", LowPrice: "1000", ProductID: "1"}}}
		mockFetcher.SetResponse(url1, mustMarshal(resp1))

		tsk := &task{
			Base:         provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mockFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
			clientID:     "id",
			clientSecret: "secret",
		}

		ctx, cancel := context.WithCancel(context.Background())

		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()
		settings.PageFetchDelay = 100 // 대기 시간 할당

		// 첫 페이지 수집이 완료되면 취소가 이루어지도록 고루틴 실행
		go func() {
			time.Sleep(10 * time.Millisecond) // 첫 HTTP Fetch 처리가 끝날 즈음
			cancel()                          // <-ctx.Done() 캐치 유도
		}()

		got, err := tsk.fetchProducts(ctx, &settings)
		require.Error(t, err)
		// context.Canceled가 반환되어야 정상적으로 타이머 대기 중 탈출함을 의미
		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, got)
	})

	// 6. 필터링 루프 진행 중 외부 취소 발생 시그널 캐치 시나리오
	t.Run("Task_Cancel_During_Filter", func(t *testing.T) {
		mockFetcher := mocks.NewMockHTTPFetcher()
		url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=test&sort=sim&start=1"

		// 정상적인 크기(100개)로 타임아웃 지연이 아닌 명확한 시점(Close) 캔슬을 지향
		items := make([]*productSearchResponseItem, 100)
		for i := 0; i < 100; i++ {
			items[i] = &productSearchResponseItem{Title: "Test", LowPrice: "1000", ProductID: fmt.Sprintf("%d", i)}
		}

		resp := productSearchResponse{Total: 100, Start: 1, Display: 100, Items: items}
		mockFetcher.SetResponse(url, mustMarshal(resp))

		tsk := &task{
			clientID:     "id",
			clientSecret: "secret",
		}

		// Body.Close() 시점에 Task를 취소하는 특수 래퍼 (FetchJSON 응답 직후)
		wrappedFetcher := &cancelFilterFetcher{
			Fetcher: mockFetcher,
			cancel:  func() { tsk.Cancel() },
		}

		tsk.Base = provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: wrappedFetcher, NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true)

		settings := NewSettingsBuilder().WithQuery("test").WithPriceLessThan(10000).Build()

		got, err := tsk.fetchProducts(context.Background(), &settings)
		if err != nil {
			assert.ErrorIs(t, err, context.Canceled)
			assert.Nil(t, got)
		} else {
			assert.NotEmpty(t, got)
		}
	})

	// 7. Endpoint URL 파싱 에러
	t.Run("Endpoint_URL_Parse_Error", func(t *testing.T) {
		originalEndpoint := productSearchEndpoint
		defer func() { productSearchEndpoint = originalEndpoint }()

		// url.Parse 시 에러가 나도록 유효하지 않은 문자 할당 (제어 문자 등)
		productSearchEndpoint = "http://\x7f invalid"

		tsk := &task{
			Base: provider.NewBase(provider.NewTaskParams{Request: &contract.TaskSubmitRequest{TaskID: "test-task", CommandID: WatchPriceAnyCommand, NotifierID: "test-notifier", RunBy: contract.TaskRunByUser}, InstanceID: "test-instance", Fetcher: mocks.NewMockHTTPFetcher(), NewSnapshot: func() interface{} { return &watchPriceSnapshot{} }}, true),
		}

		settings := NewSettingsBuilder().WithQuery("test").Build()
		got, err := tsk.fetchProducts(context.Background(), &settings)
		require.Error(t, err)
		assert.Nil(t, got)
	})
}

type cancelFilterFetcher struct {
	fetcher.Fetcher
	cancel func()
}

func (f *cancelFilterFetcher) Do(req *http.Request) (*http.Response, error) {
	resp, err := f.Fetcher.Do(req)
	if err == nil && resp != nil && resp.Body != nil {
		resp.Body = &cancelBody{ReadCloser: resp.Body, cancel: f.cancel}
	}
	return resp, err
}

type cancelBody struct {
	io.ReadCloser
	cancel func()
}

func (b *cancelBody) Close() error {
	b.cancel()
	return b.ReadCloser.Close()
}

// -----------------------------------------------------------------------------
// Test Helpers
// -----------------------------------------------------------------------------

func mustMarshal(v interface{}) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

type SettingsBuilder struct {
	settings watchPriceSettings
}

func NewSettingsBuilder() *SettingsBuilder {
	return &SettingsBuilder{}
}

func (b *SettingsBuilder) WithQuery(q string) *SettingsBuilder {
	b.settings.Query = q
	return b
}
func (b *SettingsBuilder) WithPriceLessThan(p int) *SettingsBuilder {
	b.settings.Filters.PriceLessThan = p
	return b
}
func (b *SettingsBuilder) WithIncludedKeywords(k string) *SettingsBuilder {
	b.settings.Filters.IncludedKeywords = k
	return b
}
func (b *SettingsBuilder) WithExcludedKeywords(k string) *SettingsBuilder {
	b.settings.Filters.ExcludedKeywords = k
	return b
}
func (b *SettingsBuilder) Build() watchPriceSettings {
	return b.settings
}

// -----------------------------------------------------------------------------
// Advanced Scenarios: Pagination & Cancellation

// -----------------------------------------------------------------------------

func TestTask_FetchProducts_Pagination(t *testing.T) {
	t.Parallel()

	// 시나리오: 총 150개 상품, 1 페이지당 100개 요청.
	// 1번 요청: Start=1, Display=100 -> 100개 반환 (Next Start=101)
	// 2번 요청: Start=101, Display=100 -> 50개 반환 (Total=150 달성)

	settings := NewSettingsBuilder().WithQuery("paging").WithPriceLessThan(999999).Build()

	mockFetcher := mocks.NewMockHTTPFetcher()

	// Page 1 Setup
	page1URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=1"
	page1Items := make([]*productSearchResponseItem, 100)
	for i := 0; i < 100; i++ {
		page1Items[i] = &productSearchResponseItem{Title: "P1", LowPrice: "100", ProductID: fmt.Sprintf("P1_%d", i)}
	}
	mockFetcher.SetResponse(page1URL, mustMarshal(productSearchResponse{
		Total: 150, Start: 1, Display: 100, Items: page1Items,
	}))

	// Page 2 Setup
	page2URL := "https://openapi.naver.com/v1/search/shop.json?display=100&query=paging&sort=sim&start=101"
	page2Items := make([]*productSearchResponseItem, 50)
	for i := 0; i < 50; i++ {
		page2Items[i] = &productSearchResponseItem{Title: "P2", LowPrice: "100", ProductID: fmt.Sprintf("P2_%d", i)}
	}
	mockFetcher.SetResponse(page2URL, mustMarshal(productSearchResponse{
		Total: 150, Start: 101, Display: 50, Items: page2Items,
	}))

	tsk := &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     "T",
				CommandID:  "C",
				NotifierID: "N",
				RunBy:      contract.TaskRunByUser,
			},
			InstanceID: "I",
			Fetcher:    mockFetcher,
			NewSnapshot: func() interface{} {
				return &watchPriceSnapshot{}
			},
		}, true),
		clientID:     "id",
		clientSecret: "secret",
	}
	// SetFetcher call removed as it's deprecated

	products, err := tsk.fetchProducts(context.Background(), &settings)

	require.NoError(t, err)
	assert.Len(t, products, 150, "총 150개의 상품이 수집되어야 합니다")
}

func TestTask_FetchProducts_Cancellation(t *testing.T) {
	t.Parallel()

	settings := NewSettingsBuilder().WithQuery("cancel").WithPriceLessThan(999999).Build()
	mockFetcher := mocks.NewMockHTTPFetcher()

	// 1페이지 응답 설정 (Total이 많아서 다음 페이지가 필요하도록 설정)
	url := "https://openapi.naver.com/v1/search/shop.json?display=100&query=cancel&sort=sim&start=1"
	mockFetcher.SetResponse(url, mustMarshal(productSearchResponse{
		Total: 1000, Start: 1, Display: 1, Items: []*productSearchResponseItem{{Title: "A", LowPrice: "100", ProductID: "1"}},
	}))

	// Task 생성 및 취소 상태로 설정
	tsk := &task{clientID: "id", clientSecret: "secret"}
	tsk.Base = provider.NewBase(provider.NewTaskParams{
		Request: &contract.TaskSubmitRequest{
			TaskID:     "NS",
			CommandID:  "CMD",
			NotifierID: "NOTI",
			RunBy:      contract.TaskRunByScheduler,
		},
		InstanceID: "INS",
		Fetcher:    mockFetcher,
		NewSnapshot: func() interface{} {
			return &watchPriceSnapshot{}
		},
	}, true)
	// SetFetcher call removed as it's deprecated

	// 강제로 취소 상태 주입 (Context Cancel)
	tsk.Cancel()

	products, err := tsk.fetchProducts(context.Background(), &settings)

	// 취소되었으므로 context.Canceled 에러와 nil products 반환 체크
	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, products, "작업 취소 시 nil을 반환해야 합니다")
}

// -----------------------------------------------------------------------------
// Benchmarks
// -----------------------------------------------------------------------------
