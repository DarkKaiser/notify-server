package kurly

import (
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// renderProductLink 테스트
// =============================================================================

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
			name:         "Text Mode: 일반 텍스트 — 이스케이프 없이 그대로 출력",
			productID:    "789",
			productName:  "Fresh Apple",
			supportsHTML: false,
			want:         "Fresh Apple(789)",
		},
		{
			name:         "Text Mode: 특수문자 — 이스케이프 없이 그대로 출력",
			productID:    "123",
			productName:  "Bread & Butter <New>",
			supportsHTML: false,
			want:         "Bread & Butter <New>(123)",
		},
		{
			name:         "HTML Mode: 특수문자 — HTML 이스케이프 적용",
			productID:    "456",
			productName:  "Bread & Butter <New>",
			supportsHTML: true,
			want:         `<a href="https://www.kurly.com/goods/456"><b>Bread &amp; Butter &lt;New&gt;</b></a>`,
		},
		{
			name:         "HTML Mode: 정상 이름 — 볼드+링크 포맷",
			productID:    "999",
			productName:  "맛있는 사과",
			supportsHTML: true,
			want:         `<a href="https://www.kurly.com/goods/999"><b>맛있는 사과</b></a>`,
		},
		{
			name:         "HTML Mode: XSS 스크립트 태그 — 이스케이프 처리",
			productID:    "111",
			productName:  "<script>alert(1)</script>",
			supportsHTML: true,
			want:         `<a href="https://www.kurly.com/goods/111"><b>&lt;script&gt;alert(1)&lt;/script&gt;</b></a>`,
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

// =============================================================================
// writeFormattedPrice 테스트
// =============================================================================

// TestWriteFormattedPrice 할인 유효성 방어 조건과 HTML/텍스트 포맷 분기를 검증합니다.
func TestWriteFormattedPrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		price           int
		discountedPrice int
		discountRate    int
		supportsHTML    bool
		want            string
	}{
		// ── 정가 단독 출력 (방어 조건) ─────────────────────────────────────────
		{
			name:            "정가 판매 (discountedPrice=0) → 정가만 출력",
			price:           10000,
			discountedPrice: 0,
			discountRate:    0,
			supportsHTML:    false,
			want:            "10,000원",
		},
		{
			name:            "할인가 == 정가 → 실질 할인 없음, 정가만 출력",
			price:           10000,
			discountedPrice: 10000,
			discountRate:    0,
			supportsHTML:    false,
			want:            "10,000원",
		},
		{
			name:            "할인가 > 정가 (데이터 오류) → 정가만 출력",
			price:           9000,
			discountedPrice: 10000,
			discountRate:    0,
			supportsHTML:    false,
			want:            "9,000원",
		},
		{
			name:            "할인가 음수 → 정가만 출력",
			price:           10000,
			discountedPrice: -100,
			discountRate:    0,
			supportsHTML:    false,
			want:            "10,000원",
		},
		// ── 정상 할인 — 텍스트 모드 ───────────────────────────────────────────
		{
			name:            "텍스트 모드 — 할인율 있음 → '원 ⇒ 원 (%)' 형식",
			price:           20000,
			discountedPrice: 15000,
			discountRate:    25,
			supportsHTML:    false,
			want:            "20,000원 ⇒ 15,000원 (25%)",
		},
		{
			name:            "텍스트 모드 — 할인율 0% → 할인율 미표시",
			price:           10000,
			discountedPrice: 9900,
			discountRate:    0,
			supportsHTML:    false,
			want:            "10,000원 ⇒ 9,900원",
		},
		// ── 정상 할인 — HTML 모드 ─────────────────────────────────────────────
		{
			name:            "HTML 모드 — 할인율 있음 → '<s>원</s> 원 (%)' 취소선 형식",
			price:           20000,
			discountedPrice: 15000,
			discountRate:    25,
			supportsHTML:    true,
			want:            "<s>20,000원</s> 15,000원 (25%)",
		},
		{
			name:            "HTML 모드 — 할인율 0% → 취소선만, 할인율 미표시",
			price:           10000,
			discountedPrice: 9000,
			discountRate:    0,
			supportsHTML:    true,
			want:            "<s>10,000원</s> 9,000원",
		},
		// ── 숫자 포맷 ─────────────────────────────────────────────────────────
		{
			name:            "천 단위 쉼표 포맷 확인 (1,000,000원)",
			price:           1000000,
			discountedPrice: 0,
			discountRate:    0,
			supportsHTML:    false,
			want:            "1,000,000원",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sb strings.Builder
			writeFormattedPrice(&sb, tt.price, tt.discountedPrice, tt.discountRate, tt.supportsHTML)
			assert.Equal(t, tt.want, sb.String())
		})
	}
}

