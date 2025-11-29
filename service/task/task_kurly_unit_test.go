package task

import (
	"path/filepath"
	"testing"

	"github.com/darkkaiser/notify-server/g"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKurlyTask_RunWatchProductPrice(t *testing.T) {
	t.Run("정상적인 상품 가격 파싱", func(t *testing.T) {
		mockFetcher := NewMockHTTPFetcher()

		// Mock product page 1
		productPage1 := `
			<html>
				<head>
					<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":{"no":5000001}}}}</script>
				</head>
				<body>
					<div id="product-atf">
						<section class="css-1ua1wyk">
							<div class="css-84rb3h">
								<div class="css-6zfm8o">
									<div class="css-o3fjh7">
										<h1>테스트 상품1</h1>
									</div>
								</div>
							</div>
							<h2 class="css-xrp7wx">
								<div class="css-o2nlqt">
									<span>10,000</span>
									<span>원</span>
								</div>
							</h2>
						</section>
					</div>
				</body>
			</html>`
		mockFetcher.SetResponse("https://www.kurly.com/goods/5000001", []byte(productPage1))

		// Mock product page 2
		productPage2 := `
			<html>
				<head>
					<script id="__NEXT_DATA__" type="application/json">{"props":{"pageProps":{"product":{"no":5000002}}}}</script>
				</head>
				<body>
					<div id="product-atf">
						<section class="css-1ua1wyk">
							<div class="css-84rb3h">
								<div class="css-6zfm8o">
									<div class="css-o3fjh7">
										<h1>테스트 상품2</h1>
									</div>
								</div>
							</div>
							<h2 class="css-xrp7wx">
								<span class="css-8h3us8">20%</span>
								<div class="css-o2nlqt">
									<span>8,000</span>
									<span>원</span>
								</div>
							</h2>
							<span class="css-1s96j0s">
								<span>10,000원</span>
							</span>
						</section>
					</div>
				</body>
			</html>`
		mockFetcher.SetResponse("https://www.kurly.com/goods/5000002", []byte(productPage2))

		// Task setup
		csvPath := filepath.Join("testdata", "kurly", "watch_products_test.csv")
		task := &kurlyTask{
			task: task{
				id:        TidKurly,
				commandID: TcidKurlyWatchProductPrice,
				fetcher:   mockFetcher,
			},
			config: &g.AppConfig{},
		}

		taskCommandData := &kurlyWatchProductPriceTaskCommandData{
			WatchProductsFile: csvPath,
		}

		taskResultData := &kurlyWatchProductPriceResultData{}
		message, changedData, err := task.runWatchProductPrice(taskCommandData, taskResultData, false)

		require.NoError(t, err)
		assert.Contains(t, message, "테스트 상품1", "상품1이 메시지에 포함되어야 합니다")
		assert.Contains(t, message, "테스트 상품2", "상품2가 메시지에 포함되어야 합니다")

		require.NotNil(t, changedData)
		resultData := changedData.(*kurlyWatchProductPriceResultData)
		assert.Equal(t, 2, len(resultData.Products), "2개의 상품이 추출되어야 합니다")
		assert.Equal(t, "테스트 상품1", resultData.Products[0].Name)
		assert.Equal(t, 10000, resultData.Products[0].Price)
		assert.Equal(t, "테스트 상품2", resultData.Products[1].Name)
		assert.Equal(t, 8000, resultData.Products[1].DiscountedPrice)
		assert.Equal(t, 20, resultData.Products[1].DiscountRate)
	})
}
