package config

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	applog "github.com/darkkaiser/notify-server/pkg/log"
	"github.com/go-playground/validator/v10"
	log "github.com/sirupsen/logrus"

	appvalidation "github.com/darkkaiser/notify-server/pkg/validation"
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
	Tasks     []TaskConfig    `json:"tasks" validate:"unique=ID"`
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

// VerifyRecommendations 애플리케이션 전반의 운영 적합성 및 안정성을 점검합니다.
func (c *AppConfig) VerifyRecommendations() {
	// NotifyAPI 권고 사항 검토
	c.NotifyAPI.VerifyRecommendations()
}

// validateTasks Task 설정의 유효성을 검사합니다.
func (c *AppConfig) validateTasks(notifierIDs []string) error {
	// Tasks 중복 ID 검사 (Validator)
	if err := validate.Var(c.Tasks, "unique=ID"); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				if fieldErr.Tag() == "unique" {
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("TaskID(%v)가 중복되었습니다", fieldErr.Value()))
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "Task 설정 검증 실패")
	}

	for _, t := range c.Tasks {
		// Commands 중복 ID 검사 (Validator)
		// 구조체 태그를 활용하기 위해 validate.Var 사용
		if err := validate.Var(t.Commands, "unique=ID"); err != nil {
			if validationErrors, ok := err.(validator.ValidationErrors); ok {
				for _, fieldErr := range validationErrors {
					if fieldErr.Tag() == "unique" {
						return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("CommandID(%v)가 중복되었습니다", fieldErr.Value()))
					}
				}
			}
			return apperrors.Wrap(err, apperrors.InvalidInput, "Command 설정 검증 실패")
		}

		for _, cmd := range t.Commands {
			if !slices.Contains(notifierIDs, cmd.DefaultNotifierID) {
				return apperrors.New(apperrors.NotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s::%s Task의 기본 NotifierID(%s)가 존재하지 않습니다", t.ID, cmd.ID, cmd.DefaultNotifierID))
			}

			// Cron 표현식 검증 (Scheduler가 활성화된 경우)
			if cmd.Scheduler.Runnable {
				if err := appvalidation.ValidateCronExpression(cmd.Scheduler.TimeSpec); err != nil {
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
	if _, err := time.ParseDuration(c.RetryDelay); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("HTTP Retry 설정 오류: 잘못된 duration 형식입니다 (%s)", c.RetryDelay))
	}
	return nil
}

// NotifierConfig 알림 설정 구조체
type NotifierConfig struct {
	DefaultNotifierID string           `json:"default_notifier_id"`
	Telegrams         []TelegramConfig `json:"telegrams" validate:"unique=ID"`
}

// Validate NotifierConfig의 유효성을 검사하고, 정의된 모든 Notifier의 ID 목록을 반환합니다.
// 반환된 ID 목록은 Task 및 Application 설정에서 참조하는 NotifierID의 유효성을 검증하는 데 사용됩니다.
func (c *NotifierConfig) Validate() ([]string, error) {
	// Notifier 중복 ID 검사 (Validator)
	if err := validate.Var(c.Telegrams, "unique=ID"); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				if fieldErr.Tag() == "unique" {
					return nil, apperrors.New(apperrors.InvalidInput, fmt.Sprintf("NotifierID(%v)가 중복되었습니다", fieldErr.Value()))
				}
			}
		}
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, "Notifier 설정 검증 실패")
	}

	var notifierIDs []string
	for _, telegram := range c.Telegrams {
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
	Commands []CommandConfig        `json:"commands" validate:"unique=ID"`
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
	Applications []ApplicationConfig `json:"applications" validate:"unique=ID"`
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

	// Applications 중복 ID 검사 (Validator)
	if err := validate.Var(c.Applications, "unique=ID"); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				if fieldErr.Tag() == "unique" {
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("ApplicationID(%v)가 중복되었습니다", fieldErr.Value()))
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "Applications 설정 검증 실패")
	}

	for _, app := range c.Applications {
		if !slices.Contains(notifierIDs, app.DefaultNotifierID) {
			return apperrors.New(apperrors.NotFound, fmt.Sprintf("전체 NotifierID 목록에서 %s Application의 기본 NotifierID(%s)가 존재하지 않습니다", app.ID, app.DefaultNotifierID))
		}

		if strings.TrimSpace(app.AppKey) == "" {
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("%s Application의 APP_KEY가 입력되지 않았습니다", app.ID))
		}
	}

	return nil
}