// =============================================================================
// renderProductDiffs 테스트
// =============================================================================

// TestRenderProductDiffs 변동 상품 목록 전체를 하나의 메시지로 렌더링하는 로직을 검증합니다.
func TestRenderProductDiffs(t *testing.T) {
	t.Parallel()

	baseProduct := &product{ID: 100, Name: "사과", Price: 5000}
	prevProduct := &product{ID: 100, Name: "사과", Price: 6000}
	lowestTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("빈 diffs → 빈 문자열 반환", func(t *testing.T) {
		t.Parallel()
		got := renderProductDiffs(nil, false)
		assert.Equal(t, "", got)
	})

	t.Run("빈 슬라이스 → 빈 문자열 반환", func(t *testing.T) {
		t.Parallel()
		got := renderProductDiffs([]productDiff{}, false)
		assert.Equal(t, "", got)
	})

	t.Run("productEventNew → mark.New 이모지 포함, 상품명 출력", func(t *testing.T) {
		t.Parallel()
		diffs := []productDiff{
			{Type: productEventNew, Product: baseProduct, Prev: nil},
		}
		got := renderProductDiffs(diffs, false)
		assert.Contains(t, got, "사과")
		assert.Contains(t, got, string(mark.New))
	})

	t.Run("productEventReappeared → mark.New 이모지 포함 (재입고)", func(t *testing.T) {
		t.Parallel()
		diffs := []productDiff{
			{Type: productEventReappeared, Product: baseProduct, Prev: nil},
		}
		got := renderProductDiffs(diffs, false)
		assert.Contains(t, got, string(mark.New))
	})

	t.Run("productEventPriceChanged → mark.Modified, 이전 가격 포함", func(t *testing.T) {
		t.Parallel()
		diffs := []productDiff{
			{Type: productEventPriceChanged, Product: baseProduct, Prev: prevProduct},
		}
		got := renderProductDiffs(diffs, false)
		assert.Contains(t, got, string(mark.Modified))
		assert.Contains(t, got, "이전 가격")
		assert.Contains(t, got, "6,000원")
	})

	t.Run("productEventLowestPriceAchieved → mark.BestPrice, 이전 가격 포함", func(t *testing.T) {
		t.Parallel()
		p := &product{ID: 100, Name: "사과", Price: 4000, LowestPrice: 4000, LowestPriceTimeUTC: lowestTime}
		diffs := []productDiff{
			{Type: productEventLowestPriceAchieved, Product: p, Prev: prevProduct},
		}
		got := renderProductDiffs(diffs, false)
		assert.Contains(t, got, string(mark.BestPrice))
		assert.Contains(t, got, "이전 가격")
	})

	t.Run("productEventNone → 렌더링 생략 (separator 없음)", func(t *testing.T) {
		t.Parallel()
		diffs := []productDiff{
			{Type: productEventNone, Product: baseProduct},
		}
		got := renderProductDiffs(diffs, false)
		// productEventNone은 switch에서 처리되지 않으므로 빈 문자열
		assert.Equal(t, "", got)
	})

	t.Run("다중 diff → 항목 사이 빈 줄(\\n\\n) 구분자 삽입", func(t *testing.T) {
		t.Parallel()
		p2 := &product{ID: 200, Name: "바나나", Price: 3000}
		diffs := []productDiff{
			{Type: productEventNew, Product: baseProduct},
			{Type: productEventNew, Product: p2},
		}
		got := renderProductDiffs(diffs, false)
		assert.Contains(t, got, "사과")
		assert.Contains(t, got, "바나나")
		assert.Contains(t, got, "\n\n", "항목 사이에 빈 줄 구분자가 있어야 합니다")
	})

	t.Run("HTML 모드 — 상품명이 링크 태그로 감싸짐", func(t *testing.T) {
		t.Parallel()
		diffs := []productDiff{
			{Type: productEventNew, Product: &product{ID: 101, Name: "배", Price: 4000}},
		}
		got := renderProductDiffs(diffs, true)
		assert.Contains(t, got, `href="https://www.kurly.com/goods/101"`)
		assert.Contains(t, got, "<b>배</b>")
	})
}

