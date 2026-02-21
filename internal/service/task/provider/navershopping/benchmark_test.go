package navershopping

// =============================================================================
// Benchmark Test Suite — navershopping 패키지
// =============================================================================
//
// 실행 방법:
//
//	go test -bench=. -benchmem ./internal/service/task/provider/navershopping
//
// 특정 벤치마크만 실행:
//
//	go test -bench=BenchmarkParseProduct -benchmem ...
//
// 프로파일링:
//
//	go test -bench=. -benchmem -cpuprofile=cpu.out -memprofile=mem.out ...
//	go tool pprof cpu.out

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// =============================================================================
// 벤치마크 헬퍼
// =============================================================================

// newBenchTask 벤치마크 전용 경량 task를 생성합니다.
func newBenchTask(b *testing.B, runBy contract.TaskRunBy, fetcher ...*mocks.MockHTTPFetcher) *task {
	b.Helper()
	var f *mocks.MockHTTPFetcher
	if len(fetcher) > 0 {
		f = fetcher[0]
	} else {
		f = mocks.NewMockHTTPFetcher()
	}
	return &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     TaskID,
				CommandID:  WatchPriceAnyCommand,
				NotifierID: "bench-notifier",
				RunBy:      runBy,
			},
			InstanceID:  "bench-instance",
			Fetcher:     f,
			NewSnapshot: func() interface{} { return &watchPriceSnapshot{} },
		}, true),
	}
}

// makeBenchProducts 벤치마크용 상품 슬라이스를 생성합니다.
func makeBenchProducts(count int) []*product {
	products := make([]*product, count)
	for i := 0; i < count; i++ {
		products[i] = &product{
			ProductID:   fmt.Sprintf("%d", i),
			ProductType: "1",
			Title:       fmt.Sprintf("Benchmark Product %d", i),
			Link:        fmt.Sprintf("https://link/%d", i),
			LowPrice:    10000 + i*100,
			MallName:    "BenchMall",
		}
	}
	return products
}

// =============================================================================
// BenchmarkParseProduct — parseProduct 핫 패스 성능 측정
// =============================================================================

// BenchmarkParseProduct 단일 상품 항목을 도메인 모델로 변환하는 성능을 측정합니다.
//
// 측정 대상:
//   - HTML 태그 제거(strutil.StripHTML)
//   - 쉼표 제거 및 정수 파싱(strconv.Atoi)
func BenchmarkParseProduct(b *testing.B) {
	tsk := newBenchTask(b, contract.TaskRunByScheduler)

	b.Run("기본 ASCII 상품명", func(b *testing.B) {
		b.ReportAllocs()
		item := &productSearchResponseItem{
			Title: "Samsung Galaxy S24 Ultra 512GB", LowPrice: "1,350,000",
			ProductID: "123456", Link: "https://link", MallName: "Official Store",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tsk.parseProduct(item)
		}
	})

	b.Run("HTML 태그 포함 상품명", func(b *testing.B) {
		b.ReportAllocs()
		item := &productSearchResponseItem{
			Title: "<b>Apple</b> iPhone 15 <b>Pro Max</b> 256GB", LowPrice: "1,500,000",
			ProductID: "789012", Link: "https://link", MallName: "Apple Store",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tsk.parseProduct(item)
		}
	})

	b.Run("유효하지 않은 가격 (nil 반환 경로)", func(b *testing.B) {
		b.ReportAllocs()
		item := &productSearchResponseItem{
			Title: "Free Item", LowPrice: "N/A",
			ProductID: "000", Link: "https://link", MallName: "Mall",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = tsk.parseProduct(item)
		}
	})
}

// =============================================================================
// BenchmarkBuildProductSearchURL — URL 생성 성능 측정
// =============================================================================

// BenchmarkBuildProductSearchURL 쿼리 파라미터를 조합하여 API URL을 생성하는 성능을 측정합니다.
func BenchmarkBuildProductSearchURL(b *testing.B) {
	base, _ := url.Parse(productSearchEndpoint)

	b.Run("ASCII 쿼리", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = buildProductSearchURL(base, "iPhone 15 Pro", 1, defaultDisplayCount)
		}
	})

	b.Run("한글 쿼리 (URL 인코딩 포함)", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = buildProductSearchURL(base, "아이폰 15 프로 맥스", 101, defaultDisplayCount)
		}
	})
}

// =============================================================================
// BenchmarkSnapshotCompare — watchPriceSnapshot.Compare 성능 측정
// =============================================================================

// BenchmarkSnapshotCompare 스냅샷 비교 알고리즘(Map 색인 + 순회)의 성능을 측정합니다.
func BenchmarkSnapshotCompare(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		size := size
		b.Run(fmt.Sprintf("상품_%d개", size), func(b *testing.B) {
			b.ReportAllocs()

			prev := &watchPriceSnapshot{Products: makeBenchProducts(size)}

			// 절반은 가격 변동, 나머지 절반은 신규
			curr := &watchPriceSnapshot{Products: make([]*product, size)}
			copy(curr.Products, prev.Products)
			for i := size / 2; i < size; i++ {
				curr.Products[i] = &product{
					ProductID: fmt.Sprintf("%d", i), ProductType: "1",
					Title:    fmt.Sprintf("Benchmark Product %d", i),
					Link:     fmt.Sprintf("https://link/%d", i),
					LowPrice: 9000 + i*50, // 가격 변동
					MallName: "BenchMall",
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = curr.Compare(prev)
			}
		})
	}
}

