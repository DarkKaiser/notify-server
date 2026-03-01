package kurly

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// =============================================================================
// 공통 픽스처 생성 헬퍼
// =============================================================================

// generateFillerHTML N줄의 더미 HTML 콘텐츠를 생성합니다.
// strings.Builder를 사용하여 O(n) 성능을 보장합니다.
func generateFillerHTML(lines int) string {
	var sb strings.Builder
	sb.Grow(lines * 40) // 줄당 약 40바이트 사전 할당
	for i := range lines {
		fmt.Fprintf(&sb, "<div>Filler content line %d</div>\n", i)
	}
	return sb.String()
}

// buildBenchmarkHTML 벤치마크용 상품 상세 페이지 HTML을 생성합니다.
// fillerLines: HTML 복잡도를 높이기 위한 더미 줄 수.
func buildBenchmarkHTML(productID, fillerLines int) string {
	return fmt.Sprintf(`
<html>
<body>
	<script id="__NEXT_DATA__">
		{"props":{"pageProps":{"product":{"no":%d}}}}
	</script>
	<div id="product-atf">
		<section class="css-1ua1wyk">
			<div class="css-84rb3h">
				<div class="css-6zfm8o">
					<div class="css-o3fjh7">
						<h1>Test Product Name That Is Quite Long To Simulate Real World Data</h1>
					</div>
				</div>
			</div>
			<h2 class="css-xrp7wx">
				<span class="css-8h3us8">20%%</span>
				<div class="css-o2nlqt">
					<span>8,000</span>
					<span>원</span>
				</div>
			</h2>
			<span class="css-1s96j0s">
				<span>10,000원</span>
			</span>
			<div class="filler">%s</div>
		</section>
	</div>
</body>
</html>`, productID, generateFillerHTML(fillerLines))
}

// buildBenchmarkTask 벤치마크용 task를 생성합니다.
func buildBenchmarkTask(b *testing.B, fetcher *mocks.MockHTTPFetcher) *task {
	b.Helper()
	return &task{
		Base: provider.NewBase(provider.NewTaskParams{
			Request: &contract.TaskSubmitRequest{
				TaskID:     TaskID,
				CommandID:  WatchProductPriceCommand,
				NotifierID: "bench-notifier",
				RunBy:      contract.TaskRunByUnknown,
			},
			InstanceID: "bench_instance",
			Fetcher:    fetcher,
			NewSnapshot: func() interface{} {
				return &watchProductPriceSnapshot{}
			},
		}, true),
	}
}

// buildBenchmarkCSV N개 상품을 포함하는 임시 CSV 파일을 생성하고 경로를 반환합니다.
// Cleanup 등록이 포함되어 있으므로 defer 불필요합니다.
func buildBenchmarkCSV(b *testing.B, productIDs []int) string {
	b.Helper()
	var sb strings.Builder
	sb.WriteString("No,Name,Status\n")
	for _, id := range productIDs {
		fmt.Fprintf(&sb, "%d,상품_%d,1\n", id, id)
	}
	f, err := os.CreateTemp("", "benchmark_*.csv")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(sb.String()); err != nil {
		b.Fatal(err)
	}
	if err := f.Close(); err != nil {
		b.Fatal(err)
	}
	return f.Name()
}

// makeBenchmarkProducts N개의 상품 슬라이스를 생성합니다.
func makeBenchmarkProducts(n int) []*product {
	products := make([]*product, n)
	for i := range n {
		p := &product{
			ID:                 i + 1,
			Name:               fmt.Sprintf("상품_%d", i+1),
			Price:              10000 + i*100,
			DiscountedPrice:    9000 + i*100,
			DiscountRate:       10,
			LowestPrice:        8000,
			LowestPriceTimeUTC: time.Now().UTC(),
		}
		products[i] = p
	}
	return products
}

// =============================================================================
// BenchmarkKurlyTask_RunWatchProductPrice — 엔드 투 엔드 전체 흐름 벤치마크
// =============================================================================

