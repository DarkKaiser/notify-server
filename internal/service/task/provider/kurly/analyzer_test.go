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
		p.updateLowestPrice()
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
			wantDataChanged: false,
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

			msg, shouldSave := analyzeAndReport(tt.runBy, curSnap, prevProductsMap, nil, nil, false)

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
