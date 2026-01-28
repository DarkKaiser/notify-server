package fetcher

import (
	"fmt"
	"net/http"

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

// CheckResponseStatus HTTP 응답 상태 코드를 분석하여 도메인 에러로 변환합니다.
// 200 OK가 아닌 경우 상태 코드에 따라 적절한 에러 타입을 반환합니다.
func CheckResponseStatus(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	errType := apperrors.ExecutionFailed
	// 5xx (Server Error) or 429 (Too Many Requests) -> Unavailable
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests {
		errType = apperrors.Unavailable
	}

	return apperrors.New(errType, fmt.Sprintf("HTTP 요청이 실패했습니다. 상태 코드: %s", resp.Status))
}
