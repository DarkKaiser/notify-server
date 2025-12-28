package config

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/internal/pkg/validation"
)

// 애플리케이션 기본 정보
const (
	AppName string = "notify-server"

	AppConfigFileName = AppName + ".json"
)

// HTTP 재시도 기본값
const (
	// DefaultMaxRetries HTTP 요청 실패 시 최대 재시도 횟수 기본값
	DefaultMaxRetries = 3

	// DefaultRetryDelay 재시도 사이의 대기 시간 기본값
	DefaultRetryDelay = "2s"
)

// AppConfig 애플리케이션 전체 설정 구조체
// JSON to Go struct 변환 도구: mholt.github.io/json-to-go
type AppConfig struct {
	Debug     bool            `json:"debug"`
	HTTPRetry HTTPRetryConfig `json:"http_retry"`
	Notifiers NotifierConfig  `json:"notifiers"`
	Tasks     []TaskConfig    `json:"tasks"`
	NotifyAPI NotifyAPIConfig `json:"notify_api"`
}

// Validate AppConfig의 유효성을 검사합니다.
func (c *AppConfig) Validate() error {
	// HTTP Retry 설정 검증
	if err := c.HTTPRetry.Validate(); err != nil {
		return err
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
		if err := validation.ValidateNoDuplicate(taskIDs, t.ID, "TaskID"); err != nil {
			return err
		}
		taskIDs = append(taskIDs, t.ID)

		var commandIDs []string
		for _, cmd := range t.Commands {
			if err := validation.ValidateNoDuplicate(commandIDs, cmd.ID, "CommandID"); err != nil {
				return err
			}
			commandIDs = append(commandIDs, cmd.ID)

			if !slices.Contains(notifierIDs, cmd.DefaultNotifierID) {
				return apperrors.New(apperrors.NotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s::%s Task의 기본 NotifierID(%s)가 존재하지 않습니다", t.ID, cmd.ID, cmd.DefaultNotifierID))
			}

			// Cron 표현식 검증 (Scheduler가 활성화된 경우)
			if cmd.Scheduler.Runnable {
				if err := validation.ValidateRobfigCronExpression(cmd.Scheduler.TimeSpec); err != nil {
					return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s::%s Task의 Scheduler 설정 오류", t.ID, cmd.ID))
				}
			}
		}
	}
	return nil
}

// HTTPRetryConfig HTTP 재시도 설정 구조체
type HTTPRetryConfig struct {
	MaxRetries int    `json:"max_retries"`
	RetryDelay string `json:"retry_delay"`
}

// Validate HTTPRetryConfig의 유효성을 검사합니다.
func (c *HTTPRetryConfig) Validate() error {
	if err := validation.ValidateDuration(c.RetryDelay); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "HTTP Retry 설정 오류")
	}
	return nil
}

// NotifierConfig 알림 설정 구조체
type NotifierConfig struct {
	DefaultNotifierID string           `json:"default_notifier_id"`
	Telegrams         []TelegramConfig `json:"telegrams"`
}

// Validate NotifierConfig의 유효성을 검사하고, 정의된 모든 Notifier의 ID 목록을 반환합니다.
// 반환된 ID 목록은 Task 및 Application 설정에서 참조하는 NotifierID의 유효성을 검증하는 데 사용됩니다.
func (c *NotifierConfig) Validate() ([]string, error) {
	var notifierIDs []string
	for _, telegram := range c.Telegrams {
		if err := validation.ValidateNoDuplicate(notifierIDs, telegram.ID, "NotifierID"); err != nil {
			return nil, err
		}
		notifierIDs = append(notifierIDs, telegram.ID)
	}

	if !slices.Contains(notifierIDs, c.DefaultNotifierID) {
		return nil, apperrors.New(apperrors.NotFound, fmt.Sprintf("전체 NotifierID 목록에서 기본 NotifierID(%s)가 존재하지 않습니다", c.DefaultNotifierID))
	}

	return notifierIDs, nil
}

// TelegramConfig 텔레그램 알림 설정 구조체
type TelegramConfig struct {
	ID       string `json:"id"`
	BotToken string `json:"bot_token"`
	ChatID   int64  `json:"chat_id"`
}

// TaskConfig Task 설정 구조체
type TaskConfig struct {
	ID       string                 `json:"id"`
	Title    string                 `json:"title"`
	Commands []CommandConfig        `json:"commands"`
	Data     map[string]interface{} `json:"data"`
}

// CommandConfig Task 명령 설정 구조체
type CommandConfig struct {
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
	CORS         CORSConfig          `json:"cors"`
	Applications []ApplicationConfig `json:"applications"`
}

