// Package handler v1 API의 HTTP 요청 핸들러를 제공합니다.
//
// 이 패키지는 HTTP 요청을 받아 검증하고, 비즈니스 로직을 호출한 후,
// 적절한 HTTP 응답을 반환하는 핸들러 함수들을 포함합니다.
package handler

import (
	"github.com/darkkaiser/notify-server/internal/service/api/auth"
	"github.com/darkkaiser/notify-server/internal/service/notification"
)

// Handler v1 API 요청을 처리하고 비즈니스 로직을 연결하는 핸들러입니다.
//
// 이 구조체는 다음 역할을 수행합니다:
//   - HTTP 요청 바인딩 및 검증
//   - 애플리케이션 인증 처리
//   - 비즈니스 로직(알림 전송) 호출
//   - HTTP 응답 생성
//
// Handler는 의존성 주입을 통해 생성되며, 인증 관리자와 알림 전송 서비스를 주입받습니다.
type Handler struct {
	// applicationManager 애플리케이션 인증을 담당하는 매니저
	// API 요청 시 app_key를 검증하여 등록된 애플리케이션인지 확인합니다.
	authenticator *auth.Authenticator

	// notificationSender 알림 메시지 발송을 담당하는 인터페이스
	// 텔레그램 등의 메신저로 메시지를 전송합니다.
	notificationSender notification.Sender
}

// NewHandler Handler 인스턴스를 생성합니다.
//
// 매개변수:
//   - applicationManager: 애플리케이션 인증을 담당하는 매니저
//   - notificationSender: 알림 전송을 담당하는 Sender 구현체
//
// 반환값:
//   - 초기화된 Handler 인스턴스
func NewHandler(authenticator *auth.Authenticator, notificationSender notification.Sender) *Handler {
	return &Handler{
		authenticator: authenticator,

		notificationSender: notificationSender,
	}
}
