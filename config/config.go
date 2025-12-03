package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	applog "github.com/darkkaiser/notify-server/log"
	apperrors "github.com/darkkaiser/notify-server/pkg/errors"
	"github.com/darkkaiser/notify-server/utils"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
)

const (
	AppName string = "notify-server"

	AppConfigFileName = AppName + ".json"

	// DefaultMaxRetries HTTP 요청 실패 시 최대 재시도 횟수 기본값
	DefaultMaxRetries = 3
	// DefaultRetryDelay 재시도 사이의 대기 시간 기본값
	DefaultRetryDelay = "2s"
)

// HTTPRetryConfig HTTP 재시도 설정 구조체
type HTTPRetryConfig struct {
	MaxRetries int    `json:"max_retries"`
	RetryDelay string `json:"retry_delay"`
}

// NotifierConfig 알림 설정 구조체
type NotifierConfig struct {
	DefaultNotifierID string           `json:"default_notifier_id"`
	Telegrams         []TelegramConfig `json:"telegrams"`
}

// TelegramConfig 텔레그램 알림 설정 구조체
type TelegramConfig struct {
	ID       string `json:"id"`
	BotToken string `json:"bot_token"`
	ChatID   int64  `json:"chat_id"`
}

type TaskConfig struct {
	ID       string                 `json:"id"`
	Title    string                 `json:"title"`
	Commands []TaskCommandConfig    `json:"commands"`
	Data     map[string]interface{} `json:"data"`
}

type TaskCommandConfig struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Scheduler   struct {
		Runnable bool   `json:"runnable"`
		TimeSpec string `json:"time_spec"`
	} `json:"scheduler"`
	Notifier struct {
		Usable bool `json:"usable"`
	} `json:"notifier"`
	DefaultNotifierID string                 `json:"default_notifier_id"`
	Data              map[string]interface{} `json:"data"`
}

// NotifyAPIConfig 알림 API 설정 구조체
type NotifyAPIConfig struct {
	WS           WSConfig            `json:"ws"`
	Applications []ApplicationConfig `json:"applications"`
}

