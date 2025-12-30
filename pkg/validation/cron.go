package validation

import (
	"fmt"

	"github.com/darkkaiser/notify-server/pkg/cronx"
)

// ValidateCronExpression Cron 표현식의 유효성을 검사합니다.
//
// 이 함수는 표준 Linux Cron(5필드)이 아닌, 초(Seconds) 단위를 포함하는
// Extended/Quartz 포맷(6필드)을 기준으로 검증합니다.
// 예: "0 30 * * * *" (매시간 30분 0초)
func ValidateCronExpression(spec string) error {
	_, err := cronx.StandardParser().Parse(spec)
	if err != nil {
		return fmt.Errorf("Cron 표현식 파싱 실패(spec=%q): %w", spec, err)
	}
	return nil
}
