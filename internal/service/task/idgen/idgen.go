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

	// idBufferCap ID 생성 시 할당할 기본 버퍼 용량입니다.
	//
	// 계산 근거:
	//   - int64 최대값(9경...) Base62 변환 시 약 11자리
	//   - 시퀀스 고정 길이 6자리
	//   - 여유분 포함하여 18 바이트로 설정
	idBufferCap = 18

	// sequenceLen 동일 나노초 내 순서를 보장하기 위한 시퀀스의 고정 길이입니다.
	sequenceLen = 6
)

// generator 작업 인스턴스의 고유 식별자 생성을 담당합니다.
//
// 생성 전략:
//   - 타임스탬프(나노초 단위)를 기반으로 시간 순서를 반영합니다.
//   - 원자적(atomic) 카운터를 결합하여 동일 나노초 내 중복을 방지합니다.
//   - Base62 인코딩을 사용하여 URL-safe하고 가독성 높은 ID를 생성합니다.
type generator struct {
	// counter 동일 나노초 내에서 생성되는 ID의 순번을 추적합니다.
	// atomic.AddUint32로 안전하게 증가시키며, uint32 범위(약 42억)까지 지원합니다.
	// 오버플로우 시 0으로 돌아가지만, 타임스탬프가 변경되므로 실질적 충돌 위험은 없습니다.
	counter uint32
}

// 컴파일 타임에 인터페이스 구현 여부를 검증합니다.
var _ contract.IDGenerator = (*generator)(nil)

