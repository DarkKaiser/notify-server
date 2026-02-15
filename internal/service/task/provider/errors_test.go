package provider

import (
	"fmt"
	"testing"

	"github.com/darkkaiser/notify-server/internal/pkg/errors"
	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Sentinel Error Tests
// =============================================================================

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name         string
		actualErr    error
		expectedType errors.ErrorType
		expectedMsg  string
	}{
		// 리소스 조회 실패
		{
			name:         "ErrTaskNotFound",
			actualErr:    ErrTaskNotFound,
			expectedType: apperrors.NotFound,
			expectedMsg:  "해당 작업을 찾을 수 없습니다",
		},
		{
			name:         "ErrCommandNotFound",
			actualErr:    ErrCommandNotFound,
			expectedType: apperrors.NotFound,
			expectedMsg:  "해당 명령을 찾을 수 없습니다",
		},
		// 기능 미지원
		{
			name:         "ErrTaskNotSupported",
			actualErr:    ErrTaskNotSupported,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "지원하지 않는 작업입니다",
		},
		{
			name:         "ErrCommandNotSupported",
			actualErr:    ErrCommandNotSupported,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "지원하지 않는 명령입니다",
		},
		// 식별자 중복
		{
			name:         "ErrDuplicateTaskID",
			actualErr:    ErrDuplicateTaskID,
			expectedType: apperrors.Conflict,
			expectedMsg:  "중복된 TaskID입니다",
		},
		{
			name:         "ErrDuplicateCommandID",
			actualErr:    ErrDuplicateCommandID,
			expectedType: apperrors.Conflict,
			expectedMsg:  "중복된 CommandID입니다",
		},
		// 설정 검증
		{
			name:         "ErrTaskConfigNil",
			actualErr:    ErrTaskConfigNil,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "Task 설정은 필수값입니다",
		},
		{
			name:         "ErrCommandConfigNil",
			actualErr:    ErrCommandConfigNil,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "Command 설정은 nil일 수 없습니다",
		},
		{
			name:         "ErrCommandConfigsEmpty",
			actualErr:    ErrCommandConfigsEmpty,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "최소 하나 이상의 Command 설정이 필요합니다",
		},
		{
			name:         "ErrNewTaskNil",
			actualErr:    ErrNewTaskNil,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "NewTask 팩토리 함수는 필수값입니다",
		},
		{
			name:         "ErrNewSnapshotNil",
			actualErr:    ErrNewSnapshotNil,
			expectedType: apperrors.InvalidInput,
			expectedMsg:  "NewSnapshot 팩토리 함수는 필수값입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.actualErr, "Error should not be nil")
			assert.True(t, errors.Is(tt.actualErr, tt.expectedType), "Type mismatch")
			assert.Contains(t, tt.actualErr.Error(), tt.expectedMsg)
		})
	}
}

// =============================================================================
// Constructor Error Tests
// =============================================================================

