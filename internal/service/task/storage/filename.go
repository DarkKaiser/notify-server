package storage

import (
	"fmt"
	"hash/fnv"
	"strings"
	"unicode/utf8"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/iancoleman/strcase"
)

// filenameReplacer 파일명 생성 시 파일 시스템에서 문제를 일으킬 수 있는 특수문자를 안전한 문자로 치환합니다.
//
// [치환이 필요한 이유]
// 파일명에 특정 문자가 포함되면 다음과 같은 문제가 발생할 수 있습니다:
// - 파일 시스템 오류: 운영체제가 파일 생성/접근을 거부하거나 예상치 못한 동작을 유발
// - 보안 취약점: 경로 이탈(Path Traversal) 공격이나 명령어 주입(Command Injection) 위험
// - 크로스 플랫폼 호환성 문제: Windows/Linux/macOS 간 파일명 규칙 차이로 인한 오류
//
// [치환 규칙]
// - 경로 이탈 방지: ".." (상위 디렉토리), "/" 및 "\" (경로 구분자)를 하이픈으로 치환
// - Windows 예약 문자: < > : " | ? * 등 Windows에서 금지된 문자를 하이픈으로 치환
// - 쉘/URL 안전성: 공백, 세미콜론 등 스크립트나 URL에서 문제가 될 수 있는 문자를 하이픈으로 치환
var filenameReplacer = strings.NewReplacer(
	"..", "--",
	"/", "-",
	"\\", "-",
	"|", "-",
	"<", "-",
	">", "-",
	":", "-",
	"\"", "-",
	"?", "-",
	"*", "-",
)

// generateFilename 작업 ID와 명령 ID를 조합하여 시스템에서 안전하게 사용할 수 있는 고유한 파일명을 생성합니다.
//
// [파일명 생성 전략: 하이브리드 방식]
// 사람이 읽기 쉬우면서도 시스템적으로 완전히 고유한 파일명을 만들기 위해 두 가지 접근을 결합했습니다:
//
// 1. 가독성
//   - ID를 Kebab-Case로 변환하여 파일 탐색기에서 쉽게 식별할 수 있도록 합니다
//   - 예: "MyTask" → "my-task", "SendEmail_v2" → "send-email-v2"
//
// 2. 고유성
//   - 원본 ID의 64비트 해시값을 추가하여 다음 문제들을 해결합니다:
//   - 이름 충돌: 서로 다른 ID가 정제 후 같은 이름이 되는 경우 방지
//   - 대소문자 구분: Windows 등 대소문자를 구분하지 않는 파일 시스템에서의 충돌 방지
//   - 길이 제한: 긴 ID가 잘려서 같아지는 경우에도 해시로 구분 가능
//
// [생성 패턴]
// "task-{정제된작업이름}-{정제된명령이름}-{16자리해시}.json"
func generateFilename(taskID contract.TaskID, commandID contract.TaskCommandID) string {
	// 1단계: 가독성 확보 - 사람이 읽을 수 있는 형태로 변환
	//
	// sanitizeName: 특수문자를 제거하고 Kebab-Case로 변환
	// truncateByBytes: 파일 시스템의 경로 길이 제한(일반적으로 255바이트)을 고려하여
	//                  각 부분을 50바이트로 제한 (전체 파일명은 약 120바이트 이내)
	taskName := sanitizeName(string(taskID))
	taskName = truncateByBytes(taskName, 50)

	commandName := sanitizeName(string(commandID))
	commandName = truncateByBytes(commandName, 50)

	// 2단계: 고유성 확보 - 충돌 방지를 위한 해시 생성
	//
	// [해시 충돌 방지 전략]
	// 단순히 두 문자열을 연결하면 다음과 같은 충돌이 발생할 수 있습니다:
	// - "ab" + "c" = "abc"
	// - "a" + "bc" = "abc"  ← 서로 다른 입력인데 같은 결과!
	//
	// 이를 방지하기 위해 "길이 접두사(Length Prefix)" 기법을 사용합니다:
	// - "{길이}:{내용}|{길이}:{내용}" 형식으로 해싱
	// - 예: "2:ab|1:c" vs "1:a|2:bc" → 서로 다른 해시값 생성
	hasher := fnv.New64a()
	_, _ = fmt.Fprintf(hasher, "%d:%s|%d:%s", len(taskID), taskID, len(commandID), commandID)
	hashSum := hasher.Sum64()

	// 3단계: 최종 파일명 조립
	//
	// 가독성 있는 이름과 고유한 해시를 결합하여 완전한 파일명을 생성합니다.
	// 해시는 16자리 16진수(64비트)로 표현되어 충돌 확률이 사실상 0에 가깝습니다.
	return fmt.Sprintf("task-%s-%s-%016x.json", taskName, commandName, hashSum)
}

// sanitizeName 파일명으로 안전하게 사용할 수 있도록 문자열을 정제합니다.
func sanitizeName(s string) string {
	// 1단계: Kebab-Case 변환으로 기본 정제
	kebab := strcase.ToKebab(s)

	// 2단계: 제어 문자(0x00-0x1F) 및 DEL(0x7F) 제거/치환
	// Windows 등 일부 파일 시스템은 제어 문자를 파일명에 허용하지 않습니다.
	// Kebab 변환 후에도 남아있을 수 있는 제어 문자를 검사하여 안전한 문자(하이픈)로 치환합니다.
	kebab = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7F {
			return '-'
		}
		return r
	}, kebab)

	// 3단계: 파일 시스템 위험 문자 명시적 치환
	// ToKebab이 이미 많은 특수문자를 처리해주지만, 보안상 중요한 문자들은 명시적으로 제거
	return filenameReplacer.Replace(kebab)
}

// truncateByBytes 문자열을 UTF-8 바이트 길이 기준으로 안전하게 자릅니다.
//
// [필요성]
// 파일 시스템은 문자 개수가 아닌 바이트 길이로 파일명 제한을 적용합니다.
// 예: Windows/Linux 대부분 255바이트 제한
//
// [안전한 자르기]
// UTF-8에서 한글, 이모지 등은 2~4바이트를 차지하므로, 단순히 바이트 인덱스로 자르면
// 문자가 중간에 잘려 깨진 문자가 생성될 수 있습니다.
//
// 이 함수는 Rune(유니코드 문자) 단위로 순회하며, limit 바이트를 초과하지 않으면서
// 마지막 문자를 온전히 포함할 수 있는 지점까지만 자릅니다.
func truncateByBytes(s string, limit int) string {
	if len(s) <= limit {
		return s
	}

	var totalBytes int
	for i := 0; i < len(s); {
		_, size := utf8.DecodeRuneInString(s[i:])

		if totalBytes+size > limit {
			// 다음 글자를 포함하면 제한을 초과하므로, 현재까지의 길이로 자릅니다.
			return s[:totalBytes]
		}

		totalBytes += size
		i += size
	}

	return s[:totalBytes]
}