// =============================================================================
// renderDuplicateRecords 테스트
// =============================================================================

// TestRenderDuplicateRecords 중복 등록 레코드 렌더링을 검증합니다.
func TestRenderDuplicateRecords(t *testing.T) {
	t.Parallel()

	// columnID=0, columnName=1 기준 (watch_list_loader.go 상수)
	makeRecord := func(id, name string) []string {
		return []string{id, name, "1"}
	}

	t.Run("빈 입력 → 빈 문자열 반환", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords(nil, false)
		assert.Equal(t, "", got)
	})

	t.Run("단일 레코드 — 텍스트 모드", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords([][]string{makeRecord("123", "사과")}, false)
		assert.Contains(t, got, "사과(123)")
		assert.Contains(t, got, "• ")
	})

	t.Run("단일 레코드 — HTML 모드, 볼드+링크", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords([][]string{makeRecord("456", "바나나")}, true)
		assert.Contains(t, got, `href="https://www.kurly.com/goods/456"`)
		assert.Contains(t, got, "<b>바나나</b>")
	})

	t.Run("상품명 비어있으면 fallbackProductName 대체", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords([][]string{makeRecord("789", "")}, false)
		assert.Contains(t, got, fallbackProductName)
	})

	t.Run("상품명 공백만 있으면 fallbackProductName 대체", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords([][]string{makeRecord("789", "   ")}, false)
		assert.Contains(t, got, fallbackProductName)
	})

	t.Run("다중 레코드 — 항목 사이 \\n 구분자", func(t *testing.T) {
		t.Parallel()
		records := [][]string{
			makeRecord("100", "사과"),
			makeRecord("200", "바나나"),
			makeRecord("300", "포도"),
		}
		got := renderDuplicateRecords(records, false)
		assert.Contains(t, got, "사과(100)")
		assert.Contains(t, got, "바나나(200)")
		assert.Contains(t, got, "포도(300)")
		// 항목 사이 단일 \n 구분자 (이중 \n\n이 아님)
		lines := strings.Split(got, "\n")
		assert.Equal(t, 3, len(lines), "3개 레코드는 3줄이어야 합니다")
	})

	t.Run("ID 앞뒤 공백 → TrimSpace 후 사용", func(t *testing.T) {
		t.Parallel()
		got := renderDuplicateRecords([][]string{{"  123  ", "사과", "1"}}, false)
		assert.Contains(t, got, "사과(123)")
		assert.NotContains(t, got, "  123  ")
	})
}

// =============================================================================
// renderUnavailableProducts 테스트
// =============================================================================

// TestRenderUnavailableProducts 단종/판매 불가 상품 렌더링을 검증합니다.
func TestRenderUnavailableProducts(t *testing.T) {
	t.Parallel()

	makeItem := func(id, name string) struct{ ID, Name string } {
		return struct{ ID, Name string }{ID: id, Name: name}
	}

	t.Run("빈 입력 → 빈 문자열 반환", func(t *testing.T) {
		t.Parallel()
		got := renderUnavailableProducts(nil, false)
		assert.Equal(t, "", got)
	})

	t.Run("단일 상품 — 텍스트 모드", func(t *testing.T) {
		t.Parallel()
		items := []struct{ ID, Name string }{makeItem("111", "단종 사과")}
		got := renderUnavailableProducts(items, false)
		assert.Contains(t, got, "단종 사과(111)")
		assert.Contains(t, got, "• ")
	})

	t.Run("단일 상품 — HTML 모드, 볼드+링크", func(t *testing.T) {
		t.Parallel()
		items := []struct{ ID, Name string }{makeItem("222", "단종 바나나")}
		got := renderUnavailableProducts(items, true)
		assert.Contains(t, got, `href="https://www.kurly.com/goods/222"`)
		assert.Contains(t, got, "<b>단종 바나나</b>")
	})

	t.Run("다중 상품 — 항목 사이 \\n 구분자", func(t *testing.T) {
		t.Parallel()
		items := []struct{ ID, Name string }{
			makeItem("100", "상품A"),
			makeItem("200", "상품B"),
		}
		got := renderUnavailableProducts(items, false)
		assert.Contains(t, got, "상품A(100)")
		assert.Contains(t, got, "상품B(200)")
		lines := strings.Split(got, "\n")
		assert.Equal(t, 2, len(lines))
	})

	t.Run("HTML XSS 방어 — 상품명 이스케이프", func(t *testing.T) {
		t.Parallel()
		items := []struct{ ID, Name string }{makeItem("333", "<b>주입</b>")}
		got := renderUnavailableProducts(items, true)
		assert.NotContains(t, got, "<b>주입</b>")
		assert.Contains(t, got, "&lt;b&gt;주입&lt;/b&gt;")
	})
}

