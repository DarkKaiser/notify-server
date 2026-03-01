package kurly

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/task/fetcher/mocks"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// -------------------------------------------------------------------------
// 1. Mocks & Stubs
// -------------------------------------------------------------------------

// mockWatchListLoader WatchListLoader 인터페이스의 Mock 구현체입니다.
type mockWatchListLoader struct {
	mock.Mock
}

func (m *mockWatchListLoader) Load() ([][]string, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([][]string), args.Error(1)
}

// -------------------------------------------------------------------------
// 2. executeWatchProductPrice Tests
// -------------------------------------------------------------------------

func TestExecuteWatchProductPrice_TableDriven(t *testing.T) {
	t.Parallel()

	// -------------------------------------------------------------------------
	// Fixtures
	// -------------------------------------------------------------------------

	// HTML Mock Data 1: 할인율 없는 상품 (ID: 100)
	htmlNoDiscount := `
		<!DOCTYPE html>
		<html>
		<head>
			<script id="__NEXT_DATA__" type="application/json">
				{"props":{"pageProps":{"product":{"no":100, "name":"싱싱한 사과", "seo":{"og_title":"싱싱한 사과", "og_price":"5000"}}}}}
			</script>
		</head>
		<body>
			<div id="product-atf">
				<section class="css-1ua1wyk">
					<div class="css-84rb3h">
						<div class="css-6zfm8o">
							<div class="css-o3fjh7">
								<h1>싱싱한 사과</h1>
							</div>
						</div>
					</div>
					<h2 class="css-xrp7wx">
						<div class="css-o2nlqt">
							<span>5,000</span>
							<span>원</span>
						</div>
					</h2>
				</section>
			</div>
		</body>
		</html>
	`

	// HTML Mock Data 2: 단종/판매중지 상품 (Next.js 데이터 내 product가 null 인 경우, ID: 200)
	htmlUnavailable := `
		<!DOCTYPE html>
		<html>
		<head>
			<script id="__NEXT_DATA__" type="application/json">
				{"props":{"pageProps":{"product":null}}}
			</script>
		</head>
		<body>
		</body>
		</html>
	`

	// -------------------------------------------------------------------------
	// Table-Driven Tests
	// -------------------------------------------------------------------------

	tests := []struct {
		name         string
		setupMock    func(*mockWatchListLoader, *mocks.MockHTTPFetcher)
		prevSnapshot *watchProductPriceSnapshot
		check        func(*testing.T, string, any, error)
	}{
		{
			name: "성공: 최초 실행 시 모든 상품 수집 후 스냅샷 갱신",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				// Loader 설정: 1개의 유효한 상품을 반환
				mLoader.On("Load").Return([][]string{
					{"100", "싱싱한 사과", "1"},
				}, nil)

				// Fetcher 설정: 정상적인 단일 상품 페이지 반환
				mFetcher.SetResponse("https://www.kurly.com/goods/100", []byte(htmlNoDiscount))
			},
			prevSnapshot: nil,
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.NoError(t, err)

				newSnapshot, ok := snapshot.(*watchProductPriceSnapshot)
				require.True(t, ok)
				require.NotNil(t, newSnapshot)
				assert.Len(t, newSnapshot.Products, 1)

				p := newSnapshot.Products[0]
				assert.Equal(t, 100, p.ID)
				assert.Equal(t, "싱싱한 사과", p.Name)
				assert.Equal(t, 5000, p.Price)
				assert.Equal(t, 5000, p.LowestPrice)
				assert.False(t, p.IsUnavailable)

				assert.Contains(t, msg, "상품 정보가 변경되었습니다")
				assert.Contains(t, msg, "싱싱한 사과")
				assert.Contains(t, msg, "🆕")
			},
		},
		{
			name: "성공: 변경사항 없음 (HasChanged: false)",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				mLoader.On("Load").Return([][]string{
					{"100", "싱싱한 사과", "1"},
				}, nil)

				mFetcher.SetResponse("https://www.kurly.com/goods/100", []byte(htmlNoDiscount))
			},
			prevSnapshot: &watchProductPriceSnapshot{
				Products: []*product{
					{
						ID:                 100,
						Name:               "싱싱한 사과",
						Price:              5000,
						DiscountedPrice:    0,
						DiscountRate:       0,
						LowestPrice:        5000,
						LowestPriceTimeUTC: time.Now().UTC(),
						IsUnavailable:      false,
						FetchFailedCount:   0,
					},
				},
			},
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.NoError(t, err)
				assert.Nil(t, snapshot)
				assert.Empty(t, msg)
			},
		},
		{
			name: "실패: Loader가 에러를 반환하는 경우",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				mLoader.On("Load").Return(nil, errors.New("CSV 파일 읽기 실패"))
			},
			prevSnapshot: nil,
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "CSV 파일 읽기 실패")
				assert.Nil(t, snapshot)
				assert.Empty(t, msg)
			},
		},
		{
			name: "성공: 중복 상품 감지 및 누적 보존 처리 확인",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				mLoader.On("Load").Return([][]string{
					{"100", "싱싱한 사과", "0"},
					{"100", "싱싱한 사과", "1"},
				}, nil)

				mFetcher.SetResponse("https://www.kurly.com/goods/100", []byte(htmlNoDiscount))
			},
			prevSnapshot: nil,
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.NoError(t, err)

				newSnapshot, ok := snapshot.(*watchProductPriceSnapshot)
				require.True(t, ok)

				assert.Contains(t, newSnapshot.DuplicateNotifiedIDs, "100")
				assert.Contains(t, msg, "중복으로 등록된 상품 목록")
			},
		},
		{
			name: "성공: 단종/판매중지 상품 처리 (IsUnavailable=true)",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				mLoader.On("Load").Return([][]string{
					{"200", "단종된 상품", "1"},
				}, nil)

				mFetcher.SetResponse("https://www.kurly.com/goods/200", []byte(htmlUnavailable))
			},
			prevSnapshot: &watchProductPriceSnapshot{
				Products: []*product{
					{
						ID:               200,
						Name:             "단종된 상품",
						Price:            0,
						LowestPrice:      0,
						IsUnavailable:    false,
						FetchFailedCount: 0,
					},
				},
			},
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.NoError(t, err)

				newSnapshot, ok := snapshot.(*watchProductPriceSnapshot)
				require.True(t, ok)
				assert.Len(t, newSnapshot.Products, 1)

				p := newSnapshot.Products[0]
				assert.True(t, p.IsUnavailable)

				assert.Contains(t, msg, "알 수 없는 상품 목록")
			},
		},
		{
			name: "성공: 일시적 파싱 누락(임시 실패) 시 FetchFailedCount 누적(1회)",
			setupMock: func(mLoader *mockWatchListLoader, mFetcher *mocks.MockHTTPFetcher) {
				mLoader.On("Load").Return([][]string{
					{"100", "싱싱한 사과", "1"},
				}, nil)

				invalidSectionHTML := `
					<!DOCTYPE html>
					<html>
					<head>
						<script id="__NEXT_DATA__" type="application/json">
							{"props":{"pageProps":{"product":{"no":100, "name":"싱싱한 사과", "seo":{"og_title":"싱싱한 사과", "og_price":"5000"}}}}}
						</script>
					</head>
					<body>
						<!-- product-atf 삭제됨 -->
					</body>
					</html>
				`
				mFetcher.SetResponse("https://www.kurly.com/goods/100", []byte(invalidSectionHTML))
			},
			prevSnapshot: &watchProductPriceSnapshot{
				Products: []*product{
					{
						ID:               100,
						Name:             "싱싱한 사과",
						Price:            5000,
						LowestPrice:      5000,
						IsUnavailable:    false,
						FetchFailedCount: 0,
					},
				},
			},
			check: func(t *testing.T, msg string, snapshot any, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "상품 정보 섹션(#product-atf > section.css-1ua1wyk)을 찾을 수 없습니다")
			},
		},
	}

	for _, tt := range tests {
		tt := tt // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Mock 초기화
			mockLoader := new(mockWatchListLoader)
			mockFetcher := mocks.NewMockHTTPFetcher()
			tt.setupMock(mockLoader, mockFetcher)

			// Task 객체 초기화
			testTask := &task{
				Base: provider.NewBase(provider.NewTaskParams{
					Request: &contract.TaskSubmitRequest{
						TaskID:    TaskID,
						CommandID: WatchProductPriceCommand,
						RunBy:     contract.TaskRunByScheduler,
					},
					Fetcher: mockFetcher, // 원래 Execute 계층에는 Fetcher가 주입되고, 내부에서 scraper.New()로 래핑됨.
				}, true),
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			msg, snapshot, err := testTask.executeWatchProductPrice(ctx, mockLoader, tt.prevSnapshot, true)

			tt.check(t, msg, snapshot, err)
			mockLoader.AssertExpectations(t)
			// MockHTTPFetcher는 AssertExpectations 메서드가 없으므로 생략
		})
	}
}
