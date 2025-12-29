// Package mark 애플리케이션 전반에서 사용되는 이모지 상수를 중앙 관리하는 패키지입니다.
package mark

// Mark 이모지 상수를 위한 타입입니다.
type Mark string

const (
	// 신규
	New Mark = "🆕"

	// 변경
	Modified Mark = "🔁"

	// 품절/종료
	Unavailable Mark = "🚫"

	// 최저가
	BestPrice Mark = "🔥"

	// 긴급/오류
	Alert Mark = "🚨"
)

// WithSpace 마크(이모지) 앞에 구분용 공백을 추가하여 반환합니다.
func (m Mark) WithSpace() string {
	if m == "" {
		return ""
	}
	return " " + string(m)
}

// String 마크의 순수 이모지 값을 문자열로 반환합니다.
func (m Mark) String() string {
	return string(m)
}
