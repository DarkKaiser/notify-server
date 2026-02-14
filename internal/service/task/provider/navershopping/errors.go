package navershopping

import (
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrClientIDMissing Task 설정 검증 시 네이버 쇼핑 API의 client_id가 누락되었거나 공백일 때 반환됩니다.
	// 네이버 쇼핑 API 인증에 필수적인 설정값이므로 반드시 유효한 값이 설정되어야 합니다.
	ErrClientIDMissing = apperrors.New(apperrors.InvalidInput, "client_id는 필수 설정값입니다")

	// ErrClientSecretMissing Task 설정 검증 시 네이버 쇼핑 API의 client_secret이 누락되었거나 공백일 때 반환됩니다.
	// 네이버 쇼핑 API 인증에 필수적인 설정값이므로 반드시 유효한 값이 설정되어야 합니다.
	ErrClientSecretMissing = apperrors.New(apperrors.InvalidInput, "client_secret은 필수 설정값입니다")
)