func TestConstructorErrors(t *testing.T) {
	taskID := contract.TaskID("TEST_TASK")
	cmdID := contract.TaskCommandID("TEST_CMD")
	causeErr := fmt.Errorf("underlying error")

	t.Run("newErrTaskNotFound", func(t *testing.T) {
		err := newErrTaskNotFound(taskID)
		assert.True(t, errors.Is(err, apperrors.NotFound))
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), "해당 작업을 찾을 수 없습니다")
	})

	t.Run("newErrCommandNotFound", func(t *testing.T) {
		err := newErrCommandNotFound(taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.NotFound))
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
		assert.Contains(t, err.Error(), "해당 명령을 찾을 수 없습니다")
	})

	t.Run("NewErrTaskNotSupported", func(t *testing.T) {
		err := NewErrTaskNotSupported(taskID)
		assert.True(t, errors.Is(err, apperrors.InvalidInput))
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), "지원하지 않는 작업입니다")
	})

	t.Run("NewErrCommandNotSupported", func(t *testing.T) {
		t.Run("Without supported commands", func(t *testing.T) {
			err := NewErrCommandNotSupported(cmdID, nil)
			assert.True(t, errors.Is(err, apperrors.InvalidInput))
			assert.Contains(t, err.Error(), string(cmdID))
			assert.Contains(t, err.Error(), "지원하지 않는 명령입니다")
			assert.NotContains(t, err.Error(), "사용 가능한 명령")
		})

		t.Run("With supported commands", func(t *testing.T) {
			supported := []contract.TaskCommandID{"CMD_1", "CMD_2"}
			err := NewErrCommandNotSupported(cmdID, supported)
			assert.True(t, errors.Is(err, apperrors.InvalidInput))
			assert.Contains(t, err.Error(), string(cmdID))
			assert.Contains(t, err.Error(), "사용 가능한 명령: CMD_1, CMD_2")
		})
	})

	t.Run("newErrInvalidTaskID", func(t *testing.T) {
		err := newErrInvalidTaskID(causeErr, taskID)
		assert.True(t, errors.Is(err, apperrors.InvalidInput))
		assert.ErrorIs(t, err, causeErr)
		assert.Contains(t, err.Error(), "유효하지 않은 TaskID입니다")
	})

	t.Run("newErrDuplicateTaskID", func(t *testing.T) {
		err := newErrDuplicateTaskID(taskID)
		assert.True(t, errors.Is(err, apperrors.Conflict))
		assert.Contains(t, err.Error(), "중복된 TaskID입니다")
		assert.Contains(t, err.Error(), string(taskID))
	})

	t.Run("newErrInvalidCommandID", func(t *testing.T) {
		err := newErrInvalidCommandID(causeErr, cmdID)
		assert.True(t, errors.Is(err, apperrors.InvalidInput))
		assert.ErrorIs(t, err, causeErr)
		assert.Contains(t, err.Error(), "유효하지 않은 CommandID입니다")
	})

	t.Run("newErrDuplicateCommandID", func(t *testing.T) {
		err := newErrDuplicateCommandID(cmdID)
		assert.True(t, errors.Is(err, apperrors.Conflict))
		assert.Contains(t, err.Error(), "중복된 CommandID입니다")
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrSnapshotFactoryReturnedNil", func(t *testing.T) {
		err := newErrSnapshotFactoryReturnedNil(cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "NewSnapshot 팩토리 함수가 nil을 반환했습니다")
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrTaskSettingsProcessingFailed", func(t *testing.T) {
		err := newErrTaskSettingsProcessingFailed(causeErr, taskID)
		assert.True(t, errors.Is(err, apperrors.InvalidInput))
		assert.ErrorIs(t, err, causeErr)
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
		assert.Contains(t, err.Error(), string(taskID))
	})

	t.Run("newErrCommandSettingsProcessingFailed", func(t *testing.T) {
		err := newErrCommandSettingsProcessingFailed(causeErr, taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.InvalidInput))
		assert.ErrorIs(t, err, causeErr)
		assert.Contains(t, err.Error(), "추가 설정 정보 처리에 실패했습니다")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrExecuteFuncNotInitialized", func(t *testing.T) {
		err := newErrExecuteFuncNotInitialized(taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "Execute()가 초기화되지 않았습니다")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrScraperNotInitialized", func(t *testing.T) {
		err := newErrScraperNotInitialized(taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "Scraper가 초기화되지 않았습니다")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrStorageNotInitialized", func(t *testing.T) {
		err := newErrStorageNotInitialized(taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "Storage가 초기화되지 않았습니다")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrSnapshotCreationFailed", func(t *testing.T) {
		err := newErrSnapshotCreationFailed(taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "작업 결과 객체(Snapshot) 생성에 실패했습니다 (nil 반환)")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("newErrSnapshotLoadingFailed", func(t *testing.T) {
		err := newErrSnapshotLoadingFailed(causeErr, taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.ErrorIs(t, err, causeErr)
		assert.Contains(t, err.Error(), "이전 작업 결과(Snapshot) 로딩 중 Storage 오류 발생")
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})

	t.Run("NewErrTypeAssertionFailed", func(t *testing.T) {
		expected := 10
		got := "string"
		err := NewErrTypeAssertionFailed(expected, got)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "snapshot의 타입 단언에 실패하였습니다")
		assert.Contains(t, err.Error(), "expected: int")
		assert.Contains(t, err.Error(), "got: string")
	})

	t.Run("newErrRuntimePanic", func(t *testing.T) {
		panicVal := "something went wrong"
		err := newErrRuntimePanic(panicVal, taskID, cmdID)
		assert.True(t, errors.Is(err, apperrors.Internal))
		assert.Contains(t, err.Error(), "Task 실행 중 런타임 패닉(Panic) 발생")
		assert.Contains(t, err.Error(), panicVal)
		assert.Contains(t, err.Error(), string(taskID))
		assert.Contains(t, err.Error(), string(cmdID))
	})
}
