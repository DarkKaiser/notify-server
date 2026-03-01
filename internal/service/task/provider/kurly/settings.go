package kurly

import (
	"strings"

	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

// watchProductPriceSettings 마켓컬리 상품 가격 감시 명령(WatchProductPrice)의 설정값을 담는 구조체입니다.
type watchProductPriceSettings struct {
	// WatchListFile 감시할 상품 목록이 정의된 CSV 파일의 경로입니다. (필수)
	WatchListFile string `json:"watch_list_file"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*watchProductPriceSettings)(nil)

// Validate 설정값의 유효성을 검증합니다.
func (s *watchProductPriceSettings) Validate() error {
	// 앞뒤 공백을 제거하여 정규화합니다.
	s.WatchListFile = strings.TrimSpace(s.WatchListFile)
	if s.WatchListFile == "" {
		return ErrWatchListFileEmpty
	}

	// 대소문자 구분 없이 .csv 확장자 여부를 검사합니다.
	if !strings.HasSuffix(strings.ToLower(s.WatchListFile), ".csv") {
		return ErrWatchListFileNotCSV
	}

	return nil
}
