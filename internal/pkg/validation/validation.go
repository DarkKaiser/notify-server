package validation

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/internal/pkg/log"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

var (
	// cronParser Cron 표현식 파서
	cronParser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	// urlRegex URL 유효성 검사 정규식
	// 형식: ^https?://[호스트](?::[포트])?(?:/[경로])*$
	//
	// 구성 요소:
	//   - 스키마: http 또는 https (필수)
	//   - 호스트: 다음 중 하나
	//     * 도메인명: 영문자, 숫자, 하이픈, 점으로 구성, 최소 2자 이상의 TLD 필요
	//       예: example.com, api.example.co.kr
	//     * localhost: 로컬 개발 환경 지원
	//     * IPv4 주소: 각 옥텟이 0-255 범위 (예: 192.168.1.1)
	//   - 포트: 선택적, 숫자로 구성 (예: :8080)
	//   - 경로: 선택적, 슬래시로 시작하는 경로 세그먼트 (예: /path/to/resource)
	//
	// 예제:
	//   - https://example.com
	//   - http://localhost:3000
	//   - https://192.168.1.1:8443/api
	urlRegex = regexp.MustCompile(`^https?://(?:[^:@/]+(?::[^@/]+)?@)?(?:[a-zA-Z0-9.-]+(?:\.[a-zA-Z]{2,})+|localhost|(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?))(?::\d+)?(?:[/?#].*)?$`)
)

// ValidateRobfigCronExpression Cron 표현식의 유효성을 검사합니다.
// robfig/cron 패키지를 사용하며, 초 단위를 포함한 7개 필드 형식을 지원합니다.
// 형식: 초 분 시 일 월 요일 (예: "0 */5 * * * *" - 5분마다)
func ValidateRobfigCronExpression(spec string) error {
	_, err := cronParser.Parse(spec)
	if err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("잘못된 Cron 표현식입니다: %s", spec))
	}
	return nil
}

// ValidateDuration duration 문자열의 유효성을 검사합니다.
func ValidateDuration(d string) error {
	_, err := time.ParseDuration(d)
	if err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("잘못된 duration 형식입니다: %s (예: 2s, 100ms, 1m)", d))
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

// ValidateFileExists 파일 존재 여부를 검사합니다
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func ValidateFileExists(path string, warnOnly bool) error {
	if path == "" {
		return nil // 빈 경로는 검사하지 않음
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			errMsg := apperrors.New(apperrors.NotFound, fmt.Sprintf("파일이 존재하지 않습니다: %s", path))
			if warnOnly {
				applog.WithComponentAndFields("validation", log.Fields{
					"file_path": path,
				}).Warn(errMsg.Error())
				return nil
			}
			return errMsg
		}
		return apperrors.Wrap(err, apperrors.Internal, fmt.Sprintf("파일 접근 오류: %s", path))
	}
	return nil
}

// ValidateURL URL 형식의 유효성을 검사합니다.
func ValidateURL(urlStr string) error {
	if urlStr == "" {
		return nil
	}

	// 1. 정규식으로 기본 형식 검사
	if !urlRegex.MatchString(urlStr) {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("잘못된 URL 형식입니다 (정규식 불일치): %s", urlStr))
	}

	// 2. url.Parse로 상세 파싱 검사
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("잘못된 URL 형식입니다 (URL 파싱 실패): %s", urlStr))
	}

	// Scheme 검증 (http 또는 https만 허용) - 정규식에서 이미 체크하지만 이중 확인
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("URL은 http 또는 https 스키마를 사용해야 합니다: %s", urlStr))
	}

	// Host 검증
	if parsedURL.Host == "" {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("URL에 호스트가 없습니다: %s", urlStr))
	}

	return nil
}

// ValidateCORSOrigin CORS Origin의 유효성을 검사합니다.
// Origin은 스키마(http/https), 도메인(또는 IP), 포트로만 구성되어야 하며, 경로(Path)나 쿼리 스트링을 포함할 수 없습니다.
// 또한 마지막에 슬래시(/)가 없어야 합니다.
func ValidateCORSOrigin(origin string) error {
	if origin == "*" {
		return nil
	}

	// 빈 문자열 또는 공백만 있는 경우 체크
	trimmedOrigin := strings.TrimSpace(origin)
	if trimmedOrigin == "" {
		return apperrors.New(apperrors.InvalidInput, "Origin은 빈 문자열일 수 없습니다")
	}

	if strings.HasSuffix(origin, "/") {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Origin은 슬래시(/)로 끝날 수 없습니다: %s", origin))
	}

	parsedURL, err := url.Parse(origin)
	if err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("잘못된 Origin 형식입니다: %s", origin))
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Origin은 http 또는 https 스키마를 사용해야 합니다: %s", origin))
	}

	if parsedURL.Path != "" && parsedURL.Path != "/" {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Origin에는 경로(Path)를 포함할 수 없습니다: %s", origin))
	}

	if parsedURL.RawQuery != "" {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Origin에는 쿼리 스트링을 포함할 수 없습니다: %s", origin))
	}

	// 호스트 검증 (정규식 사용)
	if !urlRegex.MatchString(origin) {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("잘못된 Origin 형식입니다 (호스트/포트 오류): %s", origin))
	}

	return nil
}

// ValidatePort TCP/UDP 네트워크 포트 번호의 유효성을 검사합니다.
// 유효 범위: 1-65535, 1024 미만 포트는 시스템 예약 포트로 경고를 출력합니다.
func ValidatePort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("포트 번호는 1-65535 범위여야 합니다 (입력값: %d)", port))
	}
	if port < 1024 {
		// 경고만 로그로 출력 (에러는 아님)
		applog.WithComponentAndFields("validation", log.Fields{
			"port": port,
		}).Warn("1-1023 포트는 시스템 예약 포트입니다. 권한이 필요할 수 있습니다")
	}
	return nil
}

// ValidateNoDuplicate 목록에 중복된 값이 없는지 검사합니다.
func ValidateNoDuplicate(list []string, value, valueType string) error {
	if slices.Contains(list, value) {
		return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s(%s)가 중복되었습니다", valueType, value))
	}
	return nil
}
