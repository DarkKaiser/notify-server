package scraper

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrHTMLStructureChanged HTML 페이지 구조가 변경되어 파싱에 실패했을 때 반환됩니다.
	ErrHTMLStructureChanged = apperrors.New(apperrors.ExecutionFailed, "불러온 페이지의 문서구조가 변경되었습니다. CSS셀렉터를 확인하세요")
)

// NewErrHTMLStructureChanged HTML 페이지의 DOM 구조 변경으로 인한 파싱 실패 시,
// 디버깅에 필요한 컨텍스트(대상 URL, CSS 선택자 등 상세 정보)를 포함한 구조화된 에러를 생성합니다.
func NewErrHTMLStructureChanged(url, details string) error {
	message := ErrHTMLStructureChanged.Error()
	if url != "" {
		message += fmt.Sprintf(" (%s)", url)
	}
	if details != "" {
		message += fmt.Sprintf(": %s", details)
	}
	return apperrors.New(apperrors.ExecutionFailed, message)
}
