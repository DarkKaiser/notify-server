package task

import (
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

const (
	// Base62 문자셋 (0-9, A-Z, a-z) - ASCII 순서와 일치시킴 (Lexicographical Sortable)
	// 0-9: 48-57
	// A-Z: 65-90
	// a-z: 97-122
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	base62Len   = int64(len(base62Chars))
)

// instanceIDGenerator 고유한 InstanceID를 생성합니다.
// 타임스탬프(나노초)와 원자적 카운터를 결합하여 동시성 환경에서도 충돌 없는 ID를 보장합니다.
type instanceIDGenerator struct {
	counter uint32
}

// New 새로운 InstanceID를 생성합니다.
// 생성된 ID는 시간 순서로 정렬 가능하며(대략적), 단일 프로세스 내에서 유일성을 보장합니다.
func (g *instanceIDGenerator) New() contract.TaskInstanceID {
	// 나노초 단위 타임스탬프
	now := time.Now().UnixNano()

	// Atomic 카운터 증가 (동일 나노초 내 충돌 방지)
	seq := atomic.AddUint32(&g.counter, 1)

	// 기본 용량 할당 (int64 최대값 Base62 변환 시 약 11자리 + seq 6자리)
	b := make([]byte, 0, 18)

	// 타임스탬프 인코딩 (Base62)
	b = g.appendBase62(b, now)

	// 시퀀스 인코딩 (Base62) - 고정 길이(6자리) 패딩으로 Monotonic 보장
	// uint32 최대값 4,294,967,295 -> Base62로 "4XXkCP" (6자리)
	// 패딩을 해야 문자열 정렬 시 자릿수 차이로 인한 문제와 아스키 코드 순서 문제를 완화할 수 있음.
	b = g.appendBase62FixedLength(b, int64(seq), 6)

	return contract.TaskInstanceID(b)
}

// appendBase62 정수 값을 Base62로 인코딩하여 버퍼에 추가합니다.
func (g *instanceIDGenerator) appendBase62(dst []byte, num int64) []byte {
	if num == 0 {
		return append(dst, base62Chars[0])
	}
	if num < 0 {
		num = -num
	}

	var temp [20]byte
	i := len(temp)

	for num > 0 {
		i--
		temp[i] = base62Chars[num%base62Len]
		num /= base62Len
	}

	return append(dst, temp[i:]...)
}

// appendBase62FixedLength 정수를 Base62로 인코딩하되 고정 길이를 맞춥니다 (앞에 '0' 패딩)
// 이를 통해 사전순 정렬을 보장합니다.
func (g *instanceIDGenerator) appendBase62FixedLength(dst []byte, num int64, length int) []byte {
	startLen := len(dst)

	// 먼저 일반 인코딩
	dst = g.appendBase62(dst, num)

	encodedLen := len(dst) - startLen
	paddingNeeded := length - encodedLen

	if paddingNeeded > 0 {
		// 패딩이 필요하면 뒤로 밀고 앞에 0을 채움
		dst = append(dst, make([]byte, paddingNeeded)...)

		// 이동: encoded 부분을 뒤로 밈
		copy(dst[startLen+paddingNeeded:], dst[startLen:startLen+encodedLen])

		// 패딩 채우기
		for i := 0; i < paddingNeeded; i++ {
			dst[startLen+i] = base62Chars[0]
		}
	}

	return dst
}
