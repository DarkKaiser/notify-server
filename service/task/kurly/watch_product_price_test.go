package kurly

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	tasksvc "github.com/darkkaiser/notify-server/service/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//
// Mock Objects
//

// MockFetcher는 http.Fetcher 인터페이스를 모킹합니다.
type MockFetcher struct {
	mock.Mock
}

func (m *MockFetcher) Get(url string) (*http.Response, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *MockFetcher) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// Helper to create a response with body
func createMockResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

//
// Tests
//

func TestWatchProductPriceSettings_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		settings  *watchProductPriceSettings
		wantErr   bool
		errSubstr string
	}{
		{
			name: "성공: 정상적인 CSV 파일 경로",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "products.csv",
			},
			wantErr: false,
		},
		{
			name: "성공: 대소문자 구분 없이 CSV 확장자 허용",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "PRODUCTS.CSV",
			},
			wantErr: false,
		},
		{
			name: "실패: 파일 경로 미입력",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "",
			},
			wantErr:   true,
			errSubstr: "watch_products_file이 입력되지 않았거나 공백입니다",
		},
		{
			name: "실패: 지원하지 않는 파일 확장자 (.txt)",
			settings: &watchProductPriceSettings{
				WatchProductsFile: "products.txt",
			},
			wantErr:   true,
			errSubstr: ".csv 확장자를 가진 파일 경로만 지정할 수 있습니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.settings.validate()
			if tt.wantErr {
				require.Error(t, err)
				if tt.errSubstr != "" {
					assert.Contains(t, err.Error(), tt.errSubstr)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractDuplicateRecords(t *testing.T) {
	t.Parallel()
	tsk := &task{}

	tests := []struct {
		name          string
		input         [][]string
		wantDistinct  int
		wantDuplicate int
	}{
		{
			name: "중복 없음",
			input: [][]string{
				{"1001", "A", "1"},
				{"1002", "B", "1"},
			},
			wantDistinct:  2,
			wantDuplicate: 0,
		},
		{
			name: "단일 중복 발생",
			input: [][]string{
				{"1001", "A", "1"},
				{"1001", "A", "1"},
			},
			wantDistinct:  1,
			wantDuplicate: 1,
		},
		{
			name: "다수 중복 발생",
			input: [][]string{
				{"1001", "A", "1"},
				{"1002", "B", "1"},
				{"1001", "A", "1"},
				{"1002", "B", "1"},
				{"1003", "C", "1"},
			},
			wantDistinct:  3,
			wantDuplicate: 2,
		},
		{
			name: "빈 행 무시",
			input: [][]string{
				{"1001", "A", "1"},
				{},
				{"1002", "B", "1"},
			},
			wantDistinct:  2,
			wantDuplicate: 0,
		},
		{
			name:          "빈 입력",
			input:         [][]string{},
			wantDistinct:  0,
			wantDuplicate: 0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			distinct, duplicate := tsk.extractDuplicateRecords(tt.input)
			assert.Equal(t, tt.wantDistinct, len(distinct))
			assert.Equal(t, tt.wantDuplicate, len(duplicate))
		})
	}
}

func createDoc(html string) *goquery.Document {
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	return doc
}

func TestTask_ParseProductFromPage(t *testing.T) {
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
			mockFetcher := new(MockFetcher)
			url := fmt.Sprintf(productPageURLFormat, tt.productID)

			if tt.mockFetchErr != nil {
				mockFetcher.On("Get", url).Return(nil, tt.mockFetchErr)
			} else {
				mockFetcher.On("Get", url).Return(createMockResponse(tt.mockStatusCode, tt.mockHTML), nil)
			}

			tsk := &task{}
			tsk.SetFetcher(mockFetcher)

			got, err := tsk.fetchProductInfo(tt.productID)

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

func TestTask_DiffAndNotify(t *testing.T) {
	t.Parallel()
	tsk := &task{}

	newProduct := func(id, price int) *product {
		p := &product{ID: id, Name: "Test", Price: price}
		p.updateLowestPrice()
		return p
	}

	tests := []struct {
		name            string
		current         []*product
		prev            []*product
		runBy           tasksvc.RunBy
		wantMsgContent  []string
		wantDataChanged bool
	}{
		{
			name:            "변경 없음 (Scheduler)",
			current:         []*product{newProduct(1, 1000)},
			prev:            []*product{newProduct(1, 1000)},
			runBy:           tasksvc.RunByScheduler,
			wantMsgContent:  nil,
			wantDataChanged: false,
		},
		{
			name:            "변경 없음 (User) - 메시지는 생성되지만 데이터 갱신 없음",
			current:         []*product{newProduct(1, 1000)},
			prev:            []*product{newProduct(1, 1000)},
			runBy:           tasksvc.RunByUser,
			wantMsgContent:  []string{"변경된 상품 정보가 없습니다", "현재 등록된 상품 정보는 아래와 같습니다"},
			wantDataChanged: false,
		},
		{
			name:    "가격 변경 발생",
			current: []*product{newProduct(1, 800)},
			prev:    []*product{newProduct(1, 1000)},
			runBy:   tasksvc.RunByScheduler,
			wantMsgContent: []string{
				"상품 정보가 변경되었습니다",
				"이전 가격", "1,000원",
				"현재 가격", "800원",
			},
			wantDataChanged: true,
		},
		{
			name:            "신규 상품 추가",
			current:         []*product{newProduct(1, 1000), newProduct(2, 2000)},
			prev:            []*product{newProduct(1, 1000)},
			runBy:           tasksvc.RunByScheduler,
			wantMsgContent:  []string{"상품 정보가 변경되었습니다", "🆕", "2,000원"},
			wantDataChanged: true,
		},
		{
			name: "판매 중지 (Unavailable)",
			current: func() []*product {
				p := newProduct(1, 1000)
				p.IsUnavailable = true
				return []*product{p}
			}(),
			prev:            []*product{newProduct(1, 1000)},
			runBy:           tasksvc.RunByScheduler,
			wantMsgContent:  nil,
			wantDataChanged: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tsk.SetRunBy(tt.runBy)

			curSnap := &watchProductPriceSnapshot{Products: tt.current}
			prevSnap := &watchProductPriceSnapshot{Products: tt.prev}

			var prevProductsMap map[int]*product
			if prevSnap != nil {
				prevProductsMap = make(map[int]*product, len(prevSnap.Products))
				for _, p := range prevSnap.Products {
					prevProductsMap[p.ID] = p
				}
			}

			msg, shouldSave := tsk.analyzeAndReport(curSnap, prevProductsMap, nil, nil, false)

			if len(tt.wantMsgContent) > 0 {
				assert.NotEmpty(t, msg)
				for _, part := range tt.wantMsgContent {
					assert.Contains(t, msg, part)
				}
			} else {
				assert.Empty(t, msg)
			}

			assert.Equal(t, tt.wantDataChanged, shouldSave, "데이터 저장 필요 여부(shouldSave)가 기대값과 다릅니다")
		})
	}
}

// TestRenderProductLink HTML/Text 모드에 따른 링크 생성 및 이스케이프 동작을 검증합니다.
func TestRenderProductLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		productID    string
		productName  string
		supportsHTML bool
		want         string
	}{
		{
			name:         "Text Mode: Special Characters (Should NOT Escape)",
			productID:    "123",
			productName:  "Bread & Butter <New>",
			supportsHTML: false,
			want:         "Bread & Butter <New>(123)",
		},
		{
			name:         "HTML Mode: Special Characters (Should Escape)",
			productID:    "456",
			productName:  "Bread & Butter <New>",
			supportsHTML: true,
			want:         `<a href="https://www.kurly.com/goods/456"><b>Bread &amp; Butter &lt;New&gt;</b></a>`,
		},
		{
			name:         "Text Mode: Normal",
			productID:    "789",
			productName:  "Fresh Apple",
			supportsHTML: false,
			want:         "Fresh Apple(789)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := renderProductLink(tt.productID, tt.productName, tt.supportsHTML)
			assert.Equal(t, tt.want, got)
		})
	}
}
