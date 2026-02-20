package naver

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"github.com/darkkaiser/notify-server/internal/pkg/mark"
)

const (
	// naverSearchURL 네이버 통합 검색 페이지의 기본 URL입니다.
	// 알림 메시지에서 공연 제목을 클릭하면 이 URL에 검색어를 추가하여 사용자를 네이버 검색 결과 페이지로 안내합니다.
	naverSearchURL = "https://search.naver.com/search.naver"

	// estimatedPerformanceMsgSize 단일 공연 정보를 렌더링할 때 필요한 예상 버퍼 크기(Byte)입니다.
	estimatedPerformanceMsgSize = 512
)

// renderPerformance 단일 공연 정보를 알림 메시지 포맷으로 렌더링합니다.
//
// 매개변수:
//   - p: 렌더링할 공연 정보
//   - supportsHTML: 알림 수신 채널이 HTML 포맷을 지원하는지 여부
//   - m: 공연 상태를 나타내는 마크 (예: "🆕" for 신규 공연, "" for 기존 공연)
//
// 반환값: 포맷팅된 공연 정보 문자열
//   - HTML 지원 시: 클릭 가능한 링크와 볼드 처리된 제목
//   - 텍스트 전용: 제목과 URL을 일반 텍스트로 표시
func renderPerformance(p *performance, supportsHTML bool, m mark.Mark) string {
	var sb strings.Builder

	// 공연 평균 메시지 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(estimatedPerformanceMsgSize)

	if supportsHTML {
		const htmlFormat = `☞ <a href="%s?query=%s"><b>%s</b></a>%s
      • 장소 : %s`

		fmt.Fprintf(&sb,
			htmlFormat,
			naverSearchURL,
			url.QueryEscape(p.Title),
			template.HTMLEscapeString(p.Title),
			m.WithSpace(),
			template.HTMLEscapeString(p.Place),
		)
	} else {
		const textFormat = `☞ %s%s (%s?query=%s)
      • 장소 : %s`

		fmt.Fprintf(&sb, textFormat, p.Title, m.WithSpace(), naverSearchURL, url.QueryEscape(p.Title), p.Place)
	}

	return sb.String()
}

// renderPerformanceDiffs 이전 스냅샷과 비교하여 발견된 신규 공연 목록을 알림 메시지로 렌더링합니다.
//
// 매개변수:
//   - diffs: 신규 공연 목록
//   - supportsHTML: 알림 수신 채널이 HTML 포맷을 지원하는지 여부
//
// 반환값: 신규 공연들을 포맷팅한 메시지 문자열
func renderPerformanceDiffs(diffs []performanceDiff, supportsHTML bool) string {
	if len(diffs) == 0 {
		return ""
	}

	var sb strings.Builder

	// 공연 개수 × 평균 메시지 크기로 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(len(diffs) * estimatedPerformanceMsgSize)

	needSeparator := false
	for _, diff := range diffs {
		if diff.Type != performanceEventNew {
			continue
		}

		// 첫 번째 렌더링 항목이 아니면 구분을 위해 빈 줄 추가
		if needSeparator {
			sb.WriteString("\n\n")
		}

		needSeparator = true

		sb.WriteString(renderPerformance(diff.Performance, supportsHTML, mark.New))
	}

	return sb.String()
}

// renderCurrentStatus 신규 공연이 없을 때 현재 등록된 모든 공연 목록을 사용자에게 보고하는 메시지를 생성합니다.
//
// 이 함수는 신규 공연이 발견되지 않았을 때 호출되며, 사용자에게 "현재 어떤 공연들이 등록되어 있는지" 알려주는 역할을 합니다.
//
// 매개변수:
//   - snapshot: 현재 등록된 공연 정보 스냅샷
//   - supportsHTML: 알림 수신 채널이 HTML 포맷을 지원하는지 여부
//
// 반환값: 현재 상태를 설명하는 메시지
//   - 등록된 공연이 없으면: "등록된 공연정보가 존재하지 않습니다."
//   - 등록된 공연이 있으면: 안내 문구 + 전체 공연 목록
func renderCurrentStatus(snapshot *watchNewPerformancesSnapshot, supportsHTML bool) string {
	if snapshot == nil || len(snapshot.Performances) == 0 {
		return "등록된 공연정보가 존재하지 않습니다."
	}

	var sb strings.Builder

	// 등록된 공연 수에 따라 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(len(snapshot.Performances) * estimatedPerformanceMsgSize)

	for i, p := range snapshot.Performances {
		// 첫 번째 공연이 아니면 구분을 위해 빈 줄 추가
		if i > 0 {
			sb.WriteString("\n\n")
		}

		// 기존 공연이므로 마크("")는 빈 문자열로 전달
		sb.WriteString(renderPerformance(p, supportsHTML, ""))
	}

	return "신규로 등록된 공연정보가 없습니다.\n\n현재 등록된 공연정보는 아래와 같습니다:\n\n" + sb.String()
}
