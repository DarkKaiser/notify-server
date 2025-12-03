package validation

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

// ValidateRobfigCronExpression Cron 표현식의 유효성을 검사합니다.
// robfig/cron 패키지를 사용하며, 초 단위를 포함한 7개 필드 형식을 지원합니다.
// 형식: 초 분 시 일 월 요일 (예: "0 */5 * * * *" - 5분마다)
func ValidateRobfigCronExpression(spec string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(spec)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("잘못된 Cron 표현식입니다: %s", spec))
	}
	return nil
}

// ValidatePort TCP/UDP 네트워크 포트 번호의 유효성을 검사합니다.
// 유효 범위: 1-65535, 1024 미만 포트는 시스템 예약 포트로 경고를 출력합니다.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("포트 번호는 1-65535 범위여야 합니다 (입력값: %d)", port))
	}
	if port < 1024 {
		// 경고만 로그로 출력 (에러는 아님)
		applog.WithComponentAndFields("validation", log.Fields{
			"port": port,
		}).Warn("1-1023 포트는 시스템 예약 포트입니다. 권한이 필요할 수 있습니다")
	}
	return nil
}

// ValidateDuration duration 문자열의 유효성을 검사합니다.
func ValidateDuration(d string) error {
	_, err := time.ParseDuration(d)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("잘못된 duration 형식입니다: %s (예: 2s, 100ms, 1m)", d))
	}
	return nil
}

// ValidateFileExistsOrURL 파일 경로 또는 URL의 유효성을 검사합니다.
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func ValidateFileExistsOrURL(path string, warnOnly bool) error {
	if path == "" {
		return nil
	}

	// URL 형식인지 확인
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return ValidateURL(path)
	}

	// 파일 경로로 검증
	return ValidateFileExists(path, warnOnly)
}

// ValidateFileExists 파일 존재 여부를 검사합니다 (선택적).
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func ValidateFileExists(path string, warnOnly bool) error {
	if path == "" {
		return nil // 빈 경로는 검사하지 않음
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			errMsg := apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("파일이 존재하지 않습니다: %s", path))
			if warnOnly {
				applog.WithComponentAndFields("validation", log.Fields{
					"file_path": path,
				}).Warn(errMsg.Error())
				return nil
			}
			return errMsg
		}
		return apperrors.Wrap(err, apperrors.ErrInternal, fmt.Sprintf("파일 접근 오류: %s", path))
	}
	return nil
}

// ValidateURL URL 형식의 유효성을 검사합니다.
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return nil
	}

	// URL 파싱
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("잘못된 URL 형식입니다: %s", urlStr))
	}

	// Scheme 검증 (http 또는 https만 허용)
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("URL은 http 또는 https 스키마를 사용해야 합니다: %s", urlStr))
	}

	// Host 검증
	if parsedURL.Host == "" {
		return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("URL에 호스트가 없습니다: %s", urlStr))
	}

	return nil
}
