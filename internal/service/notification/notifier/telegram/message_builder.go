package telegram

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/pkg/strutil"
)

const (
	// maxTitleLength 제목의 최대 길이 제한
	// 너무 긴 제목으로 인해 HTML 태그가 닫히지 않은 채 메시지가 분할되는 문제를 방지합니다.
	maxTitleLength = 200

	// titleFormat 제목이 포함된 메시지 포맷
	// 형식: "<b>【 제목 】</b>\n\n원본메시지"
	// 제목을 굵은 글씨로 강조하고 원본 메시지와 구분하기 위해 빈 줄을 추가합니다.
	titleFormat = "<b>【 %s 】</b>\n\n%s"

	// errorFormat 에러 발생 시 메시지 포맷
	// 형식: "원본메시지\n\n*** 오류가 발생하였습니다. ***"
	// 메시지 하단에 에러 경고 문구를 추가하여 사용자의 주의를 환기시킵니다.
	errorFormat = "%s\n\n*** 오류가 발생하였습니다. ***"

	// elapsedTimeFormat 경과 시간 표시 포맷
	// 형식: " (1시간 30분 10초 지남)"
	// 작업 실행 시간을 읽기 쉬운 형식으로 메시지에 추가합니다.
	elapsedTimeFormat = " (%s 지남)"
)

// buildEnrichMessage 원본 알림 메시지에 메타데이터를 추가하여 사용자에게 더 풍부한 정보를 제공합니다.
//
// 파라미터:
//   - notification: 알림 정보를 담은 객체 (메시지, 제목, 작업 ID, 에러 상태 등)
//
// 반환값:
//   - 메타데이터가 추가된 최종 알림 메시지 문자열
func (n *telegramNotifier) buildEnrichMessage(notification *contract.Notification) string {
	// 원본 메시지를 시작점으로 설정
	message := notification.Message

	// 1단계: 작업 제목 추가
	message = n.withTitle(notification, message)

	// 2단계: 취소 명령어 안내 추가
	// 작업이 취소 가능(Cancelable=true)하고 인스턴스 ID가 있는 경우,
	// 사용자가 메시지를 통해 직접 작업을 취소할 수 있도록 명령어 링크를 추가합니다.
	message = n.withCancelCommand(notification, message)

	// 3단계: 경과 시간 추가
	// 작업 실행 시간이 있는 경우(ElapsedTime > 0), 읽기 쉬운 형식으로 경과 시간을 표시합니다.
	message = n.withElapsedTime(notification, message)

	// 4단계: 오류 발생 시 강조 표시 추가
	// 작업 실행 중 에러가 발생한 경우(ErrorOccurred=true),
	// 메시지 하단에 경고 문구를 추가하여 사용자의 주의를 환기시킵니다.
	if notification.ErrorOccurred {
		message = fmt.Sprintf(errorFormat, message)
	}

	// 모든 메타데이터가 추가된 최종 알림 메시지 반환
	return message
}

// withTitle 메시지에 제목을 포함시킵니다.
//
// 이 함수는 알림 메시지 상단에 작업 제목을 굵은 글씨로 표시하여 사용자가 어떤 작업에 대한
// 알림인지 즉시 파악할 수 있도록 합니다. 제목 조회는 다음 2단계 전략으로 수행됩니다:
//
//  1. 직접 제공된 제목 사용: notification.Title이 있으면 우선 사용
//  2. ID 기반 조회: 제목이 없으면 TaskID + CommandID로 등록된 봇 명령어에서 제목 조회
//
// 제목을 찾지 못한 경우 원본 메시지를 그대로 반환합니다.
//
// 파라미터:
//   - notification: 알림 정보를 담은 객체 (메시지, 제목, 작업 ID, 에러 상태 등)
//   - message: 제목을 추가할 알림 메시지
//
// 반환값:
//   - 제목이 포함된 최종 알림 메시지 (제목이 없으면 원본 메시지 그대로)
func (n *telegramNotifier) withTitle(notification *contract.Notification, message string) string {
	// 전략 1: 직접 제공된 제목 사용
	if title := notification.Title; len(title) > 0 {
		// HTML 안전성 처리: 제목을 안전하게 변환하는 2단계 프로세스
		//
		// 1단계: 길이 제한
		//   - 너무 긴 제목으로 인해 HTML 태그가 닫히지 않은 채 메시지가 분할되는 문제 방지
		//   - maxTitleLength(200자)로 제한
		//
		// 2단계: HTML 이스케이프
		//   - 사용자 입력에 포함된 HTML 특수문자(<, >, &)를 안전하게 변환
		//   - 예: "<script>" → "&lt;script&gt;"
		//
		// 중요: 반드시 Truncate → Escape 순서로 처리해야 합니다!
		//   - 잘못된 순서(Escape → Truncate)는 이스케이프된 엔티티를 자를 수 있습니다.
		//   - 예: "&lt;" (4바이트)가 "&l" (2바이트)로 잘리면 HTML 파싱 에러 발생
		sanitizedTitle := html.EscapeString(strutil.Truncate(title, maxTitleLength))

		return fmt.Sprintf(titleFormat, sanitizedTitle, message)
	}

	// 전략 2: ID 기반 제목 조회
	// notification.Title이 비어있는 경우, 작업 ID와 명령어 ID를 사용하여 미리 등록된 봇 명령어 맵에서 제목을 조회합니다.
	taskID := notification.TaskID
	commandID := notification.CommandID

	if !taskID.IsEmpty() && !commandID.IsEmpty() {
		if commands, ok := n.botCommandsByTask[taskID]; ok {
			if botCommand, exists := commands[commandID]; exists {
				// 조회된 제목도 HTML 이스케이프 처리 (사용자 입력일 수 있으므로)
				return fmt.Sprintf(titleFormat, html.EscapeString(botCommand.title), message)
			}
		}
	}

	// 제목을 찾지 못한 경우: 원본 메시지를 그대로 반환
	// 이는 에러가 아니라 정상적인 시나리오입니다 (제목 없는 단순 알림 등)
	return message
}

