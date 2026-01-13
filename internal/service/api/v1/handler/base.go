// Package handler v1 API의 HTTP 요청 핸들러를 제공합니다.
//
// 이 패키지는 알림 전송 API의 핸들러 계층을 담당하며,
// HTTP 요청을 받아 검증하고, 비즈니스 로직을 호출한 후,
// 적절한 HTTP 응답을 반환합니다.
//
// 핸들러는 다음 책임을 가집니다:
//   - 요청 데이터 바인딩 및 유효성 검증
//   - Context에서 인증된 Application 정보 추출
//   - 비즈니스 로직(notification.Sender) 호출
//   - 성공/실패 응답 생성 및 반환
package handler

import (
	"github.com/darkkaiser/notify-server/internal/service/api/constants"
	"github.com/darkkaiser/notify-server/internal/service/notification"
)

// Handler v1 API 요청을 처리하는 핸들러입니다.
//
// Handler는 HTTP 계층과 비즈니스 로직 계층 사이의 어댑터 역할을 수행하며,
// 의존성 주입을 통해 알림 전송 서비스를 주입받습니다.
//
// 주요 역할:
//   - HTTP 요청 바인딩 및 검증
//   - 비즈니스 로직 호출 (알림 전송)
//   - HTTP 응답 생성 (성공/실패)
//
// 참고: 인증은 미들웨어에서 처리되므로, 핸들러는 이미 검증된
// Application 객체를 Context에서 추출하여 사용합니다.
type Handler struct {
	// notificationSender 알림 메시지 발송을 담당하는 인터페이스
	notificationSender notification.Sender
}

// NewHandler Handler 인스턴스를 생성합니다.
//
// Parameters:
//   - notificationSender: 알림 전송을 담당하는 Sender 구현체
//
// Returns:
//   - 초기화된 Handler 포인터
func NewHandler(notificationSender notification.Sender) *Handler {
	if notificationSender == nil {
		panic(constants.PanicMsgNotificationSenderRequired)
	}

	return &Handler{
		notificationSender: notificationSender,
	}
}
