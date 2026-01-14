package types

// NotifierID 알림 발송 채널(Notifier)을 고유하게 식별하기 위한 식별자 타입입니다.
// 순환 참조 문제 해결을 위해 별도 패키지로 분리되었습니다.
type NotifierID string
