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

// renderCurrentStatus 현재 스냅샷에 기록된 전체 감시 공연 목록을 하나의 통합 메시지로 렌더링합니다.
//
// 사용자가 수동으로 작업을 실행했으나 이전 대비 변경 사항이 없을 때,
// "현재 감시 중인 공연들의 최신 상태"를 한눈에 브리핑하기 위해 analyzeAndReport에서 호출됩니다.
//
// 매개변수:
//   - snapshot: 현재 시점에 수집된 전체 공연 정보 스냅샷
//   - supportsHTML: 알림을 수신할 메신저 채널(예: 텔레그램)의 HTML 서식 지원 여부
//
// 반환값:
//   - 전체 감시 공연 목록이 포함된 렌더링된 메시지 문자열
//   - 스냅샷이 nil이거나 공연 정보가 0건인 경우 빈 문자열을 반환합니다.
func renderCurrentStatus(snapshot *watchNewPerformancesSnapshot, supportsHTML bool) string {
	if snapshot == nil || len(snapshot.Performances) == 0 {
		return ""
	}

	var sb strings.Builder

	// 등록된 공연 수에 따라 버퍼 크기 사전 할당 (메모리 재할당 최소화)
	sb.Grow(len(snapshot.Performances) * estimatedPerformanceMsgSize)

	for i, p := range snapshot.Performances {
		// 첫 번째 공연이 아니면 구분을 위해 빈 줄 추가
		if i > 0 {
			sb.WriteString("\n\n")
		}

		sb.WriteString(renderPerformance(p, supportsHTML, ""))
	}

	return sb.String()
}