// New 새로운 IDGenerator를 생성하여 반환합니다.
func New() contract.IDGenerator {
	return &generator{}
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
func (g *generator) New() contract.TaskInstanceID {
	// 1. 현재 시각을 나노초 단위로 가져옵니다.
	now := time.Now().UnixNano()

	// 2. 원자적 카운터를 증가시켜 동일 나노초 내 생성되는 ID의 순번을 확보합니다.
	seq := atomic.AddUint32(&g.counter, 1)

	// 3. 결과를 저장할 바이트 슬라이스를 미리 할당합니다.
	b := make([]byte, 0, idBufferCap)

	// 4. 타임스탬프를 Base62로 인코딩하여 버퍼에 추가합니다.
	b = appendBase62(b, now)

	// 5. 시퀀스를 6자리 고정 길이로 Base62 인코딩하여 추가합니다.
	b = appendBase62FixedLength(b, int64(seq), sequenceLen)

	return contract.TaskInstanceID(b)
}

// appendBase62 정수 값을 Base62로 인코딩하여 기존 버퍼에 추가합니다.
//
// 매개변수:
//   - dst: Base62로 변환된 결과를 추가할 대상 버퍼
//   - num: Base62로 변환할 정수 값 (음수는 절대값으로 처리)
//
// 반환값:
//   - []byte: Base62로 변환된 문자열이 추가된 버퍼
func appendBase62(dst []byte, num int64) []byte {
	// 0은 나눗셈으로 처리할 수 없는 특별한 케이스이므로, 즉시 '0'을 추가하고 반환합니다.
	if num == 0 {
		return append(dst, base62Chars[0])
	}

	// 음수는 절대값으로 변환하여 처리합니다.
	if num < 0 {
		num = -num
	}

	// 1. 임시 버퍼 준비
	// int64 최대값을 Base62로 변환해도 11자리 정도이므로, 20바이트면 충분합니다.
	var temp [20]byte
	idx := len(temp)

	// 2. Base62 변환
	// 숫자의 작은 자리수(일의 자리)부터 계산되므로, 배열의 뒤에서부터 앞으로 채워나갑니다.
	for num > 0 {
		idx--
		temp[idx] = base62Chars[num%base62Len] // 나머지(=현재 자리의 문자) 저장
		num /= base62Len                       // 몫(=다음 계산할 숫자)으로 갱신
	}

	// 3. 결과 병합
	// 임시 버퍼에서 실제로 데이터가 채워진 부분(temp[idx:])만 원본 버퍼(dst)에 추가합니다.
	return append(dst, temp[idx:]...)
}

// appendBase62FixedLength 정수를 Base62로 인코딩하되, 지정된 고정 길이를 맞춥니다.
//
// 고정 길이 패딩의 필요성:
//   - 문자열 비교 시 자릿수가 다르면 정렬 순서가 깨집니다.
//   - 예: "1" < "10" (문자열 비교) vs 1 < 10 (숫자 비교) - 순서 일치
//   - 예: "1" > "10" (X) vs "01" < "10" (O) - 패딩으로 순서 보장
//
// 동작 방식:
//  1. 버퍼 용량 확보: 기존 버퍼 길이 + 고정 길이만큼 공간을 확보합니다.
//     - 용량이 충분하면 슬라이싱만 수행 (메모리 할당 없음)
//     - 용량이 부족하면 append로 확장
//  2. 역순 인코딩: 버퍼의 끝(bufferEnd-1)에서부터 앞으로 채워나갑니다.
//     - 숫자를 Base62로 변환하며 한 자리씩 채움
//     - 별도의 자릿수 계산 루프 없이 한 번의 순회로 처리
//  3. 패딩: 숫자 변환이 끝나고 남은 앞부분을 '0'으로 채웁니다.
//
// 매개변수:
//   - dst: Base62로 변환된 결과를 추가할 대상 버퍼
//   - num: Base62로 변환할 정수 값 (음수는 절대값으로 처리)
//   - length: 목표하는 고정 길이 (부족한 만큼 앞에 '0' 패딩)
//
// 반환값:
//   - []byte: 고정 길이로 패딩된 Base62 문자열이 추가된 버퍼
func appendBase62FixedLength(dst []byte, num int64, length int) []byte {
	// 음수는 절대값으로 변환하여 처리합니다.
	if num < 0 {
		num = -num
	}

	// -------------------------------------------------------------------------
	// 1. 버퍼 용량 확보
	// -------------------------------------------------------------------------
	// 대부분의 경우 dst는 이미 충분한 용량을 가지고 있어(cap >= len + length)
	// 추가적인 메모리 할당 없이 슬라이싱만으로 처리됩니다.
	offset := len(dst)
	bufferEnd := offset + length

	// 용량이 부족한 경우에만 메모리를 재할당합니다.
	if cap(dst) < bufferEnd {
		// 새 용량 계산: 기존 용량의 2배로 설정하여 잦은 할당을 방지합니다.
		// 단, 2배로도 부족하다면 필요한 만큼 딱 맞춰서 늘립니다.
		newCap := 2 * cap(dst)
		if newCap < bufferEnd {
			newCap = bufferEnd
		}

		// 새 버퍼 할당 및 기존 데이터 이사
		// 주의: `len`은 기존 데이터만큼만 설정해야 합니다. (아직 새 데이터를 채우지 않았으므로)
		newDst := make([]byte, len(dst), newCap)
		copy(newDst, dst)
		dst = newDst
	}

	// 버퍼의 길이를 확장하여 사용할 공간을 확보합니다.
	// 이제 dst[offset : bufferEnd] 영역을 자유롭게 쓸 수 있습니다.
	dst = dst[:bufferEnd]

	// -------------------------------------------------------------------------
	// 2. 숫자를 Base62로 변환하여 채우기
	// -------------------------------------------------------------------------
	// 숫자의 일의 자리부터 계산되므로, 버퍼의 뒤(끝)에서부터 앞으로 채워나갑니다.
	writePos := bufferEnd - 1

	if num == 0 {
		dst[writePos] = base62Chars[0]
		writePos--
	} else {
		for num > 0 {
			// [예외 처리] 숫자가 너무 커서 고정 길이(length)를 초과하는 경우
			//
			// 정상적인 상황에서는 발생하지 않아야 하지만, 만약 발생한다면
			// 확보해둔 고정 길이 공간(offset ~ bufferEnd)을 벗어나 앞쪽 데이터를 덮어쓰게 됩니다.
			// 이를 방지하기 위해 버퍼를 강제로 늘려서 문자를 끼워 넣습니다.
			if writePos < offset {
				// 해결책: 버퍼 중간(offset 지점)에 새 문자를 삽입(Prepend to slice)합니다.
				// 성능 손해(copy 발생)가 있지만, 데이터 오염을 막는 것이 우선입니다.
				//
				// 구조: [기존 데이터] + [새 문자] + [이미 쓴 숫자 뒷부분...]
				dst = append(dst[:offset], append([]byte{base62Chars[num%base62Len]}, dst[offset:]...)...)

				// 버퍼 길이가 1 늘어났으므로 끝 지점도 갱신
				bufferEnd++

				// writePos는 감소시키지 않고 유지하여, 다음 루프에서도 계속 이 분기(prepend)를 타도록 함
			} else {
				// 정상 케이스: 뒤에서부터 한 글자씩 채움
				dst[writePos] = base62Chars[num%base62Len]
				writePos--
			}
			num /= base62Len
		}
	}

	// -------------------------------------------------------------------------
	// 3. 남은 앞부분 패딩
	// -------------------------------------------------------------------------
	// 할당된 고정 길이보다 숫자가 짧은 경우, 남은 앞부분을 '0'으로 채웁니다.
	for writePos >= offset {
		dst[writePos] = base62Chars[0]
		writePos--
	}

	return dst
}