// BenchmarkKurlyTask_RunWatchProductPrice HTML 파싱 → 가격 추출 → Diff 연산을 포함한
// executeWatchProductPrice 전체 흐름을 측정합니다.
func BenchmarkKurlyTask_RunWatchProductPrice(b *testing.B) {
	const productID = 12345
	url := fmt.Sprintf(productPageURLFormat, productID)

	mockFetcher := mocks.NewMockHTTPFetcher()
	mockFetcher.SetResponse(url, []byte(buildBenchmarkHTML(productID, 1000)))

	tTask := buildBenchmarkTask(b, mockFetcher)
	csvPath := buildBenchmarkCSV(b, []int{productID})

	loader := &csvWatchListLoader{filePath: csvPath}
	snapshot := &watchProductPriceSnapshot{Products: make([]*product, 0)}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, _, err := tTask.executeWatchProductPrice(context.Background(), loader, snapshot, false)
		if err != nil {
			b.Fatalf("executeWatchProductPrice 실패: %v", err)
		}
	}
}

// =============================================================================
// BenchmarkMergeWithPreviousState — 규모별 상태 병합 성능
// =============================================================================

// BenchmarkMergeWithPreviousState_Scale 상품 규모별(1/10/100개)로 mergeWithPreviousState 성능을 비교합니다.
func BenchmarkMergeWithPreviousState_Scale(b *testing.B) {
	scales := []int{1, 10, 100}

	for _, n := range scales {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			products := makeBenchmarkProducts(n)
			prevSnap := &watchProductPriceSnapshot{Products: products}
			currProducts := makeBenchmarkProducts(n)

			// 감시 목록 ID set
			watchedIDs := make(map[int]struct{}, n)
			for i := range n {
				watchedIDs[i+1] = struct{}{}
			}

			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				mergeWithPreviousState(currProducts, prevSnap, watchedIDs)
			}
		})
	}
}

// =============================================================================
// BenchmarkExtractProductDiffs — 규모별 Diff 추출 성능
// =============================================================================

// BenchmarkExtractProductDiffs_Scale 상품 규모별 extractProductDiffs 성능을 비교합니다.
// 가격 변동 없음(hot path)과 전체 가격 변동(cold path) 두 시나리오를 측정합니다.
func BenchmarkExtractProductDiffs_Scale(b *testing.B) {
	scales := []int{1, 10, 100}

	for _, n := range scales {
		products := makeBenchmarkProducts(n)

		// 시나리오 A: 변동 없음 (같은 가격)
		b.Run(fmt.Sprintf("NoChange/N=%d", n), func(b *testing.B) {
			snap := &watchProductPriceSnapshot{Products: products}
			prevMap := prevMapFrom(products...)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				extractProductDiffs(snap, prevMap)
			}
		})

		// 시나리오 B: 전체 가격 변동 (최악의 경우)
		b.Run(fmt.Sprintf("AllChanged/N=%d", n), func(b *testing.B) {
			changed := make([]*product, n)
			for i, p := range products {
				cp := *p
				cp.Price = p.Price + 1000 // 모두 가격 인상
				changed[i] = &cp
			}
			snap := &watchProductPriceSnapshot{Products: changed}
			prevMap := prevMapFrom(products...)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				extractProductDiffs(snap, prevMap)
			}
		})
	}
}

// =============================================================================
// BenchmarkRenderProductDiffs — Diff 렌더링 성능
// =============================================================================

// BenchmarkRenderProductDiffs_Scale 규모별, HTML/텍스트 모드별 renderProductDiffs 성능을 비교합니다.
func BenchmarkRenderProductDiffs_Scale(b *testing.B) {
	buildDiffs := func(n int) []productDiff {
		diffs := make([]productDiff, n)
		for i := range n {
			p := &product{
				ID:                 i + 1,
				Name:               fmt.Sprintf("상품_%d", i+1),
				Price:              10000,
				DiscountedPrice:    9000,
				DiscountRate:       10,
				LowestPrice:        8500,
				LowestPriceTimeUTC: time.Now().UTC(),
			}
			prev := &product{ID: i + 1, Price: 11000}
			diffs[i] = productDiff{Type: productEventPriceChanged, Product: p, Prev: prev}
		}
		return diffs
	}

	scales := []int{1, 10, 50}
	for _, n := range scales {
		diffs := buildDiffs(n)

		b.Run(fmt.Sprintf("Text/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				renderProductDiffs(diffs, false)
			}
		})

		b.Run(fmt.Sprintf("HTML/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				renderProductDiffs(diffs, true)
			}
		})
	}
}

