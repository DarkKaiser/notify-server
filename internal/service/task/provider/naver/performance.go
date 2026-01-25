package naver

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
)

const (
	// searchResultPageURL 사용자에게 제공할 '검색 결과 페이지'의 기본 URL입니다.
	//
	// [목적]
	//  - 알림 메시지에서 공연명을 클릭했을 때 이동할 하이퍼링크(Target URL)를 생성하는 데 사용됩니다.
	//  - 쿼리 파라미터(?query=...)를 추가하여 사용자가 해당 공연의 상세 검색 결과를 즉시 확인할 수 있도록 돕습니다.
	searchResultPageURL = "https://search.naver.com/search.naver"
)

// performance 크롤링된 공연 정보를 담는 도메인 모델입니다.
type performance struct {
	Title     string `json:"title"`
	Place     string `json:"place"`
	Thumbnail string `json:"thumbnail"`
}

func (p *performance) Equals(other *performance) bool {
	if p == nil || other == nil {
		return false
	}
	return p.Title == other.Title && p.Place == other.Place
}

// Key 공연을 고유하게 식별하기 위한 문자열 키를 반환합니다.
//
// 반환값은 "제목|장소" 형식으로, 파이프(|) 문자를 구분자로 사용하여 제목과 장소를 결합합니다.
// 이 키는 Map 기반 중복 제거나 빠른 조회(O(1))가 필요한 상황에서 사용됩니다.
//
// [중요] 이 메서드의 비교 기준(Title + Place)은 Equals() 메서드와 반드시 일치해야 합니다.
// 만약 두 공연이 Equals()로 동일하다면, Key()도 동일한 값을 반환해야 합니다.
func (p *performance) Key() string {
	return p.Title + "|" + p.Place
}

// Render 공연 정보를 알림 메시지 포맷으로 렌더링하여 반환합니다.
// 주로 단일 공연 정보 조회와 같이 비교 대상이 없는 경우에 사용됩니다.
func (p *performance) Render(supportsHTML bool, m mark.Mark) string {
	return p.renderInternal(supportsHTML, m)
}

// RenderDiff 현재 공연 정보와 과거 정보를 비교하여 변경 사항을 강조한 알림 메시지를 생성합니다.
// 현재 Naver 패키지는 '신규' 위주이므로 Render와 큰 차이가 없을 수 있으나,
// 향후 확장성 및 타 패키지(NaverShopping)와의 일관성을 위해 인터페이스를 분리합니다.
func (p *performance) RenderDiff(supportsHTML bool, m mark.Mark, prev *performance) string {
	return p.renderInternal(supportsHTML, m)
}

// renderInternal 공연 알림 메시지를 생성하는 핵심 내부 구현체입니다.
func (p *performance) renderInternal(supportsHTML bool, m mark.Mark) string {
	var sb strings.Builder

	// 예상 버퍼 크기 할당
	sb.Grow(512)

	if supportsHTML {
		const htmlFormat = `☞ <a href="%s?query=%s"><b>%s</b></a>%s
      • 장소 : %s`

		fmt.Fprintf(&sb,
			htmlFormat,
			searchResultPageURL,
			url.QueryEscape(p.Title),
			template.HTMLEscapeString(p.Title),
			m.WithSpace(),
			p.Place,
		)
	} else {
		const textFormat = `☞ %s%s
      • 장소 : %s`

		fmt.Fprintf(&sb, textFormat, p.Title, m.WithSpace(), p.Place)
	}

	return sb.String()
}
