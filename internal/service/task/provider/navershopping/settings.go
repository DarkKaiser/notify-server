package navershopping

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// taskSettings 네이버 쇼핑 오픈 API 호출에 필요한 공통 인증 정보를 담는 구조체입니다.
//
// 네이버 개발자 센터(https://developers.naver.com)에서 애플리케이션을 등록한 후
// 발급받은 '클라이언트 ID'와 '클라이언트 시크릿'을 여기에 설정합니다.
// 이 설정 정보는 해당 작업(Task)에 속한 모든 내부 명령(Command)들이 API 호출 시 공유하여 사용합니다.
type taskSettings struct {
	// ClientID 네이버 오픈 API 인증에 사용할 클라이언트 ID 값입니다. (필수 입력값)
	ClientID string `json:"client_id"`

	// ClientSecret 네이버 오픈 API 인증에 사용할 클라이언트 시크릿 값입니다. (필수 입력값)
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

// watchPriceSettings 네이버 쇼핑 통합 검색 결과를 바탕으로 특정 상품의 가격 변동을 감시할 때 필요한 세부 설정들을 정의하는 구조체입니다.
type watchPriceSettings struct {
	// Query 네이버 쇼핑에서 검색할 상품의 키워드입니다. (필수)
	// 예: "아이폰 15 프로", "로지텍 MX Master 3S"
	Query string `json:"query"`

	// Filters 단순 검색 결과를 넘어, 사용자가 원하는 정확한 조건의 상품만 추려내기 위한 상세 필터링 조건들입니다. (선택)
	Filters struct {
		// IncludedKeywords 검색된 상품의 이름에 반드시 포함되어야만 하는 키워드 목록입니다. (쉼표로 구분)
		// 여러 개를 입력할 경우 **모든 키워드가 포함(AND 조건)**된 상품만 알림 대상으로 수집합니다.
		IncludedKeywords string `json:"included_keywords"`

		// ExcludedKeywords 검색된 상품의 이름에 포함되어 있다면 알림 대상에서 즉시 제외시킬 키워드 목록입니다. (쉼표로 구분)
		// 입력된 키워드 중 **하나라도 포함(OR 조건)**되면 해당 상품은 수집 및 알림에서 제외됩니다.
		ExcludedKeywords string `json:"excluded_keywords"`

		// PriceLessThan 사용자가 구매를 희망하는 최대한도 가격(원)입니다. (필수, 0보다 큰 양수)
		// 상품의 최저가가 이 금액 '미만'일 때만 알림이 발생합니다.
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
