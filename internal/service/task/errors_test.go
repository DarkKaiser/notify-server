package task

import (
	"fmt"
	"strings"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// TestStaticErrors는 errors.go에 정의된 4종의 정적 에러들이 명세대로 잘 구성되어 있는지 검증합니다.
//
// 검증 항목:
//   - 정의된 에러 변수가 nil이 아님을 확인
//   - Error() 반환 문자열에 핵심 텍스트가 포함되어 있는지 확인
//   - 에러의 타입이 apperrors.Internal 인지 확인
func TestStaticErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		expectedMsg string
	}{
		{
			name:        "ErrNotificationSenderNotInitialized",
			err:         ErrNotificationSenderNotInitialized,
			expectedMsg: "NotificationSender 객체가 초기화되지 않았습니다",
		},
		{
			name:        "ErrServiceNotRunning",
			err:         ErrServiceNotRunning,
			expectedMsg: "Task 서비스가 현재 실행 중이지 않아 요청을 수행할 수 없습니다",
		},
		{
			name:        "ErrInvalidTaskSubmitRequest",
			err:         ErrInvalidTaskSubmitRequest,
			expectedMsg: "작업 실행 요청 정보가 유효하지 않아 요청을 처리할 수 없습니다",
		},
		{
			name:        "ErrCancelQueueFull",
			err:         ErrCancelQueueFull,
			expectedMsg: "작업 취소 대기열이 포화 상태에 도달하여 일시적으로 요청을 접수할 수 없습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. nil 여부 확인
			require.Error(t, tt.err, "에러 객체는 nil이 아니어야 합니다")

			// 2. 메시지 포함 여부 확인
			require.True(t, strings.Contains(tt.err.Error(), tt.expectedMsg),
				"에러 메시지에 기대하는 문자열이 포함되어야 합니다.\nExpected contains: %s\nGot: %s", tt.expectedMsg, tt.err.Error())

			// 3. apperrors.Internal 속성 검증
			require.Equal(t, apperrors.Internal, apperrors.UnderlyingType(tt.err),
				"에러의 근본 원인은 apperrors.Internal 카테고리로 분류되어야 합니다")
		})
	}
}

// TestNewTaskSubmitPanicError는 Submit() 처리 중 패닉 시 생성되는 에러 헬퍼 함수를 검증합니다.
func TestNewTaskSubmitPanicError(t *testing.T) {
	panicVal := "simulated submit panic"

	err := newTaskSubmitPanicError(panicVal)

	require.Error(t, err)
	require.Equal(t, apperrors.Internal, apperrors.UnderlyingType(err), "패닉 복구 에러는 Internal 카테고리로 래핑되어야 합니다")

	expectedMsg := fmt.Sprintf("상세: %v", panicVal)
	require.True(t, strings.Contains(err.Error(), expectedMsg), "에러 메시지에 원래 패닉 값이 명시되어야 합니다")
	require.True(t, strings.Contains(err.Error(), "작업 실행 요청 처리 중 예기치 않은 내부 오류"), "에러 메시지에 실행 요청 패닉임을 알 수 있는 문구가 포함되어야 합니다")
}

// TestNewTaskCancelPanicError는 Cancel() 처리 중 패닉 시 생성되는 에러 헬퍼 함수를 검증합니다.
func TestNewTaskCancelPanicError(t *testing.T) {
	panicVal := 12345 // 문자열이 아닌 다른 타입의 패닉 값 테스트

	err := newTaskCancelPanicError(panicVal)

	require.Error(t, err)
	require.Equal(t, apperrors.Internal, apperrors.UnderlyingType(err), "패닉 복구 에러는 Internal 카테고리로 래핑되어야 합니다")

	expectedMsg := fmt.Sprintf("상세: %v", panicVal)
	require.True(t, strings.Contains(err.Error(), expectedMsg), "에러 메시지에 원래 패닉 값이 포맷팅되어 포함되어야 합니다")
	require.True(t, strings.Contains(err.Error(), "작업 취소 처리 중 예기치 않은 내부 오류"), "에러 메시지에 취소 처리 패닉임을 알 수 있는 문구가 포함되어야 합니다")
}