// VerifyRecommendations Notify API 서비스의 운영 적합성 및 안정성을 점검합니다.
func (c *NotifyAPIConfig) VerifyRecommendations() {
	c.WS.VerifyRecommendations()
}

// WSConfig 웹서버 설정 구조체
type WSConfig struct {
	TLSServer   bool   `json:"tls_server"`
	TLSCertFile string `json:"tls_cert_file" validate:"required_if=TLSServer true,omitempty,file"`
	TLSKeyFile  string `json:"tls_key_file" validate:"required_if=TLSServer true,omitempty,file"`
	ListenPort  int    `json:"listen_port" validate:"min=1,max=65535"`
}

// Validate WSConfig의 유효성을 검사합니다.
func (c *WSConfig) Validate() error {
	if err := validate.Struct(c); err != nil {
		// Validator 에러를 사용자 친화적인 메시지로 변환
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				switch fieldErr.StructField() {
				case "ListenPort":
					return apperrors.New(apperrors.InvalidInput, "웹 서버 포트 설정이 올바르지 않습니다 (허용 범위: 1-65535)")
				case "TLSCertFile":
					switch fieldErr.Tag() {
					case "required_if":
						return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 인증서 파일 경로(TLSCertFile)는 필수입니다")
					case "file":
						return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("TLS 인증서 파일이 존재하지 않거나 유효하지 않습니다 (입력값: %v)", fieldErr.Value()))
					default:
						return apperrors.New(apperrors.InvalidInput, "TLS 인증서 파일 설정이 올바르지 않습니다")
					}
				case "TLSKeyFile":
					switch fieldErr.Tag() {
					case "required_if":
						return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 키 파일 경로(TLSKeyFile)는 필수입니다")
					case "file":
						return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("TLS 키 파일이 존재하지 않거나 유효하지 않습니다 (입력값: %v)", fieldErr.Value()))
					default:
						return apperrors.New(apperrors.InvalidInput, "TLS 키 파일 설정이 올바르지 않습니다")
					}
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "웹 서버 구성 검증에 실패하였습니다")
	}

	return nil
}

// VerifyRecommendations 웹서버의 운영 보안 및 안정성 설정을 점검합니다.
func (c *WSConfig) VerifyRecommendations() {
	// 시스템 예약 포트(1024 미만) 사용 경고
	if c.ListenPort < 1024 {
		applog.WithComponentAndFields("config", log.Fields{
			"port": c.ListenPort,
		}).Warn("시스템 예약 포트(1-1023)가 설정되었습니다. 서버 구동 시 관리자 권한이 필요할 수 있습니다")
	}
}

// CORSConfig CORS 설정 구조체
type CORSConfig struct {
	AllowOrigins []string `json:"allow_origins" validate:"dive,cors_origin"`
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
			// 와일드카드만 있는 경우는 유효함 (validator skip)
			continue
		}
	}

	// 각 Origin 유효성 검사
	if err := validate.Struct(c); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				if fieldErr.Tag() == "cors_origin" {
					// 상세 에러 메시지가 잘려서 아쉽지만, validator의 한계로 인해 일반적인 메시지 반환
					// 필요하다면 validation.ValidateCORSOrigin을 다시 호출하여 정확한 메시지를 얻을 수도 있음
					// 여기서는 간단하게 처리
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("CORS 설정 오류: 유효하지 않은 Origin 형식입니다 (input=%q). 'Scheme://Host[:Port]' 표준을 준수해야 합니다", fieldErr.Value()))
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "CORS 설정 유효성 검증 실패")
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
