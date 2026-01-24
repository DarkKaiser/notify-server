package telegram

import (
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/service/contract"
	"github.com/darkkaiser/notify-server/internal/service/notification/notifier"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Unit Tests for Message Builder & Helpers
// =============================================================================

func TestFormatElapsedTime(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "0초",
			duration: 0,
			expected: " (0초 지남)",
		},
		{
			name:     "초 단위만",
			duration: 30 * time.Second,
			expected: " (30초 지남)",
		},
		{
			name:     "분 단위만 (0초 생략)",
			duration: 5 * time.Minute,
			expected: " (5분 지남)",
		},
		{
			name:     "시 단위만 (0분 0초 생략)",
			duration: 2 * time.Hour,
			expected: " (2시간 지남)",
		},
		{
			name:     "분 + 초",
			duration: 5*time.Minute + 30*time.Second,
			expected: " (5분 30초 지남)",
		},
		{
			name:     "시 + 분 (0초 생략)",
			duration: 2*time.Hour + 30*time.Minute,
			expected: " (2시간 30분 지남)",
		},
		{
			name:     "시 + 분 + 초 (전체)",
			duration: 1*time.Hour + 30*time.Minute + 10*time.Second, // 예시와 동일
			expected: " (1시간 30분 10초 지남)",
		},
		{
			name:     "복잡한 시간 (3661초)",
			duration: 3661 * time.Second,
			expected: " (1시간 1분 1초 지남)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatElapsedTime(tt.duration)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Tests for telegramNotifier methods
// =============================================================================

// setupNotifierForBuilder 테스트용 Notifier 인스턴스를 생성하는 헬퍼 함수
func setupNotifierForBuilder() *telegramNotifier {
	return &telegramNotifier{
		Base:              notifier.NewBase("test-id", true, 100, 10*time.Second),
		botCommandsByTask: make(map[contract.TaskID]map[contract.TaskCommandID]botCommand),
	}
}

func TestTelegramNotifier_WithTitle(t *testing.T) {
	n := setupNotifierForBuilder()

	// 봇 명령어 맵 설정 (ID 조회 테스트용)
	taskID := contract.TaskID("task-1")
	cmdID := contract.TaskCommandID("cmd-1")
	n.botCommandsByTask[taskID] = map[contract.TaskCommandID]botCommand{
		cmdID: {title: "Bot Command Title"},
	}

	tests := []struct {
		name         string
		notification contract.Notification
		message      string
		expected     string // expected substring or full match
	}{
		{
			name: "Basic Title",
			notification: contract.Notification{
				Title: "My Title",
			},
			message:  "Hello",
			expected: "<b>【 My Title 】</b>\n\nHello",
		},
		{
			name: "HTML Special Characters Escaping",
			notification: contract.Notification{
				Title: "<script>alert('xss')</script>",
			},
			message:  "Content",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;", // 제목은 이스케이프 되어야 함
		},
		{
			name: "Long Title Truncation",
			notification: contract.Notification{
				Title: strings.Repeat("A", 300), // 300자 > 200자 제한
			},
			message:  "Content",
			expected: strings.Repeat("A", maxTitleLength), // 200자까지만 포함되어야 함
		},
		{
			name: "No Title - Fallback to Bot Command Lookup",
			notification: contract.Notification{
				Title:     "",
				TaskID:    taskID,
				CommandID: cmdID,
			},
			message:  "Content",
			expected: "<b>【 Bot Command Title 】</b>",
		},
		{
			name: "No Title & No Lookup Match",
			notification: contract.Notification{
				Title:     "",
				TaskID:    "unknown-task",
				CommandID: "unknown-cmd",
			},
			message:  "Original Message",
			expected: "Original Message", // 변경 없음
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.withTitle(&tt.notification, tt.message)
			assert.Contains(t, result, tt.expected)

			// Truncation 테스트의 경우 길이 확인도 추가 수행
			if tt.name == "Long Title Truncation" {
				// Format overhead 제외한 제목 부분 길이 확인 필요하지만,
				// 여기서는 결과 문자열에 Truncated Title이 포함되어 있는지로 충분
				assert.NotContains(t, result, strings.Repeat("A", maxTitleLength+1))
			}
		})
	}
}

func TestTelegramNotifier_WithCancelCommand(t *testing.T) {
	n := setupNotifierForBuilder()

	tests := []struct {
		name         string
		notification contract.Notification
		message      string
		expected     string
		shouldModify bool
	}{
		{
			name: "Cancelable with InstanceID",
			notification: contract.Notification{
				Cancelable: true,
				InstanceID: "inst-123",
			},
			message:      "Job Running",
			expected:     string(botCommandPrefix) + "cancel" + string(botCommandSeparator) + "inst-123",
			shouldModify: true,
		},
		{
			name: "Cancelable but Empty InstanceID",
			notification: contract.Notification{
				Cancelable: true,
				InstanceID: "",
			},
			message:      "Job Running",
			shouldModify: false,
		},
		{
			name: "Not Cancelable",
			notification: contract.Notification{
				Cancelable: false,
				InstanceID: "inst-123",
			},
			message:      "Job Done",
			shouldModify: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.withCancelCommand(&tt.notification, tt.message)
			if tt.shouldModify {
				assert.Contains(t, result, tt.expected)
				assert.Contains(t, result, "\n\n") // 빈 줄 확인
			} else {
				assert.Equal(t, tt.message, result)
			}
		})
	}
}