// WSConfig 웹서버 설정 구조체
type WSConfig struct {
	TLSServer   bool   `json:"tls_server"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	ListenPort  int    `json:"listen_port"`
}

// ApplicationConfig 애플리케이션 설정 구조체
type ApplicationConfig struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	DefaultNotifierID string `json:"default_notifier_id"`
	AppKey            string `json:"app_key"`
}

// Convert JSON to Go struct : https://mholt.github.io/json-to-go/
type AppConfig struct {
	Debug     bool            `json:"debug"`
	HTTPRetry HTTPRetryConfig `json:"http_retry"`
	Notifiers NotifierConfig  `json:"notifiers"`
	Tasks     []TaskConfig    `json:"tasks"`
	NotifyAPI NotifyAPIConfig `json:"notify_api"`
}

func InitAppConfig() (*AppConfig, error) {
	return InitAppConfigWithFile(AppConfigFileName)
}

// InitAppConfigWithFile 지정된 파일에서 설정을 로드합니다.
// 이 함수는 테스트에서 사용할 수 있도록 파일명을 인자로 받습니다.
func InitAppConfigWithFile(filename string) (*AppConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var appConfig AppConfig
	err = json.Unmarshal(data, &appConfig)
	if err != nil {
		return nil, err
	}

	// HTTP Retry 설정 기본값 적용
	if appConfig.HTTPRetry.MaxRetries == 0 {
		appConfig.HTTPRetry.MaxRetries = DefaultMaxRetries
	}
	if appConfig.HTTPRetry.RetryDelay == "" {
		appConfig.HTTPRetry.RetryDelay = DefaultRetryDelay
	}

	//
	// 파일 내용에 대해 유효성 검사를 한다.
	//
	if err := appConfig.Validate(); err != nil {
		return nil, apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("%s 파일의 내용이 유효하지 않습니다", filename))
	}

	return &appConfig, nil
}

// validateCronExpression Cron 표현식의 유효성을 검사합니다.
func validateCronExpression(spec string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(spec)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("잘못된 Cron 표현식입니다: %s", spec))
	}
	return nil
}

// validatePort 포트 번호의 유효성을 검사합니다.
func validatePort(port int) error {
	if port < 1 || port > 65535 {
		return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("포트 번호는 1-65535 범위여야 합니다 (입력값: %d)", port))
	}
	if port < 1024 {
		// 경고만 로그로 출력 (에러는 아님)
		applog.WithComponentAndFields("config", log.Fields{
			"port": port,
		}).Warn("1-1023 포트는 시스템 예약 포트입니다. 권한이 필요할 수 있습니다")
	}
	return nil
}

// validateDuration duration 문자열의 유효성을 검사합니다.
func validateDuration(d string) error {
	_, err := time.ParseDuration(d)
	if err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("잘못된 duration 형식입니다: %s (예: 2s, 100ms, 1m)", d))
	}
	return nil
}

// validateFileExists 파일 존재 여부를 검사합니다 (선택적).
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func validateFileExists(path string, warnOnly bool) error {
	if path == "" {
		return nil // 빈 경로는 검사하지 않음
	}

	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			errMsg := apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("파일이 존재하지 않습니다: %s", path))
			if warnOnly {
				applog.WithComponentAndFields("config", log.Fields{
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

// validateURL URL 형식의 유효성을 검사합니다.
func validateURL(urlStr string) error {
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

// validateFileOrURL 파일 경로 또는 URL의 유효성을 검사합니다.
// warnOnly가 true면 경고만 출력하고 에러는 반환하지 않습니다.
func validateFileOrURL(path string, warnOnly bool) error {
	if path == "" {
		return nil
	}

	// URL 형식인지 확인
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return validateURL(path)
	}

	// 파일 경로로 검증
	return validateFileExists(path, warnOnly)
}

// Validate AppConfig의 유효성을 검사합니다.
func (c *AppConfig) Validate() error {
	// HTTP Retry 설정 검증
	if err := validateDuration(c.HTTPRetry.RetryDelay); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, "HTTP Retry 설정 오류")
	}

	// Notifiers 유효성 검사
	notifierIDs, err := c.Notifiers.Validate()
	if err != nil {
		return err
	}

	// Tasks 유효성 검사
	if err := c.validateTasks(notifierIDs); err != nil {
		return err
	}

	// NotifyAPI 유효성 검사
	if err := c.NotifyAPI.Validate(notifierIDs); err != nil {
		return err
	}

	return nil
}

// validateTasks Task 설정의 유효성을 검사합니다.
func (c *AppConfig) validateTasks(notifierIDs []string) error {
	var taskIDs []string
	for _, t := range c.Tasks {
		if utils.Contains(taskIDs, t.ID) {
			return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("TaskID(%s)가 중복되었습니다", t.ID))
		}
		taskIDs = append(taskIDs, t.ID)

		var commandIDs []string
		for _, cmd := range t.Commands {
			if utils.Contains(commandIDs, cmd.ID) {
				return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("CommandID(%s)가 중복되었습니다", cmd.ID))
			}
			commandIDs = append(commandIDs, cmd.ID)

			if !utils.Contains(notifierIDs, cmd.DefaultNotifierID) {
				return apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s::%s Task의 기본 NotifierID(%s)가 존재하지 않습니다", t.ID, cmd.ID, cmd.DefaultNotifierID))
			}

			// Cron 표현식 검증 (Scheduler가 활성화된 경우)
			if cmd.Scheduler.Runnable {
				if err := validateCronExpression(cmd.Scheduler.TimeSpec); err != nil {
					return apperrors.Wrap(err, apperrors.ErrInvalidInput, fmt.Sprintf("%s::%s Task의 Scheduler 설정 오류", t.ID, cmd.ID))
				}
			}
		}
	}
	return nil
}

// Validate NotifierConfig의 유효성을 검사합니다.
// 유효한 NotifierID 목록을 반환합니다.
func (c *NotifierConfig) Validate() ([]string, error) {
	var notifierIDs []string
	for _, telegram := range c.Telegrams {
		if utils.Contains(notifierIDs, telegram.ID) {
			return nil, apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("NotifierID(%s)가 중복되었습니다", telegram.ID))
		}
		notifierIDs = append(notifierIDs, telegram.ID)
	}

	if !utils.Contains(notifierIDs, c.DefaultNotifierID) {
		return nil, apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("전체 NotifierID 목록에서 기본 NotifierID(%s)가 존재하지 않습니다", c.DefaultNotifierID))
	}

	return notifierIDs, nil
}

// Validate NotifyAPIConfig의 유효성을 검사합니다.
func (c *NotifyAPIConfig) Validate(notifierIDs []string) error {
	// 포트 번호 검증
	if err := validatePort(c.WS.ListenPort); err != nil {
		return apperrors.Wrap(err, apperrors.ErrInvalidInput, "웹서버 포트 설정 오류")
	}

	// WS 설정 검사
	if c.WS.TLSServer {
		if strings.TrimSpace(c.WS.TLSCertFile) == "" {
			return apperrors.New(apperrors.ErrInvalidInput, "웹서버의 Cert 파일 경로가 입력되지 않았습니다")
		}
		if strings.TrimSpace(c.WS.TLSKeyFile) == "" {
			return apperrors.New(apperrors.ErrInvalidInput, "웹서버의 Key 파일 경로가 입력되지 않았습니다")
		}

		// TLS 인증서 파일/URL 존재 여부 검증 (경고만)
		validateFileOrURL(c.WS.TLSCertFile, true)
		validateFileOrURL(c.WS.TLSKeyFile, true)
	}

	// Applications 설정 검사
	var applicationIDs []string
	for _, app := range c.Applications {
		if utils.Contains(applicationIDs, app.ID) {
			return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("ApplicationID(%s)가 중복되었습니다", app.ID))
		}
		applicationIDs = append(applicationIDs, app.ID)

		if !utils.Contains(notifierIDs, app.DefaultNotifierID) {
			return apperrors.New(apperrors.ErrNotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s Application의 기본 NotifierID(%s)가 존재하지 않습니다", app.ID, app.DefaultNotifierID))
		}

		if len(app.AppKey) == 0 {
			return apperrors.New(apperrors.ErrInvalidInput, fmt.Sprintf("%s Application의 APP_KEY가 입력되지 않았습니다", app.ID))
		}
	}

	return nil
}
