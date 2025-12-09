package validation

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRobfigCronExpression(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		wantErr bool
	}{
		{"유효한 Cron (초 포함)", "0 */5 * * * *", false},
		{"유효한 Cron (매분)", "0 * * * * *", false},
		{"유효한 Cron (매일 9시)", "0 0 9 * * *", false},
		{"잘못된 Cron (필드 부족)", "* * *", true},
		{"잘못된 Cron (범위 초과)", "70 * * * * *", true},
		{"빈 문자열", "", true},
		{"잘못된 형식", "invalid cron", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRobfigCronExpression(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRobfigCronExpression() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    int
		wantErr bool
	}{
		{"유효한 포트", 8080, false},
		{"최소 포트", 1, false},
		{"최대 포트", 65535, false},
		{"일반적인 포트", 2443, false},
		{"0 포트", 0, true},
		{"음수 포트", -1, true},
		{"범위 초과", 65536, true},
		{"범위 초과 (큰 값)", 100000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		wantErr  bool
	}{
		{"초 단위", "2s", false},
		{"밀리초 단위", "100ms", false},
		{"분 단위", "1m", false},
		{"시간 단위", "1h", false},
		{"복합 단위", "1m30s", false},
		{"잘못된 형식", "2 seconds", true},
		{"빈 문자열", "", true},
		{"숫자만", "123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDuration(tt.duration)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDuration() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileExists(t *testing.T) {
	// 테스트용 임시 파일 생성
	tmpFile, err := os.CreateTemp("", "test-*.txt")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"존재하는 파일 (에러 모드)", tmpFile.Name(), false, false},
		{"존재하는 파일 (경고 모드)", tmpFile.Name(), true, false},
		{"존재하지 않는 파일 (에러 모드)", "/nonexistent/file.txt", false, true},
		{"존재하지 않는 파일 (경고 모드)", "/nonexistent/file.txt", true, false},
		{"빈 경로 (에러 모드)", "", false, false},
		{"빈 경로 (경고 모드)", "", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExists(tt.path, tt.warnOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileExists() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		urlStr  string
		wantErr bool
	}{
		{"유효한 HTTPS URL", "https://example.com/cert.pem", false},
		{"유효한 HTTP URL", "http://example.com/key.pem", false},
		{"유효한 URL (포트 포함)", "https://example.com:8443/cert.pem", false},
		{"유효한 URL (경로 포함)", "https://example.com/path/to/cert.pem", false},
		{"잘못된 스키마 (ftp)", "ftp://example.com/cert.pem", true},
		{"잘못된 스키마 (file)", "file:///path/to/cert.pem", true},
		{"호스트 없음", "https:///cert.pem", true},
		{"잘못된 URL 형식", "not-a-url", true},
		{"빈 문자열", "", false},
		{"퓨니코드 도메인", "https://xn--989a11j08n21d.com", false}, // 한국.com
		{"URL 인코딩된 경로", "https://example.com/path%20with%20spaces", false},
		{"IPv6 주소 (현재 미지원)", "http://[::1]:8080", true},
		{"URL with user info", "https://user:pass@example.com", false},
		{"URL with query params", "https://example.com?q=golang", false},
		{"URL with fragment", "https://example.com#section", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.urlStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFileExistsOrURL(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		warnOnly bool
		wantErr  bool
	}{
		{"유효한 HTTPS URL", "https://example.com/cert.pem", false, false},
		{"유효한 HTTP URL", "http://example.com/key.pem", false, false},
		{"잘못된 URL (ftp)", "ftp://example.com/cert.pem", false, true},
		{"파일 경로 (존재하지 않음, 에러 모드)", "/nonexistent/cert.pem", false, true},
		{"파일 경로 (존재하지 않음, 경고 모드)", "/nonexistent/cert.pem", true, false},
		{"빈 문자열", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFileExistsOrURL(tt.path, tt.warnOnly)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFileExistsOrURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateNoDuplicate(t *testing.T) {
	tests := []struct {
		name      string
		list      []string
		value     string
		valueType string
		wantErr   bool
	}{
		{"빈 목록에 추가", []string{}, "id1", "TestID", false},
		{"중복되지 않은 ID 추가", []string{"id1", "id2"}, "id3", "TestID", false},
		{"중복된 ID 추가", []string{"id1", "id2", "id3"}, "id2", "TestID", true},
		{"첫 번째 ID와 중복", []string{"id1", "id2"}, "id1", "TaskID", true},
		{"마지막 ID와 중복", []string{"id1", "id2", "id3"}, "id3", "CommandID", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNoDuplicate(tt.list, tt.value, tt.valueType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNoDuplicate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErr {
				// 에러 메시지에 valueType과 value가 포함되어 있는지 확인
				if !assert.Contains(t, err.Error(), tt.valueType) {
					t.Errorf("에러 메시지에 valueType(%s)이 포함되어야 합니다", tt.valueType)
				}
				if !assert.Contains(t, err.Error(), tt.value) {
					t.Errorf("에러 메시지에 value(%s)가 포함되어야 합니다", tt.value)
				}
			}
		})
	}
}

func TestValidateCORSOrigin(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		wantErr bool
	}{
		// 유효한 케이스
		{"와일드카드", "*", false},
		{"유효한 HTTP Origin", "http://localhost:3000", false},
		{"유효한 HTTPS Origin", "https://example.com", false},
		{"유효한 Origin (포트 포함)", "https://example.com:8443", false},
		{"유효한 Origin (서브도메인)", "https://api.example.com", false},
		{"유효한 Origin (IP 주소)", "http://192.168.1.1:8080", false},
		{"유효한 Origin (localhost)", "http://localhost", false},

		// 잘못된 케이스 - 슬래시로 끝남
		{"슬래시로 끝나는 Origin", "https://example.com/", true},
		{"슬래시로 끝나는 Origin (포트 포함)", "https://example.com:8443/", true},

		// 잘못된 케이스 - 경로 포함
		{"경로 포함", "https://example.com/path", true},
		{"경로 포함 (여러 레벨)", "https://example.com/path/to/resource", true},

		// 잘못된 케이스 - 쿼리 스트링 포함
		{"쿼리 스트링 포함", "https://example.com?query=1", true},
		{"쿼리 스트링과 경로 포함", "https://example.com/path?query=1", true},

		// 잘못된 케이스 - 빈 문자열 및 공백
		{"빈 문자열", "", true},
		{"공백만 있는 문자열", "   ", true},

		// 잘못된 케이스 - 잘못된 스키마
		{"잘못된 스키마 (ftp)", "ftp://example.com", true},
		{"잘못된 스키마 (file)", "file:///path/to/file", true},
		{"스키마 없음", "example.com", true},
		{"스키마 없음 (포트 포함)", "example.com:8080", true},

		// 에지 케이스
		{"포트만 있는 localhost", "http://localhost:3000", false},
		{"IPv4 주소", "http://127.0.0.1", false},
		{"IPv4 주소 (포트 포함)", "http://127.0.0.1:8080", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCORSOrigin(tt.origin)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCORSOrigin() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCORSOrigin_ErrorMessages(t *testing.T) {
	// 에러 메시지 검증 테스트
	tests := []struct {
		name           string
		origin         string
		expectedErrMsg string
	}{
		{
			name:           "슬래시로 끝나는 경우",
			origin:         "https://example.com/",
			expectedErrMsg: "슬래시(/)로 끝날 수 없습니다",
		},
		{
			name:           "경로 포함",
			origin:         "https://example.com/path",
			expectedErrMsg: "경로(Path)를 포함할 수 없습니다",
		},
		{
			name:           "쿼리 스트링 포함",
			origin:         "https://example.com?query=1",
			expectedErrMsg: "쿼리 스트링을 포함할 수 없습니다",
		},
		{
			name:           "잘못된 스키마",
			origin:         "ftp://example.com",
			expectedErrMsg: "http 또는 https 스키마를 사용해야 합니다",
		},
		{
			name:           "빈 문자열",
			origin:         "   ",
			expectedErrMsg: "빈 문자열일 수 없습니다",
		},
		{
			name:           "IPv6 주소 (현재 미지원)",
			origin:         "http://[::1]",
			expectedErrMsg: "잘못된 Origin 형식입니다",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCORSOrigin(tt.origin)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErrMsg)
		})
	}
}
