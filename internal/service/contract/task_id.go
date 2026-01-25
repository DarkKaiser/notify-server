package contract

import (
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

// TaskID 실행 가능한 작업을 식별하는 고유 식별자입니다.
type TaskID string

func (id TaskID) IsEmpty() bool {
	return len(id) == 0
}

func (id TaskID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return apperrors.New(apperrors.InvalidInput, "TaskID는 필수입니다")
	}
	return nil
}

func (id TaskID) String() string {
	return string(id)
}

// TaskCommandID 실행 가능한 명령어를 식별하는 고유 식별자입니다.
type TaskCommandID string

func (id TaskCommandID) IsEmpty() bool {
	return len(id) == 0
}

func (id TaskCommandID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return apperrors.New(apperrors.InvalidInput, "TaskCommandID는 필수입니다")
	}
	return nil
}

// Match 대상 명령 ID가 현재 명령 ID와 일치하는지, 또는 정의된 패턴에 부합하는지 검증합니다.
//
// 단순 일치(Exact Match)뿐만 아니라, 접미사 와일드카드('*')를 사용한 접두어 매칭(Prefix Match)을 지원합니다.
// 예: "CMD_*"는 "CMD_A", "CMD_B" 등과 일치한다고 판단합니다.
func (id TaskCommandID) Match(target TaskCommandID) bool {
	if target.IsEmpty() {
		return false
	}

	const wildcard = "*"

	s := string(id)
	if strings.HasSuffix(s, wildcard) {
		prefix := strings.TrimSuffix(s, wildcard)
		return strings.HasPrefix(string(target), prefix)
	}

	return id == target
}

func (id TaskCommandID) String() string {
	return string(id)
}

// TaskInstanceID 실행 중인 작업 인스턴스의 고유 식별자입니다.
type TaskInstanceID string

func (id TaskInstanceID) IsEmpty() bool {
	return len(id) == 0
}

func (id TaskInstanceID) Validate() error {
	if strings.TrimSpace(string(id)) == "" {
		return apperrors.New(apperrors.InvalidInput, "TaskInstanceID는 필수입니다")
	}
	return nil
}

func (id TaskInstanceID) String() string {
	return string(id)
}
