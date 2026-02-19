package naver

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// watchNewPerformancesSettings 네이버 신규 공연 감시 작업의 설정 구조체입니다.
type watchNewPerformancesSettings struct {
	// Query 네이버 공연 검색에 사용할 검색어입니다. (필수)
	Query string `json:"query"`

	// Filters 검색 결과를 필터링하기 위한 조건들입니다. (선택)
	Filters struct {
		// Title 공연 제목 기반 필터링 조건입니다.
		Title struct {
			IncludedKeywords string `json:"included_keywords"` // 제목에 포함되어야 할 키워드들 (쉼표로 구분)
			ExcludedKeywords string `json:"excluded_keywords"` // 제목에 포함되면 안 되는 키워드들 (쉼표로 구분)
		} `json:"title"`

		// Place 공연 장소 기반 필터링 조건입니다.
		Place struct {
			IncludedKeywords string `json:"included_keywords"` // 장소명에 포함되어야 할 키워드들 (쉼표로 구분)
			ExcludedKeywords string `json:"excluded_keywords"` // 장소명에 포함되면 안 되는 키워드들 (쉼표로 구분)
		} `json:"place"`
	} `json:"filters"`

	// === 선택적 설정 ===
	// 아래 필드들은 값이 제공되지 않거나 0 이하일 경우,
	// ApplyDefaults() 메서드에서 기본값이 자동으로 적용됩니다.

	// MaxPages 최대 수집 페이지 수입니다. (기본값: 50)
	// 네이버 공연 검색 결과는 페이지 단위로 제공되며, 이 값은 최대 몇 페이지까지 수집할지 결정합니다.
	// 너무 큰 값을 설정하면 수집 시간이 길어지고 네이버 서버에 부담을 줄 수 있습니다.
	MaxPages int `json:"max_pages"`

	// PageFetchDelay 페이지 수집 간 대기 시간(밀리초)입니다. (기본값: 100ms)
	// 연속적인 페이지 요청 사이에 대기하는 시간으로, 네이버 서버에 과도한 부하를 주지 않기 위해 사용됩니다.
	// 값이 너무 작으면 Rate Limiting에 걸릴 수 있고, 너무 크면 전체 수집 시간이 길어집니다.
	PageFetchDelay int `json:"page_fetch_delay_ms"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*watchNewPerformancesSettings)(nil)

// Validate 설정값의 유효성을 검증합니다.
func (s *watchNewPerformancesSettings) Validate() error {
	s.Query = strings.TrimSpace(s.Query)
	if s.Query == "" {
		return ErrEmptyQuery
	}

	return nil
}

// ApplyDefaults 설정되지 않은 필드에 기본값을 적용합니다.
//
// 이 메서드는 Validate() 호출 후 실행되며, 선택적 필드들이 설정되지 않았거나
// 유효하지 않은 값(0 이하)일 경우 안전한 기본값으로 초기화합니다.
//
// 기본값:
//   - MaxPages: 50 (최대 50페이지까지 수집)
//   - PageFetchDelay: 100ms (페이지 간 100밀리초 대기)
//
// 이를 통해 사용자가 설정 파일에서 일부 값을 생략하더라도 작업이 안전하게 실행됩니다.
func (s *watchNewPerformancesSettings) ApplyDefaults() {
	if s.MaxPages <= 0 {
		s.MaxPages = 50
	}

	if s.PageFetchDelay <= 0 {
		s.PageFetchDelay = 100
	}
}