// Validate NotifyAPIConfig의 유효성을 검사합니다.
func (c *NotifyAPIConfig) Validate(notifierIDs []string) error {
	// WS 설정 검사
	if err := c.WS.Validate(); err != nil {
		return err
	}

	// CORS 설정 검사
	if err := c.CORS.Validate(); err != nil {
		return err
	}

	// Applications 설정 검사
	var applicationIDs []string
	for _, app := range c.Applications {
		if err := validation.ValidateNoDuplicate(applicationIDs, app.ID, "ApplicationID"); err != nil {
			return err
		}
		applicationIDs = append(applicationIDs, app.ID)

		if !slices.Contains(notifierIDs, app.DefaultNotifierID) {
			return apperrors.New(apperrors.NotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s Application의 기본 NotifierID(%s)가 존재하지 않습니다", app.ID, app.DefaultNotifierID))
		}

		if strings.TrimSpace(app.AppKey) == "" {
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s Application의 APP_KEY가 입력되지 않았습니다", app.ID))
		}
	}

	return nil
}

// WSConfig 웹서버 설정 구조체
type WSConfig struct {
	TLSServer   bool   `json:"tls_server"`
	TLSCertFile string `json:"tls_cert_file"`
	TLSKeyFile  string `json:"tls_key_file"`
	ListenPort  int    `json:"listen_port"`
}

// Validate WSConfig의 유효성을 검사합니다.
func (c *WSConfig) Validate() error {
	// 포트 번호 검증
	if err := validation.ValidatePort(c.ListenPort); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, "웹서버 포트 설정 오류")
	}

	// TLS 설정 검사
	if c.TLSServer {
		if strings.TrimSpace(c.TLSCertFile) == "" {
			return apperrors.New(apperrors.InvalidInput, "웹서버의 Cert 파일 경로가 입력되지 않았습니다")
		}
		if strings.TrimSpace(c.TLSKeyFile) == "" {
			return apperrors.New(apperrors.InvalidInput, "웹서버의 Key 파일 경로가 입력되지 않았습니다")
		}

		// TLS 인증서 파일/URL 존재 여부 검증 (경고만)
		_ = validation.ValidateFileExistsOrURL(c.TLSCertFile, true)
		_ = validation.ValidateFileExistsOrURL(c.TLSKeyFile, true)
	}

	return nil
}

// CORSConfig CORS 설정 구조체
type CORSConfig struct {
	AllowOrigins []string `json:"allow_origins"`
}

// Validate CORS 설정의 유효성을 검사합니다.
func (c *CORSConfig) Validate() error {
	if len(c.AllowOrigins) == 0 {
		return apperrors.New(apperrors.InvalidInput, "CORS AllowOrigins 설정이 비어있습니다")
	}

	for _, origin := range c.AllowOrigins {
		if origin == "*" {
			if len(c.AllowOrigins) > 1 {
				return apperrors.New(apperrors.InvalidInput, "CORS AllowOrigins에 와일드카드(*)가 포함된 경우, 다른 Origin과 함께 사용할 수 없습니다")
			}
			continue
		}

		if err := validation.ValidateCORSOrigin(origin); err != nil {
			return err
		}
	}
	return nil
}

// ApplicationConfig 애플리케이션 설정 구조체
type ApplicationConfig struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	DefaultNotifierID string `json:"default_notifier_id"`
	AppKey            string `json:"app_key"`
}

func InitAppConfig() (*AppConfig, error) {
	return InitAppConfigWithFile(AppConfigFileName)
}

// InitAppConfigWithFile 지정된 파일에서 설정을 로드합니다.
// 이 함수는 테스트에서 사용할 수 있도록 파일명을 인자로 받습니다.
func InitAppConfigWithFile(filename string) (*AppConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, fmt.Sprintf("%s 파일을 열 수 없습니다", filename))
	}
	defer file.Close()

	var appConfig AppConfig
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&appConfig); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s 파일의 JSON 파싱이 실패하였습니다", filename))
	}

	// 기본값 설정
	appConfig.SetDefaults()

	//
	// 파일 내용에 대해 유효성 검사를 한다.
	//
	if err := appConfig.Validate(); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("%s 파일의 내용이 유효하지 않습니다", filename))
	}

	return &appConfig, nil
}

func (c *AppConfig) SetDefaults() {
	// HTTP Retry 설정 기본값 적용
	if c.HTTPRetry.MaxRetries == 0 {
		c.HTTPRetry.MaxRetries = DefaultMaxRetries
	}
	if c.HTTPRetry.RetryDelay == "" {
		c.HTTPRetry.RetryDelay = DefaultRetryDelay
	}
}