// =============================================================================
// buildNotificationMessage 테스트
// =============================================================================

// TestBuildNotificationMessage 최종 알림 메시지 조립 로직을 검증합니다.
func TestBuildNotificationMessage(t *testing.T) {
	t.Parallel()

	makeSnapshotWith := func(products ...*product) *watchProductPriceSnapshot {
		return &watchProductPriceSnapshot{Products: products}
	}
	emptySnapshot := &watchProductPriceSnapshot{Products: []*product{}}
	richSnapshot := makeSnapshotWith(
		&product{ID: 100, Name: "사과", Price: 5000},
		&product{ID: 200, Name: "바나나", Price: 3000},
	)

	// ── 이벤트 메시지가 있는 경우 ──────────────────────────────────────────────

	t.Run("가격 변동 메시지 있음 → 헤더 포함 조합 메시지 반환", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByScheduler, richSnapshot,
			"가격 변동 내용", "", "", false,
		)
		assert.Contains(t, got, "상품 정보가 변경되었습니다.")
		assert.Contains(t, got, "가격 변동 내용")
	})

	t.Run("중복 등록 메시지 있음 → 헤더 포함 조합 메시지 반환", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByScheduler, richSnapshot,
			"", "중복 상품 내용", "", false,
		)
		assert.Contains(t, got, "중복으로 등록된 상품 목록:")
		assert.Contains(t, got, "중복 상품 내용")
		assert.NotContains(t, got, "상품 정보가 변경되었습니다.")
	})

	t.Run("판매 불가 메시지 있음 → 헤더 포함 조합 메시지 반환", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByScheduler, richSnapshot,
			"", "", "단종 상품 내용", false,
		)
		assert.Contains(t, got, "알 수 없는 상품 목록:")
		assert.Contains(t, got, "단종 상품 내용")
	})

	t.Run("복수 섹션 동시 존재 → 모두 포함", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByScheduler, richSnapshot,
			"A 변동", "B 중복", "C 단종", false,
		)
		assert.Contains(t, got, "상품 정보가 변경되었습니다.")
		assert.Contains(t, got, "중복으로 등록된 상품 목록:")
		assert.Contains(t, got, "알 수 없는 상품 목록:")
	})

	// ── 이벤트 없음 + 스케줄러 실행 ───────────────────────────────────────────

	t.Run("변경 없음 + 스케줄러 실행 → 빈 문자열 반환 (침묵)", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByScheduler, richSnapshot,
			"", "", "", false,
		)
		assert.Equal(t, "", got, "스케줄러 실행에서 변경 없으면 알림 없어야 합니다")
	})

	// ── 이벤트 없음 + 사용자 직접 실행 ───────────────────────────────────────

	t.Run("변경 없음 + 사용자 실행 + 상품 있음 → 현재 상태 보고 메시지", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByUser, richSnapshot,
			"", "", "", false,
		)
		assert.Contains(t, got, "변경된 상품 정보가 없습니다.")
		assert.Contains(t, got, "현재 등록된 상품 정보는 아래와 같습니다:")
		assert.Contains(t, got, "사과")
		assert.Contains(t, got, "바나나")
	})

	t.Run("변경 없음 + 사용자 실행 + 상품 없음 → 등록 없음 안내", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByUser, emptySnapshot,
			"", "", "", false,
		)
		assert.Equal(t, "등록된 상품 정보가 존재하지 않습니다.", got)
	})

	t.Run("사용자 실행 + 상품 보고 시 다중 상품 사이 \\n\\n 구분자", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByUser, richSnapshot,
			"", "", "", false,
		)
		// 두 상품 사이에 빈 줄이 삽입되어야 합니다.
		assert.Contains(t, got, "\n\n")
	})

	t.Run("이벤트 메시지 있으면 사용자 실행이어도 이벤트 메시지 우선", func(t *testing.T) {
		t.Parallel()
		got := buildNotificationMessage(
			contract.TaskRunByUser, richSnapshot,
			"A 변동", "", "", false,
		)
		assert.Contains(t, got, "상품 정보가 변경되었습니다.")
		assert.NotContains(t, got, "변경된 상품 정보가 없습니다.")
	})
}
