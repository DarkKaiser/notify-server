package lotto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractLogFilePath(t *testing.T) {
	tests := []struct {
		name          string
		cmdOutput     string
		expectedPath  string
		expectError   bool
		errorContains string
	}{
		{
			name:         "Success",
			cmdOutput:    "Some Logs... \r\n로또 당첨번호 예측작업이 종료되었습니다. 5개의 대상 당첨번호가 추출되었습니다.(경로:/temp/result.log)",
			expectedPath: "/temp/result.log",
			expectError:  false,
		},
		{
			name:          "Failure_NoCompletionMessage",
			cmdOutput:     "Processing...",
			expectedPath:  "",
			expectError:   true,
			errorContains: "정상적으로 완료되었는지 확인할 수 없습니다",
		},
		{
			name:          "Failure_NoPathCaptured",
			cmdOutput:     "로또 당첨번호 예측작업이 종료되었습니다. 0개의 대상 당첨번호가 추출되었습니다.()",
			expectedPath:  "",
			expectError:   true,
			errorContains: "파일의 경로를 찾을 수 없습니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := extractLogFilePath(tt.cmdOutput)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedPath, path)
			}
		})
	}
}

func TestParseAnalysisResult(t *testing.T) {
	validLogContent := `
	[INFO] 2024-01-01 ...
	- 분석결과
	당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.
	당첨번호1: 1, 2, 3, 4, 5, 6
	당첨번호2: 7, 8, 9, 10, 11, 12
	당첨번호3: 13, 14, 15, 16, 17, 18
	당첨번호4: 19, 20, 21, 22, 23, 24
	당첨번호5: 25, 26, 27, 28, 29, 30
	`

	tests := []struct {
		name             string
		logContent       string
		expectError      bool
		expectedContains []string
	}{
		{
			name:        "Success",
			logContent:  validLogContent,
			expectError: false,
			expectedContains: []string{
				"당첨 확률이 높은 당첨번호 목록(5개)중에서 5개의 당첨번호가 추출되었습니다.",
				"• 당첨번호1: 1, 2, 3, 4, 5, 6",
				"• 당첨번호5: 25, 26, 27, 28, 29, 30",
			},
		},
		{
			name:        "Failure_NoResultMarker",
			logContent:  "[INFO] Just logs...",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseAnalysisResult(tt.logContent)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				for _, substr := range tt.expectedContains {
					assert.Contains(t, result, substr)
				}
			}
		})
	}
}
