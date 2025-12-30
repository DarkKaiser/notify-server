package validation

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

var (
	// cronParser Cron 표현식 파서
	cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
)

// ValidateRobfigCronExpression Cron 표현식의 유효성을 검사합니다.
// robfig/cron 패키지를 사용하며, 초 단위를 포함한 7개 필드 형식을 지원합니다.
// 형식: 초 분 시 일 월 요일 (예: "0 */5 * * * *" - 5분마다)
func ValidateRobfigCronExpression(spec string) error {
	_, err := cronParser.Parse(spec)
	if err != nil {
		return fmt.Errorf("Cron 표현식 파싱 실패(spec=%q): %w", spec, err)
	}
	return nil
}
