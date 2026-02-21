package kurly

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/task/provider"
)

type watchProductPriceSettings struct {
	WatchProductsFile string `json:"watch_products_file"`
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ provider.Validator = (*watchProductPriceSettings)(nil)

func (s *watchProductPriceSettings) Validate() error {
	s.WatchProductsFile = strings.TrimSpace(s.WatchProductsFile)
	if s.WatchProductsFile == "" {
		return apperrors.New(apperrors.InvalidInput, "watch_products_file이 입력되지 않았거나 공백입니다")
	}
	if !strings.HasSuffix(strings.ToLower(s.WatchProductsFile), ".csv") {
		return apperrors.New(apperrors.InvalidInput, "watch_products_file 설정에는 .csv 확장자를 가진 파일 경로만 지정할 수 있습니다")
	}
	return nil
}
