package lotto

import (
	"errors"
	"testing"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportedErrors(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedType apperrors.ErrorType
		expectedMsg  string
	}{
		{
			name:         "ErrAppPathMissing",
			err:          ErrAppPathMissing,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "app_path는 필수 설정값입니다",
		},
		{
			name:         "ErrPredictionCompleteMsgNotFound",
			err:          ErrPredictionCompleteMsgNotFound,
			expectedType: apperrors.ExecutionFailed,
			expectedMsg:  "당첨번호 예측 작업의 종료 상태를 확인할 수 없습니다",
		},
		{
			name:         "ErrResultFilePathNotFound",
			err:          ErrResultFilePathNotFound,
			expectedType: apperrors.ExecutionFailed,
			expectedMsg:  "당첨번호 예측 결과 파일의 경로 정보를 추출하는 데 실패했습니다",
		},
		{
			name:         "ErrAnalysisResultInvalid",
			err:          ErrAnalysisResultInvalid,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "당첨번호 예측 결과 파일의 형식이 유효하지 않거나 내용을 식별할 수 없습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.err)
			assert.Contains(t, tt.err.Error(), tt.expectedMsg)

			var appErr *apperrors.AppError
			if assert.True(t, errors.As(tt.err, &appErr)) {
				assert.Equal(t, tt.expectedType, appErr.Type())
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	// Common cause for wrapped errors
	cause := errors.New("original cause")

	tests := []struct {
		name         string
		constructor  func() error
		expectedType apperrors.ErrorType
		expectedMsg  string
		checkCause   bool
	}{
		{
			name: "newErrAppPathAbsFailed",
			constructor: func() error {
				return newErrAppPathAbsFailed(cause)
			},
			expectedType: apperrors.System,
			expectedMsg:  "app_path의 절대 경로 변환에 실패하였습니다",
			checkCause:   true,
		},
		{
			name: "newErrAppPathDirValidationFailed",
			constructor: func() error {
				return newErrAppPathDirValidationFailed(cause)
			},
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "app_path로 지정된 디렉터리 검증에 실패하였습니다",
			checkCause:   true,
		},
		{
			name: "newErrJarFileNotFound",
			constructor: func() error {
				return newErrJarFileNotFound(cause, "lotto.jar")
			},
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "로또 당첨번호 예측 프로그램(lotto.jar)을 찾을 수 없습니다",
			checkCause:   true,
		},
		{
			name: "newErrJavaNotFound",
			constructor: func() error {
				return newErrJavaNotFound(cause)
			},
			expectedType: apperrors.System,
			expectedMsg:  "Java 런타임(JRE) 환경을 찾을 수 없습니다",
			checkCause:   true,
		},
		{
			name: "newErrPredictionFailed",
			constructor: func() error {
				return newErrPredictionFailed(cause)
			},
			expectedType: apperrors.ExecutionFailed,
			expectedMsg:  "당첨번호 예측 프로세스 실행 중 오류가 발생하였습니다",
			checkCause:   true,
		},
		{
			name: "newErrResultFileAbsFailed",
			constructor: func() error {
				return newErrResultFileAbsFailed(cause)
			},
			expectedType: apperrors.System,
			expectedMsg:  "예측 결과 파일의 절대 경로를 확인(Resolve)하는 도중 시스템 오류가 발생했습니다",
			checkCause:   true,
		},
		{
			name: "newErrResultFileRelFailed",
			constructor: func() error {
				return newErrResultFileRelFailed(cause)
			},
			expectedType: apperrors.ExecutionFailed,
			expectedMsg:  "예측 결과 파일의 경로 유효성을 검증하는 도중 오류가 발생했습니다",
			checkCause:   true,
		},
		{
			name: "newErrPathTraversalDetected",
			constructor: func() error {
				return newErrPathTraversalDetected()
			},
			expectedType: apperrors.Forbidden,
			expectedMsg:  "보안 정책 위반: 허용된 경로 범위를 벗어난 파일 접근이 감지되었습니다",
			checkCause:   false, // This error does not wrap another error
		},
		{
			name: "newErrReadResultFileFailed",
			constructor: func() error {
				return newErrReadResultFileFailed(cause, "result.log")
			},
			expectedType: apperrors.System,
			expectedMsg:  "예측 결과 파일(result.log)을 읽는 도중 I/O 오류가 발생했습니다",
			checkCause:   true,
		},
		{
			name: "newErrParseResultFileFailed",
			constructor: func() error {
				return newErrParseResultFileFailed(cause, "result.log")
			},
			expectedType: apperrors.ParsingFailed,
			expectedMsg:  "예측 결과 파일(result.log)의 내용을 파싱하는 도중 오류가 발생했습니다",
			checkCause:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.constructor()

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedMsg)

			// Verify error type
			var appErr *apperrors.AppError
			if assert.True(t, errors.As(err, &appErr)) {
				assert.Equal(t, tt.expectedType, appErr.Type())
			}

			// Verify wrapped cause if applicable
			if tt.checkCause {
				assert.True(t, errors.Is(err, cause), "result error should wrap the original cause")
				assert.Equal(t, cause, errors.Unwrap(appErr), "unwrapped error should match cause")
			}
		})
	}
}
