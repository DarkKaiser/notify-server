package kurly

import (
	"fmt"
	"os"
	"testing"

	"github.com/darkkaiser/notify-server/service/task"
	"github.com/darkkaiser/notify-server/service/task/testutil"
)

func BenchmarkKurlyTask_RunWatchProductPrice(b *testing.B) {
	// 1. Setup Mock Fetcher with a realistic HTML response
	mockFetcher := testutil.NewMockHTTPFetcher()
	productID := "12345"
	url := fmt.Sprintf("%sgoods/%s", kurlyBaseURL, productID)

	// Create a reasonably large HTML content to simulate real parsing load
	htmlContent := fmt.Sprintf(`
		<html>
		<body>
			<script id="__NEXT_DATA__">
				{"props":{"pageProps":{"product":{"no":%s}}}}
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
					<!-- Adding some filler content to increase parsing complexity -->
					<div class="filler">
						%s
					</div>
				</section>
			</div>
		</body>
		</html>
	`, productID, generateFillerHTML(1000)) // 1000 lines of filler

	mockFetcher.SetResponse(url, []byte(htmlContent))

	// 2. Setup Task
	tTask := &kurlyTask{
		Task: task.NewBaseTask(ID, WatchProductPriceCommand, "test_instance", "test-notifier", task.RunByUnknown),
	}
	tTask.SetFetcher(mockFetcher)

	// 3. Setup Command Data
	// We use a temporary file for the CSV, created once
	csvContent := fmt.Sprintf("No,Name,Status\n%s,Test Product,1\n", productID)
	// Note: In a real benchmark, file I/O might be a bottleneck, but here we want to measure the whole flow including CSV reading as it's part of the task.
	// However, creating a file in every loop is bad. We should create it once.
	// But `runWatchProductPrice` opens the file every time.
	// To strictly benchmark parsing, we might want to avoid file I/O, but `runWatchProductPrice` takes a filename.
	// We will accept file I/O overhead as part of the "Task Run" benchmark.

	// Create a temporary file that persists during the benchmark
	tmpfile, err := os.CreateTemp("", "benchmark_products_*.csv")
	if err != nil {
		b.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up
	if _, err := tmpfile.Write([]byte(csvContent)); err != nil {
		b.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		b.Fatal(err)
	}

	commandData := &kurlyWatchProductPriceCommandData{
		WatchProductsFile: tmpfile.Name(),
	}

	resultData := &kurlyWatchProductPriceResultData{
		Products: make([]*kurlyProduct, 0),
	}

	b.ResetTimer() // Reset timer to ignore setup time

	for i := 0; i < b.N; i++ {
		_, _, err := tTask.executeWatchProductPrice(commandData, resultData, true)
		if err != nil {
			b.Fatalf("Task run failed: %v", err)
		}
	}
}

func generateFillerHTML(lines int) string {
	var s string
	for i := 0; i < lines; i++ {
		s += fmt.Sprintf("<div>Filler content line %d</div>\n", i)
	}
	return s
}