// withCancelCommand 작업이 취소 가능한 경우, 메시지에 취소 명령어 링크를 포함시킵니다.
//
// 이 기능은 Notification의 Cancelable 상태가 true일 때만 활성화됩니다.
// 주로 사용자가 직접 실행한 장기 실행 작업에 대해, 알림 메시지 자체를 통해 손쉽게 작업을
// 취소할 수 있는 UX를 제공하기 위함입니다.
//
// 파라미터:
//   - notification: 알림 정보를 담은 객체 (메시지, 제목, 작업 ID, 에러 상태 등)
//   - message: 현재까지 조합된 알림 메시지
//
// 반환값:
//   - 취소 명령어가 포함된(조건 충족 시) 알림 메시지
func (n *telegramNotifier) withCancelCommand(notification *contract.Notification, message string) string {
	// 조건 1: 작업이 취소 가능한지 확인
	if !notification.Cancelable {
		return message
	}

	// 조건 2: 인스턴스 ID 확인
	// 취소 명령어를 생성하려면 작업을 식별할 수 있는 InstanceID가 필요합니다.
	// InstanceID가 없으면 취소 명령어를 생성할 수 없으므로 원본 메시지를 반환합니다.
	instanceID := notification.InstanceID
	if instanceID.IsEmpty() {
		return message
	}

	// 취소 명령어 생성
	cancelCmd := fmt.Sprintf("%s%s%s%s", botCommandPrefix, botCommandCancel, botCommandSeparator, notification.InstanceID)

	// 메시지 하단에 취소 명령어 추가
	return fmt.Sprintf("%s\n\n%s", message, cancelCmd)
}

// withElapsedTime 경과 시간이 있는 경우, 메시지에 경과 시간을 포함시킵니다.
//
// 파라미터:
//   - notification: 알림 정보를 담은 객체 (메시지, 제목, 작업 ID, 에러 상태 등)
//   - message: 현재까지 조합된 알림 메시지
//
// 반환값:
//   - 경과 시간이 포함된 알림 메시지 (경과 시간이 0이면 원본 메시지 그대로)
func (n *telegramNotifier) withElapsedTime(notification *contract.Notification, message string) string {
	// 경과 시간이 0보다 큰 경우에만 포맷팅하여 추가
	if elapsedTime := notification.ElapsedTime; elapsedTime > 0 {
		return message + formatElapsedTime(elapsedTime)
	}
	return message
}

// formatElapsedTime 경과 시간을 읽기 쉬운 한국어 문자열로 변환합니다.
//
// 시간을 시/분/초 단위로 분해하여 자연스러운 형식으로 표현합니다.
// 예: 3661초 → " (1시간 1분 1초 지남)"
//
// 특수 케이스:
//   - 0초: " (0초 지남)" 표시
//   - 시간/분만 있고 초가 0: 초 생략 (예: " (1시간 30분 지남)")
//
// 파라미터:
//   - d: 경과 시간 (time.Duration)
//
// 반환값:
//   - 포맷팅된 경과 시간 문자열 (예: " (1시간 30분 10초 지남)")
func formatElapsedTime(d time.Duration) string {
	seconds := int64(d.Seconds())

	// 시/분/초 단위로 분해
	s := seconds % 60        // 초: 60으로 나눈 나머지
	m := (seconds / 60) % 60 // 분: 총 분을 60으로 나눈 나머지
	h := seconds / 3600      // 시간: 총 초를 3600으로 나눈 몫

	var parts []string

	if h > 0 {
		parts = append(parts, fmt.Sprintf("%d시간", h))
	}
	if m > 0 {
		parts = append(parts, fmt.Sprintf("%d분", m))
	}

	// 초 처리: 두 가지 경우에 추가
	// 1. 초가 0보다 큰 경우
	// 2. 시간과 분이 모두 0인 경우 (0초라도 표시)
	if s > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%d초", s))
	}

	return fmt.Sprintf(elapsedTimeFormat, strings.Join(parts, " "))
}
