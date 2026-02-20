package navershopping

// product 네이버 쇼핑 검색 API를 통해 조회된 개별 상품 정보를 표현하는 도메인 모델입니다.
//
// API 응답 데이터를 정제하여 보관하며, 이후 스냅샷 비교 로직에서
// 가격 변동 및 신규 상품 여부를 판단하는 핵심 자료로 활용됩니다.
type product struct {
	// ProductID 네이버 쇼핑에서 부여한 상품의 유니크한 ID입니다.
	// 이 식별자를 기준으로 이전 스냅샷과 현재 스냅샷 간의 상품 매칭이 수행됩니다.
	ProductID string `json:"productId"`

	// ProductType 상품의 노출 형태 및 판매 상태를 나타내는 유형 코드입니다.
	// (예: 1-일반상품, 2-중고상품, 3-단종상품, 4-판매예정 등)
	ProductType string `json:"productType"`

	// Title 상품의 공식 명칭입니다.
	// 검색 결과 목록이나 알림 메시지에서 제목으로 노출되는 정보입니다.
	Title string `json:"title"`

	// Link 해당 상품의 상세 페이지로 연결되는 직접 링크 주소(URL)입니다.
	Link string `json:"link"`

	// LowPrice 상품 정보 수집 시점의 최저 판매 가격입니다. (단위: 원)
	// 이전 스냅샷과 가격을 비교하여 변동 여부를 판단하는 기준값이 됩니다.
	LowPrice int `json:"lprice"`

	// MallName 현재 시점에 해당 최저가를 보장하고 있는 판매처 소속(쇼핑몰 이름)입니다.
	MallName string `json:"mallName"`
}

// key 상품을 고유하게 식별하기 위한 키를 반환합니다.
func (p *product) key() string {
	return p.ProductID
}

// isPriceEligible 이 상품이 알림 대상인지 판단합니다. (조건: 최저가가 0원 초과이고, 사용자가 설정한 상한가 미만)
//
// 단순히 가격 상한선(priceLessThan)과의 비교만 하는 것이 아니라,
// 네이버 API가 간혹 0원의 가격을 반환하는 비정상적인 케이스(무효 데이터)를 사전에 걸러내는 역할도 겸합니다.
//
// 매개변수:
//   - priceLessThan: 사용자가 설정한 알림 기준 상한가입니다. (단위: 원)
//     이 금액보다 낮은 상품(미만, < 조건)만 수집 대상으로 인정됩니다.
//
// 반환값:
//   - true:  최저가가 1원 이상이고, priceLessThan 미만인 경우 (수집 대상)
//   - false: 최저가가 0원 이하(비정상 데이터)이거나, priceLessThan 이상인 경우 (제외)
func (p *product) isPriceEligible(priceLessThan int) bool {
	return p.LowPrice > 0 && p.LowPrice < priceLessThan
}
