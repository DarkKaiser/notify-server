package contract

// NotifierID 알림 채널의 고유 ID 타입입니다.
// NOTE: 이 타입은 여러 패키지(config, service, notifier)에서 공통으로 참조되므로,
// 순환 참조를 피하기 위해 contract 패키지에 정의되었습니다.
type NotifierID string