// =============================================================================
// BenchmarkRenderProductDiffs — renderProductDiffs 렌더링 성능 측정
// =============================================================================

// BenchmarkRenderProductDiffs 변동 상품 목록을 알림 메시지로 렌더링하는 성능을 측정합니다.
func BenchmarkRenderProductDiffs(b *testing.B) {
	sizes := []int{1, 10, 100}

	for _, size := range sizes {
		size := size
		b.Run(fmt.Sprintf("diff_%d개", size), func(b *testing.B) {
			b.ReportAllocs()

			products := makeBenchProducts(size)
			diffs := make([]productDiff, size)
			for i, p := range products {
				if i%2 == 0 {
					diffs[i] = productDiff{Type: productEventNew, Product: p}
				} else {
					prev := &product{ProductID: p.ProductID, LowPrice: p.LowPrice + 1000}
					diffs[i] = productDiff{Type: productEventPriceChanged, Product: p, Prev: prev}
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = renderProductDiffs(diffs, false)
			}
		})
	}
}

// BenchmarkRenderProductDiffs_HTML HTML 모드에서의 렌더링 성능을 텍스트 모드와 비교합니다.
func BenchmarkRenderProductDiffs_HTML(b *testing.B) {
	b.ReportAllocs()

	products := makeBenchProducts(50)
	diffs := make([]productDiff, len(products))
	for i, p := range products {
		diffs[i] = productDiff{Type: productEventNew, Product: p}
	}

	b.Run("텍스트 모드", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = renderProductDiffs(diffs, false)
		}
	})

	b.Run("HTML 모드", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = renderProductDiffs(diffs, true)
		}
	})
}

// =============================================================================
// BenchmarkFormatProductItem — formatProductItem 핫 패스 성능 측정
// =============================================================================

// BenchmarkFormatProductItem 상품 단건 렌더링 성능을 측정합니다.
func BenchmarkFormatProductItem(b *testing.B) {
	p := &product{
		Title: "Apple MacBook Pro 14인치 M3 Pro", Link: "https://shopping.naver.com/products/12345678",
		LowPrice: 2990000, MallName: "Apple Premium Reseller",
	}
	prev := &product{LowPrice: 3200000}

	b.Run("신규 상품 (prev nil)", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = formatProductItem(p, false, mark.New, nil)
		}
	})

	b.Run("가격 변동 (prev 있음)", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = formatProductItem(p, false, mark.Modified, prev)
		}
	})
}

// =============================================================================
// BenchmarkAnalyzeAndReport — analyzeAndReport 종단 성능 측정
// =============================================================================

// BenchmarkAnalyzeAndReport 스냅샷 비교 → 메시지 렌더링 전체 파이프라인 성능을 측정합니다.
func BenchmarkAnalyzeAndReport(b *testing.B) {
	settings := NewSettingsBuilder().WithQuery("bench").WithPriceLessThan(999999).Build()

	sizes := []int{100, 500, 1000}
	for _, size := range sizes {
		size := size
		b.Run(fmt.Sprintf("상품_%d개_절반_변경", size), func(b *testing.B) {
			b.ReportAllocs()

			tsk := newBenchTask(b, contract.TaskRunByScheduler)
			prevItems := makeBenchProducts(size)
			currItems := make([]*product, size)
			copy(currItems, prevItems)
			// 절반 가격 변동
			for i := size / 2; i < size; i++ {
				cp := *currItems[i]
				cp.LowPrice = 9000 + i*50
				currItems[i] = &cp
			}

			prev := &watchPriceSnapshot{Products: prevItems}
			curr := &watchPriceSnapshot{Products: currItems}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = tsk.analyzeAndReport(&settings, curr, prev, false)
			}
		})
	}
}

// =============================================================================
// BenchmarkExecuteWatchPrice — executeWatchPrice 풀 파이프라인 성능 측정
// =============================================================================

// BenchmarkExecuteWatchPrice HTTP 모킹 → 파싱 → 필터 → 비교 → 렌더링까지의
// 전체 수집 파이프라인 성능을 측정합니다.
func BenchmarkExecuteWatchPrice(b *testing.B) {
	query := "아이폰"
	encodedQuery := url.QueryEscape(query)

	// 100개 상품 JSON 생성
	itemsJSON := ""
	for i := 0; i < 100; i++ {
		if i > 0 {
			itemsJSON += ","
		}
		itemsJSON += fmt.Sprintf(`{"title":"Bench <b>Product</b> %d","link":"https://link/%d","lprice":"%d","mallName":"Mall","productId":"%d","productType":"1"}`,
			i, i, 10000+i*100, i)
	}
	respJSON := fmt.Sprintf(`{"total":100,"start":1,"display":100,"items":[%s]}`, itemsJSON)

	pageURL := fmt.Sprintf("%s?display=100&query=%s&sort=sim&start=1", productSearchEndpoint, encodedQuery)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(pageURL, []byte(respJSON))

	tsk := newBenchTask(b, contract.TaskRunByScheduler, mockFetcher)
	tsk.clientID = "bench-id"
	tsk.clientSecret = "bench-secret"

	settings := NewSettingsBuilder().WithQuery(query).WithPriceLessThan(20000).Build()
	prevSnapshot := &watchPriceSnapshot{Products: make([]*product, 0)}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _, err := tsk.executeWatchPrice(context.Background(), &settings, prevSnapshot, false)
		if err != nil {
			b.Fatalf("executeWatchPrice 실패: %v", err)
		}
	}
}
