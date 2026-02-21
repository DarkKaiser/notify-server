package naver

import (
	"fmt"
)

// performance 네이버에서 크롤링한 공연 정보를 표현하는 도메인 모델입니다.
//
// 이 구조체는 네이버 공연 검색 API에서 받아온 데이터를 파싱하여 저장하며,
// 스냅샷 비교, 중복 제거, 알림 메시지 생성 등에 사용됩니다.
type performance struct {
	// Title 공연 제목입니다. (필수)
	Title string `json:"title"`

	// Place 공연 장소명입니다. (필수)
	Place string `json:"place"`

	// Thumbnail 공연 썸네일 이미지 URL입니다. (선택)
	Thumbnail string `json:"thumbnail"`
}

// key 공연을 고유하게 식별하기 위한 문자열 키를 반환합니다.
//
// 반환 형식: "{Title길이}:{Title}|{Place길이}:{Place}"
//
// 각 필드의 길이를 접두어로 포함하여, 필드 내용에 구분자(| 또는 :)가 포함되더라도
// 경계를 완벽하게 구분할 수 있도록 설계되었습니다.
func (p *performance) key() string {
	// 각 필드의 길이를 포함하여 결합함으로써 모호성을 제거합니다.
	// 예: Title="A|", Place="B" -> "2:A||1:B"
	// 예: Title="A", Place="|B" -> "1:A|2:|B"
	return fmt.Sprintf("%d:%s|%d:%s", len(p.Title), p.Title, len(p.Place), p.Place)
}

// equals 두 공연이 동일한 공연인지 비교합니다. (식별자 기준)
//
// 공연의 동일성은 Title(제목)과 Place(장소)의 조합으로 판단합니다.
// 예: "오페라의 유령" + "예술의전당" = 고유한 공연
//
// 이 메서드는 중복 제거 또는 스냅샷 비교 시 동일 공연 여부를 판단하는 데 사용됩니다.
func (p *performance) equals(other *performance) bool {
	if p == nil || other == nil {
		return false
	}

	return p.Title == other.Title && p.Place == other.Place
}

// contentEquals 두 공연의 모든 내용이 완전히 동일한지 비교합니다.
//
// equals()는 식별자(Title, Place)만 비교하지만, 이 메서드는
// Thumbnail 등 부가 정보까지 모두 비교하여 내용 변경 여부를 확인합니다.
func (p *performance) contentEquals(other *performance) bool {
	if !p.equals(other) {
		return false
	}

	return p.Thumbnail == other.Thumbnail
}
