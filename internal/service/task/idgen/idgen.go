package idgen

import (
	"sync/atomic"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
)

const (
	// base62Chars Base62 인코딩에 사용되는 문자셋입니다.
	// 0-9, A-Z, a-z 순서로 구성되어 있으며, 이는 ASCII 코드 순서와 일치합니다.
	// ASCII 순서 준수 이유:
	//   - 0-9: ASCII 48-57
	//   - A-Z: ASCII 65-90
	//   - a-z: ASCII 97-122
	// 이 순서를 따르면 생성된 ID가 문자열로 비교될 때 사전순 정렬(Lexicographical Sort)이
	// 시간순 정렬과 대략적으로 일치하게 되어, 데이터베이스 인덱싱이나 로그 분석 시 유리합니다.
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	base62Len = int64(len(base62Chars))
)

// Generator 작업 인스턴스의 고유 식별자 생성을 담당합니다.
//
// 생성 전략:
//   - 타임스탬프(나노초 단위)를 기반으로 시간 순서를 반영합니다.
//   - 원자적(atomic) 카운터를 결합하여 동일 나노초 내 중복을 방지합니다.
//   - Base62 인코딩을 사용하여 URL-safe하고 가독성 높은 ID를 생성합니다.
type Generator struct {
	// counter 동일 나노초 내에서 생성되는 ID의 순번을 추적합니다.
	// atomic.AddUint32로 안전하게 증가시키며, uint32 범위(약 42억)까지 지원합니다.
	// 오버플로우 시 0으로 돌아가지만, 타임스탬프가 변경되므로 실질적 충돌 위험은 없습니다.
	counter uint32
}

// New 새로운 TaskInstanceID를 생성합니다.
//
// ID 구조:
//   - [타임스탬프(Base62)][시퀀스(Base62, 6자리 고정)]
//   - 예: "2Xk9pL3m000001" (타임스탬프 부분 + 시퀀스 "000001")
//
// 생성 과정:
//  1. 현재 시각을 나노초 단위로 가져와 Base62로 인코딩 (시간 정보 반영)
//  2. 원자적 카운터를 1 증가시켜 동일 나노초 내 순번 확보
//  3. 시퀀스를 6자리 고정 길이로 Base62 인코딩 (정렬 보장)
//  4. 타임스탬프 + 시퀀스를 결합하여 최종 ID 생성
//
// 정렬 보장 메커니즘:
//   - 타임스탬프가 앞에 위치하므로 시간 순서가 우선 반영됩니다.
//   - 시퀀스를 고정 길이로 패딩하여 자릿수 차이로 인한 정렬 오류를 방지합니다.
//     (예: "1" < "10" 이지만, "000001" < "000010" 로 올바른 순서 유지)
func (g *Generator) New() contract.TaskInstanceID {
	// 1. 현재 시각을 나노초 단위로 가져옵니다.
	now := time.Now().UnixNano()

	// 2. 원자적 카운터를 증가시켜 동일 나노초 내 생성되는 ID의 순번을 확보합니다.
	seq := atomic.AddUint32(&g.counter, 1)

	// 3. 결과를 저장할 바이트 슬라이스를 미리 할당합니다.
	// 용량 계산 근거:
	//   - int64 최대값(9,223,372,036,854,775,807)을 Base62로 변환하면 약 11자리
	//   - 시퀀스 고정 길이 6자리
	//   - 여유분 포함하여 18 바이트로 설정 (재할당 방지로 성능 향상)
	b := make([]byte, 0, 18)

	// 4. 타임스탬프를 Base62로 인코딩하여 버퍼에 추가합니다.
	b = g.appendBase62(b, now)

	// 5. 시퀀스를 6자리 고정 길이로 Base62 인코딩하여 추가합니다.
	b = g.appendBase62FixedLength(b, int64(seq), 6)

	return contract.TaskInstanceID(b)
}

