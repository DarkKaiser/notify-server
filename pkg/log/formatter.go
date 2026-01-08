package log

import "github.com/sirupsen/logrus"

// silentFormatter 아무런 동작도 하지 않는 포맷터입니다.
// Logrus의 특성상 io.Discard로 출력을 버리더라도 포맷팅 연산은 수행하므로, 이를 방지하기 위해 사용합니다.
// (실제 포맷팅은 Hook에서 수행)
type silentFormatter struct{}

// Format 아무런 변환도 수행하지 않고 nil을 반환합니다.
func (f *silentFormatter) Format(_ *logrus.Entry) ([]byte, error) {
	return nil, nil
}