func TestTelegramNotifier_WithElapsedTime(t *testing.T) {
	n := setupNotifierForBuilder()

	tests := []struct {
		name         string
		notification contract.Notification
		message      string
		expected     string
	}{
		{
			name: "Positive Elapsed Time",
			notification: contract.Notification{
				ElapsedTime: 10 * time.Second,
			},
			message:  "Job Finished",
			expected: "Job Finished (10초 지남)",
		},
		{
			name: "Zero Elapsed Time",
			notification: contract.Notification{
				ElapsedTime: 0,
			},
			message:  "Job Started",
			expected: "Job Started", // 변경 없음
		},
		{
			name: "Negative Elapsed Time",
			notification: contract.Notification{
				ElapsedTime: -5 * time.Minute,
			},
			message:  "Weird Job",
			expected: "Weird Job", // 변경 없음
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := n.withElapsedTime(&tt.notification, tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTelegramNotifier_BuildEnrichMessage_Integration(t *testing.T) {
	n := setupNotifierForBuilder()

	// 통합 테스트: 모든 요소가 포함된 경우
	notification := &contract.Notification{
		Title:         "Full Report",
		Message:       "Main Content",
		Cancelable:    true,
		InstanceID:    "inst-999",
		ElapsedTime:   1 * time.Hour,
		ErrorOccurred: true,
	}

	result := n.buildEnrichMessage(notification)

	// 1. Title
	assert.Contains(t, result, "<b>【 Full Report 】</b>")
	// 2. Original Message
	assert.Contains(t, result, "Main Content")
	// 3. Cancel Command
	assert.Contains(t, result, "/cancel_inst-999")
	// 4. Elapsed Time
	assert.Contains(t, result, "(1시간 지남)")
	// 5. Error Warning
	assert.Contains(t, result, "*** 오류가 발생하였습니다. ***")

	// 순서 확인 (대략적인 위치 비교)
	titleIdx := strings.Index(result, "Full Report")
	msgIdx := strings.Index(result, "Main Content")
	cancelIdx := strings.Index(result, "cancel")
	errorIdx := strings.Index(result, "오류가 발생하였습니다")

	assert.True(t, titleIdx < msgIdx, "Title should appear before message")
	assert.True(t, msgIdx < cancelIdx, "Message should appear before cancel command")
	// 참고: ElapsedTime은 메인 메시지 바로 뒤에 붙음 ("Main Content (1시간 지남)")
	// Error는 맨 마지막에 Formatter로 감싸짐
	assert.True(t, cancelIdx < errorIdx, "Cancel command should appear before error warning")
}

func TestTelegramNotifier_BuildEnrichMessage_Minimal(t *testing.T) {
	n := setupNotifierForBuilder()

	// 최소 테스트: 아무런 메타데이터 없음
	notification := &contract.Notification{
		Message: "Simple Message",
	}

	result := n.buildEnrichMessage(notification)
	assert.Equal(t, "Simple Message", result)
}
