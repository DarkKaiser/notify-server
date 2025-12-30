package validation

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	log "github.com/sirupsen/logrus"
)

var (
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
