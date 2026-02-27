package kurly

import (
	"testing"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
)

func TestTask_DiffAndNotify(t *testing.T) {
	t.Parallel()

	newProduct := func(id, price int) *product {
		p := &product{ID: id, Name: "Test", Price: price}
		p.tryUpdateLowestPrice()
		return p
	}

	tests := []struct {
		name            string
		current         []*product
		prev            []*product
		runBy           contract.TaskRunBy
		wantMsgContent  []string
		wantDataChanged bool
	}{
		{
			name:            "변경 없음 (Scheduler)",
			current:         []*product{newProduct(1, 1000)},
			prev:            []*product{newProduct(1, 1000)},
			runBy:           contract.TaskRunByScheduler,
			wantMsgContent:  nil,
			wantDataChanged: false,
		},
		{
			name:            "변경 없음 (User) - 메시지는 생성되지만 데이터 갱신 없음",
			current:         []*product{newProduct(1, 1000)},
			prev:            []*product{newProduct(1, 1000)},
			runBy:           contract.TaskRunByUser,
			wantMsgContent:  []string{"변경된 상품 정보가 없습니다", "현재 등록된 상품 정보는 아래와 같습니다"},
			wantDataChanged: false,
		},
		{
			name:    "가격 변경 발생",
			current: []*product{newProduct(1, 800)},
			prev:    []*product{newProduct(1, 1000)},
			runBy:   contract.TaskRunByScheduler,
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
			runBy:           contract.TaskRunByScheduler,
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
			runBy:           contract.TaskRunByScheduler,
			wantMsgContent:  nil,
			wantDataChanged: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			curSnap := &watchProductPriceSnapshot{Products: tt.current}
			prevSnap := &watchProductPriceSnapshot{Products: tt.prev}

			var prevProductsMap map[int]*product
			if prevSnap != nil {
				prevProductsMap = make(map[int]*product, len(prevSnap.Products))
				for _, p := range prevSnap.Products {
					prevProductsMap[p.ID] = p
				}
			}

			diffs := extractProductDiffs(curSnap, prevProductsMap)
			msg := buildNotificationMessage(tt.runBy, curSnap, renderProductDiffs(diffs, false), "", "", false)
			shouldSave := curSnap.HasChanged(prevSnap)

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

func TestTask_SyncProductState(t *testing.T) {
	t.Parallel()

	// Given
	// 과거 스냅샷에는 1번(정상), 2번(과거) 상품 존재
	prevSnap := &watchProductPriceSnapshot{
		Products: []*product{
			{ID: 1, Name: "Product1", Price: 1000, LowestPrice: 900},
			{ID: 2, Name: "Product2", Price: 2000, LowestPrice: 1500},
		},
	}

	// 현재 스냅샷에는 1번 상품만 수집됨 (2번은 통신 오류나 비활성화로 누락)
	curSnap := &watchProductPriceSnapshot{
		Products: []*product{
			{ID: 1, Name: "Product1", Price: 950},
		},
	}

	// Given (활성 상태인 상품 ID 모의 설정: 2번 상품은 통신 누락 시나리오이므로 활성 상태여야 함)
	activeRecordIDs := map[int]struct{}{
		1: {},
		2: {},
	}

	// When
	// 1. syncProductState 실행
	updatedProducts, _ := mergeWithPreviousState(curSnap.Products, prevSnap, activeRecordIDs)
	curSnap.Products = updatedProducts

	// Then
	// 1번 상품은 최저가가 유지/갱신되어야 하고,
	// 누락되었던 2번 상품은 과거 데이터(히스토리)가 유지된 채로 현재 스냅샷에 추가되어야 합니다.
	assert.Equal(t, 2, len(curSnap.Products))

	var p1, p2 *product
	for _, p := range curSnap.Products {
		if p.ID == 1 {
			p1 = p
		} else if p.ID == 2 {
			p2 = p
		}
	}

	assert.NotNil(t, p1)
	assert.Equal(t, 900, p1.LowestPrice)

	assert.NotNil(t, p2)
	assert.Equal(t, 1500, p2.LowestPrice)
	assert.Equal(t, 2000, p2.Price)
}
