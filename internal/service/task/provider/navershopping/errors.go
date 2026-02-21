package navershopping

import (
	"fmt"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
)

var (
	// ErrClientIDMissing 네이버 오픈 API 인증에 필요한 'client_id' 설정값이 비어있을 때 반환되는 에러입니다.
	//
	// taskSettings의 Validate() 메서드에서 검증하며, 필수값이 누락되었거나 공백인 경우 발생합니다.
	ErrClientIDMissing = apperrors.New(apperrors.InvalidInput, "client_id는 필수 설정값입니다")

	// ErrClientSecretMissing 네이버 오픈 API 인증에 필요한 'client_secret' 설정값이 비어있을 때 반환되는 에러입니다.
	//
	// taskSettings의 Validate() 메서드에서 검증하며, 필수값이 누락되었거나 공백인 경우 발생합니다.
	ErrClientSecretMissing = apperrors.New(apperrors.InvalidInput, "client_secret은 필수 설정값입니다")

	// ErrEmptyQuery 네이버 쇼핑에서 검색할 상품의 검색어('query') 설정값이 비어있을 때 반환되는 에러입니다.
	//
	// watchPriceSettings의 Validate() 메서드에서 검증하며, 검색어가 누락되었거나 공백인 경우 발생합니다.
	ErrEmptyQuery = apperrors.New(apperrors.InvalidInput, "query가 입력되지 않았거나 공백입니다")
)

// newErrInvalidPrice 네이버 쇼핑 감시 가격('price_less_than') 설정값이 유효하지 않을 때 발생하는 에러를 생성합니다.
//
// watchPriceSettings의 Validate() 메서드에서 검증하며, 가격 설정값이 0 이하인 경우 호출됩니다.
//
// 매개변수:
//   - input: 검증에 실패한 실제 가격 설정값 (0 이하의 정수값)
//
// 반환값: InvalidInput 유형의 에러 객체를 반환합니다.
func newErrInvalidPrice(input int) error {
	return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("price_less_than은 0보다 커야 합니다 (입력값: %d)", input))
}

// newErrEndpointParseFailed 원본 에러를 래핑하여 엔드포인트 URL 파싱 실패 에러를 생성합니다.
//
// 매개변수:
//   - cause: url.Parse에서 반환된 원인 에러
//
// 반환값: Internal 유형의 에러 객체를 반환합니다.
func newErrEndpointParseFailed(cause error) error {
	return apperrors.Wrap(cause, apperrors.Internal, "네이버 쇼핑 검색 API 엔드포인트 URL 파싱에 실패하였습니다")
}