// =============================================================================
// BenchmarkWriteFormattedPrice — 가격 포맷팅 성능
// =============================================================================

// BenchmarkWriteFormattedPrice 가격 포맷팅(할인/정가, HTML/텍스트) 성능을 측정합니다.
// 문자열 반복 할당이 핫 경로인지 확인합니다.
func BenchmarkWriteFormattedPrice(b *testing.B) {
	cases := []struct {
		name            string
		price           int
		discountedPrice int
		discountRate    int
		supportsHTML    bool
	}{
		{"정가_텍스트", 10000, 0, 0, false},
		{"할인_텍스트", 10000, 8000, 20, false},
		{"정가_HTML", 10000, 0, 0, true},
		{"할인_HTML", 10000, 8000, 20, true},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				var sb strings.Builder
				writeFormattedPrice(&sb, c.price, c.discountedPrice, c.discountRate, c.supportsHTML)
			}
		})
	}
}

// =============================================================================
// BenchmarkExtractNewDuplicateRecords — 중복 감지 성능
// =============================================================================

// BenchmarkExtractNewDuplicateRecords 중복 레코드 처리 성능을 규모별로 측정합니다.
func BenchmarkExtractNewDuplicateRecords(b *testing.B) {
	buildRecords := func(n int) [][]string {
		records := make([][]string, n)
		for i := range n {
			records[i] = []string{fmt.Sprintf("%d", i+1), fmt.Sprintf("상품_%d", i+1), "1"}
		}
		return records
	}

	scales := []int{1, 10, 100}
	for _, n := range scales {
		records := buildRecords(n)

		b.Run(fmt.Sprintf("AllNew/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				extractNewDuplicateRecords(records, nil)
			}
		})

		// 전부 이미 발송된 상태 (캐시 히트율 100%)
		prevIDs := make([]string, n)
		for i := range n {
			prevIDs[i] = fmt.Sprintf("%d", i+1)
		}
		b.Run(fmt.Sprintf("AllCached/N=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				extractNewDuplicateRecords(records, prevIDs)
			}
		})
	}
}

// =============================================================================
// BenchmarkRenderProductLink — 링크 렌더링 성능 (HTML vs 텍스트)
// =============================================================================

// BenchmarkRenderProductLink HTML/텍스트 모드별 renderProductLink 성능을 비교합니다.
func BenchmarkRenderProductLink(b *testing.B) {
	b.Run("텍스트", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			renderProductLink("12345", "맛있는 사과 & 과일 <신선>", false)
		}
	})

	b.Run("HTML_이스케이프", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			renderProductLink("12345", "맛있는 사과 & 과일 <신선>", true)
		}
	})
}

// =============================================================================
// BenchmarkFormatProductItem — 단일 상품 렌더링 성능
// =============================================================================

// BenchmarkFormatProductItem formatProductItem의 prev 유무 및 HTML/텍스트 조합 성능을 측정합니다.
func BenchmarkFormatProductItem(b *testing.B) {
	now := time.Now().UTC()
	p := &product{
		ID:                 12345,
		Name:               "맛있는 사과",
		Price:              10000,
		DiscountedPrice:    8000,
		DiscountRate:       20,
		LowestPrice:        7500,
		LowestPriceTimeUTC: now,
	}
	prev := &product{
		ID:    12345,
		Name:  "맛있는 사과",
		Price: 11000,
	}

	cases := []struct {
		name         string
		prev         *product
		supportsHTML bool
	}{
		{"Prev없음_텍스트", nil, false},
		{"Prev없음_HTML", nil, true},
		{"Prev있음_텍스트", prev, false},
		{"Prev있음_HTML", prev, true},
	}

	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				formatProductItem(p, c.supportsHTML, mark.Modified, c.prev)
			}
		})
	}
}