// appendBase62 정수 값을 Base62로 인코딩하여 기존 버퍼에 추가합니다.
//
// 매개변수:
//   - dst: 인코딩 결과를 추가할 대상 버퍼
//   - num: Base62로 변환할 정수 값 (음수는 절댓값으로 처리)
//
// 반환값:
//   - []byte: 인코딩된 문자열이 추가된 버퍼
//
// 예시:
//   - appendBase62([]byte{}, 0) → "0"
//   - appendBase62([]byte{}, 61) → "z"
//   - appendBase62([]byte{}, 62) → "10"
//   - appendBase62([]byte{}, 123) → "1Z"
func (g *Generator) appendBase62(dst []byte, num int64) []byte {
	// 0은 특별 케이스로 처리 (반복문 진입 방지)
	if num == 0 {
		return append(dst, base62Chars[0])
	}

	// 음수는 절댓값으로 변환 (타임스탬프는 항상 양수이지만 안전장치)
	if num < 0 {
		num = -num
	}

	// 임시 버퍼: int64 최대값도 Base62로 11자리 이내이므로 20바이트면 충분
	// 스택 할당으로 힙 할당 오버헤드 제거
	var temp [20]byte
	i := len(temp)

	// Base62 변환: 낮은 자리부터 계산하여 역순으로 저장
	for num > 0 {
		i--
		temp[i] = base62Chars[num%base62Len] // 나머지로 현재 자리 계산
		num /= base62Len                     // 몫으로 다음 자리 이동
	}

	// 임시 버퍼의 유효 부분(temp[i:])을 대상 버퍼에 추가
	return append(dst, temp[i:]...)
}

// appendBase62FixedLength 정수를 Base62로 인코딩하되, 지정된 고정 길이를 맞춥니다.
//
// 고정 길이 패딩의 필요성:
//   - 문자열 비교 시 자릿수가 다르면 정렬 순서가 깨집니다.
//   - 예: "1" < "10" (문자열 비교) vs 1 < 10 (숫자 비교) - 순서 일치
//   - 예: "1" > "10" (X) vs "01" < "10" (O) - 패딩으로 순서 보장
//
// 동작 방식:
//  1. 필요한 자릿수를 먼저 계산합니다.
//  2. 버퍼의 용량(Capacity)이 충분한지 확인하고 길이를 확장합니다.
//  3. 버퍼의 끝에서부터 앞으로 채워나가며 인코딩과 패딩을 수행합니다.
//  4. 불필요한 메모리 할당과 복사를 제거하여 성능을 극대화합니다.
//
// 매개변수:
//   - dst: 인코딩 결과를 추가할 대상 버퍼
//   - num: Base62로 변환할 정수 값
//   - length: 목표 고정 길이 (부족한 만큼 앞에 '0' 패딩)
//
// 반환값:
//   - []byte: 고정 길이로 패딩된 인코딩 결과가 추가된 버퍼
func (g *Generator) appendBase62FixedLength(dst []byte, num int64, length int) []byte {
	// 음수는 절댓값으로 변환
	if num < 0 {
		num = -num
	}

	// 1. 숫자를 표현하는데 필요한 자릿수 계산
	temp := num
	digits := 0
	if temp == 0 {
		digits = 1
	}
	for temp > 0 {
		temp /= base62Len
		digits++
	}

	// 2. 최종 추가할 길이 결정 (목표 길이 vs 실제 숫자 길이)
	// 실제 숫자 길이가 목표 길이보다 길면 잘라내지 않고 그대로 모두 표현합니다.
	appendLen := length
	if digits > length {
		appendLen = digits
	}

	// 3. 버퍼 확장
	startLen := len(dst)
	targetLen := startLen + appendLen

	if cap(dst) >= targetLen {
		// 용량이 충분하면 슬라이스 길이만 확장 (메모리 할당 없음)
		dst = dst[:targetLen]
	} else {
		// 용량이 부족하면 할당 발생
		dst = append(dst, make([]byte, appendLen)...)
	}

	// 4. 뒤에서부터 앞으로 채우기 (패딩 및 숫자 변환)
	idx := targetLen - 1

	// 4-1. 숫자 변환하여 채우기
	if num == 0 {
		dst[idx] = base62Chars[0]
		idx--
	} else {
		for num > 0 {
			dst[idx] = base62Chars[num%base62Len]
			num /= base62Len
			idx--
		}
	}

	// 4-2. 남은 앞부분을 '0' (패딩)으로 채우기
	for idx >= startLen {
		dst[idx] = base62Chars[0]
		idx--
	}

	return dst
}
