package navershopping

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrClientIDMissing taskSettings.Validate() 실행 시 client_id가 비어 있거나 공백만 포함된 경우 반환되는 에러입니다.
	ErrClientIDMissing = apperrors.New(apperrors.InvalidInput, "client_id는 필수 설정값입니다")

	// ErrClientSecretMissing taskSettings.Validate() 실행 시 client_secret이 비어 있거나 공백만 포함된 경우 반환되는 에러입니다.
	ErrClientSecretMissing = apperrors.New(apperrors.InvalidInput, "client_secret은 필수 설정값입니다")

	// ErrEmptyQuery watchPriceSettings.Validate() 실행 시 query가 비어 있거나 공백만 포함된 경우 반환되는 에러입니다.
	ErrEmptyQuery = apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
)

// NewErrInvalidPrice price_less_than 설정값이 유효하지 않을 때 반환할 에러를 생성합니다.
//
// price_less_than은 감시 대상 상품의 가격 상한선으로, 반드시 1 이상의 양수여야 합니다.
// 0 또는 음수가 설정된 경우 이 함수를 통해 에러를 생성하며, 실제 입력값을 메시지에 포함하여
// 운영자가 설정 파일의 어떤 값이 문제인지 즉시 파악할 수 있도록 합니다.
//
// 매개변수:
//   - input: 검증에 실패한 실제 price_less_than 값 (0 이하)
//
// 반환값: InvalidInput 타입의 에러. 에러 메시지에 입력값이 포함됩니다.
func NewErrInvalidPrice(input int) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("price_less_than은 0보다 커야 합니다 (입력값: %d)", input))
}
