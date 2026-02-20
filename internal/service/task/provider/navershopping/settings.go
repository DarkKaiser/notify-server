package navershopping

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// taskSettings 네이버 쇼핑 API 연동에 필요한 인증 정보를 담는 Task 레벨 설정 구조체입니다.
//
// 이 설정값들은 네이버 개발자 센터(https://developers.naver.com)에서 애플리케이션을 등록한 후
// 발급받을 수 있으며, 설정 파일을 통해 주입됩니다.
type taskSettings struct {
	// ClientID 네이버 오픈 API 인증에 사용되는 클라이언트 ID입니다. (필수)
	ClientID string `json:"client_id"`

	// ClientSecret 네이버 오픈 API 인증에 사용되는 클라이언트 시크릿입니다. (필수)
	ClientSecret string `json:"client_secret"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*taskSettings)(nil)

// Validate 설정값의 유효성을 검증합니다.
func (s *taskSettings) Validate() error {
	s.ClientID = strings.TrimSpace(s.ClientID)
	if s.ClientID == "" {
		return ErrClientIDMissing
	}

	s.ClientSecret = strings.TrimSpace(s.ClientSecret)
	if s.ClientSecret == "" {
		return ErrClientSecretMissing
	}

	return nil
}

// watchPriceSettings 네이버 쇼핑에서 특정 상품의 가격 변동을 감시하는 Command의 설정 구조체입니다.
type watchPriceSettings struct {
	// Query 네이버 쇼핑 검색에 사용할 검색어입니다. (필수)
	Query string `json:"query"`

	// Filters 검색 결과를 필터링하기 위한 조건들입니다. (선택)
	Filters struct {
		// IncludedKeywords 상품명에 반드시 포함되어야 하는 키워드들입니다. (쉼표로 구분)
		IncludedKeywords string `json:"included_keywords"`

		// ExcludedKeywords 상품명에 포함되면 제외할 키워드들입니다. (쉼표로 구분)
		ExcludedKeywords string `json:"excluded_keywords"`

		// PriceLessThan 이 가격(원) 미만의 상품만 알림 대상으로 포함합니다. (필수, 양수)
		PriceLessThan int `json:"price_less_than"`
	} `json:"filters"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*watchPriceSettings)(nil)

// Validate 설정값의 유효성을 검증합니다.
func (s *watchPriceSettings) Validate() error {
	s.Query = strings.TrimSpace(s.Query)
	if s.Query == "" {
		return ErrEmptyQuery
	}

	if s.Filters.PriceLessThan <= 0 {
		return NewErrInvalidPrice(s.Filters.PriceLessThan)
	}

	return nil
}
