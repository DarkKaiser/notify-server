package kurly

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestTask_FetchProductInfo(t *testing.T) {
	// Fixture HTML Templates
	tmplNormal := `
<html>
<body>
<script id="__NEXT_DATA__">{"product": {"no": %d}}</script>
<div id="product-atf">
	<section class="css-1ua1wyk">
		<div class="css-84rb3h"><div class="css-6zfm8o"><div class="css-o3fjh7"><h1>%s</h1></div></div></div>
		<h2 class="css-xrp7wx">%s</h2>
	</section>
</div>
</body>
</html>`

	tests := []struct {
		name           string
		productID      int
		mockHTML       string
		mockFetchErr   error
		mockStatusCode int
		wantProduct    *product
		wantErr        bool
		errSubstr      string
	}{
		{
			name:           "성공: 정상 상품 파싱 (할인 없음)",
			productID:      123,
			mockStatusCode: 200,
			mockHTML: fmt.Sprintf(tmplNormal, 123, "맛있는 사과",
				`<div class="css-o2nlqt"><span>10,000</span><span>원</span></div>`),
			wantProduct: &product{
				ID:    123,
				Name:  "맛있는 사과",
				Price: 10000,
			},
			wantErr: false,
		},
		{
			name:           "성공: 정상 상품 파싱 (할인 중)",
			productID:      456,
			mockStatusCode: 200,
			mockHTML: fmt.Sprintf(tmplNormal, 456, "할인 바나나",
				`<span class="css-8h3us8">10%</span><div class="css-o2nlqt"><span>9,000</span><span>원</span></div><span class="css-1s96j0s"><span>10,000원</span></span>`),
			wantProduct: &product{
				ID:              456,
				Name:            "할인 바나나",
				Price:           10000,
				DiscountedPrice: 9000,
				DiscountRate:    10,
			},
			wantErr: false,
		},
		{
			name:           "실패: Fetch 에러",
			productID:      999,
			mockFetchErr:   errors.New("network timeout"),
			mockStatusCode: 0,
			wantErr:        true,
			errSubstr:      "network timeout",
		},
		{
			name:           "실패: HTML 파싱 실패 (__NEXT_DATA__ 없음)",
			productID:      100,
			mockStatusCode: 200,
			mockHTML:       "<html><body>Nothing here</body></html>",
			wantErr:        true,
			errSubstr:      "JSON 데이터 추출이 실패하였습니다",
		},
		{
			name:           "성공: 판매 중지 상품 (IsUnavailable)",
			productID:      101,
			mockStatusCode: 200,
			mockHTML:       `<html><body><script id="__NEXT_DATA__">{"product": null}</script></body></html>`,
			wantProduct: &product{
				ID:            101,
				IsUnavailable: true,
			},
			wantErr: false,
		},
		{
			name:           "실패: CSS 구조 변경됨 (섹션 없음)",
			productID:      102,
			mockStatusCode: 200,
			mockHTML:       `<html><body><script id="__NEXT_DATA__">{"product": {}}</script><div>Changed Layout</div></body></html>`,
			wantErr:        true,
			errSubstr:      "상품정보 섹션 추출 실패",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := new(mocks.MockFetcher)
			url := formatProductPageURL(tt.productID)

			if tt.mockFetchErr != nil {
				mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == url
				})).Return(nil, tt.mockFetchErr)
			} else {
				mockFetcher.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					return req.Method == http.MethodGet && req.URL.String() == url
				})).Return(mocks.NewMockResponse(tt.mockHTML, tt.mockStatusCode), nil)
			}

			tsk := &task{
				Base: provider.NewBase(provider.NewTaskParams{
					Request: &contract.TaskSubmitRequest{
						TaskID:     "T",
						CommandID:  "C",
						NotifierID: "N",
						RunBy:      contract.TaskRunByScheduler,
					},
					InstanceID: "I",
					Fetcher:    mockFetcher,
					NewSnapshot: func() interface{} {
						return &watchProductPriceSnapshot{}
					},
				}, true),
			}

			got, err := tsk.fetchProductInfo(context.Background(), tt.productID)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantProduct.ID, got.ID)
				assert.Equal(t, tt.wantProduct.IsUnavailable, got.IsUnavailable)
				if !got.IsUnavailable {
					assert.Equal(t, tt.wantProduct.Name, got.Name)
					assert.Equal(t, tt.wantProduct.Price, got.Price)
					assert.Equal(t, tt.wantProduct.DiscountedPrice, got.DiscountedPrice)
					assert.Equal(t, tt.wantProduct.DiscountRate, got.DiscountRate)
				}
			}
			mockFetcher.AssertExpectations(t)
		})
	}
}
