package cronx

import "github.com/robfig/cron/v3"

// StandardParser 애플리케이션의 표준 Cron 표현식 파서 구현체를 반환합니다.
//
// 이 파서는 초 단위를 포함하는 6필드 확장 형식을 사용하며, 표준 5필드 형식은 지원하지 않습니다.
//
// 지원 스펙:
//   - 필드 순서: [초] [분] [시] [일] [월] [요일]
//   - 특수 표현식: @daily, @hourly, @every <duration> 등 (Descriptor)
//
// 예시:
//   - "0 */5 * * * *" : 매 5분 0초마다 실행 (0초 시점)
//   - "@daily"        : 매일 자정에 실행
func StandardParser() cron.Parser {
	return cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
}
