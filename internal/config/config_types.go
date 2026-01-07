package config

import (
	"fmt"
	"slices"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/cronx"
	"github.com/darkkaiser/notify-server/pkg/strutil"
	"github.com/go-playground/validator/v10"
)

// AppConfig 애플리케이션의 모든 설정을 포함하는 최상위 구조체
type AppConfig struct {
	Debug     bool            `json:"debug"`
	HTTPRetry HTTPRetryConfig `json:"http_retry"`
	Notifier  NotifierConfig  `json:"notifier"`
	Tasks     []TaskConfig    `json:"tasks" validate:"unique=ID"`
	NotifyAPI NotifyAPIConfig `json:"notify_api"`
}

// validate 설정 파일 로드 직후, 각 설정 항목의 정합성과 필수 값의 유효성을 검증합니다.
func (c *AppConfig) validate(v *validator.Validate) error {
	if err := c.HTTPRetry.validate(v); err != nil {
		return err
	}

	notifierIDs := c.Notifier.GetIDs()

	if err := c.Notifier.validate(v, notifierIDs); err != nil {
		return err
	}

	if err := c.validateTasks(v, notifierIDs); err != nil {
		return err
	}

	if err := c.NotifyAPI.validate(v, notifierIDs); err != nil {
		return err
	}

	return nil
}

func (c *AppConfig) validateTasks(v *validator.Validate, notifierIDs []string) error {
	// Task ID 중복 검사
	if err := checkStruct(v, c, "작업 설정", "Tasks"); err != nil {
		return err
	}

	for _, t := range c.Tasks {
		// Task 설정 상세 검증
		if err := checkStruct(v, t, fmt.Sprintf("작업 설정 내 작업(ID: %s)", t.ID)); err != nil {
			return err
		}

		for _, cmd := range t.Commands {
			// Command 설정 상세 검증
			if err := checkStruct(v, cmd, fmt.Sprintf("작업 설정 내 작업(ID: %s)에 속한 명령(ID: %s)", t.ID, cmd.ID)); err != nil {
				return err
			}

			// 알림 채널(Notifier) 참조 무결성 검사
			if cmd.Notifier.Usable {
				if !slices.Contains(notifierIDs, cmd.DefaultNotifierID) {
					return apperrors.New(apperrors.NotFound, fmt.Sprintf("작업 설정 내 작업(ID: %s)에 속한 명령(ID: %s)에서 참조하는 알림 채널(ID: '%s')이 정의되지 않았습니다", t.ID, cmd.ID, cmd.DefaultNotifierID))
				}
			}

			// Cron 표현식 검증 (Scheduler가 활성화된 경우)
			if cmd.Scheduler.Runnable {
				if err := cronx.Validate(cmd.Scheduler.TimeSpec); err != nil {
					return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("작업 설정 내 작업(ID: %s)에 속한 명령(ID: %s)의 스케줄러(TimeSpec) 설정이 유효하지 않습니다", t.ID, cmd.ID))
				}
			}
		}
	}

	return nil
}

// lint 서비스 운영의 안정성과 보안을 위해 권장되는 설정 준수 여부를 진단합니다.
// 강제적인 에러를 발생시키지는 않으나, 잠재적 위험 요소(예: Well-known Port 사용)에 대한 경고 메시지를 반환합니다.
func (c *AppConfig) lint() []string {
	return c.NotifyAPI.lint()
}

// HTTPRetryConfig HTTP 요청 실패 시 재시도 횟수와 대기 시간을 정의하는 설정 구조체
type HTTPRetryConfig struct {
	MaxRetries int           `json:"max_retries" validate:"gte=0"`
	RetryDelay time.Duration `json:"retry_delay" validate:"gt=0"`
}

func (c *HTTPRetryConfig) validate(v *validator.Validate) error {
	if err := checkStruct(v, c, "HTTP 재시도 설정"); err != nil {
		return err
	}
	return nil
}

// NotifierConfig 텔레그램 등 다양한 알림 채널을 정의하는 설정 구조체
type NotifierConfig struct {
	DefaultNotifierID string           `json:"default_notifier_id" validate:"required"`
	Telegrams         []TelegramConfig `json:"telegrams" validate:"unique=ID"`
}

func (c *NotifierConfig) validate(v *validator.Validate, notifierIDs []string) error {
	// 필수 값 및 ID 중복 검사
	if err := checkStruct(v, c, "알림 설정", "DefaultNotifierID", "Telegrams"); err != nil {
		return err
	}

	// 각 알림 채널 설정 상세 검증
	for _, telegram := range c.Telegrams {
		if err := checkStruct(v, telegram, fmt.Sprintf("알림 설정 내 텔레그램 알림 채널(ID: %s)", telegram.ID)); err != nil {
			return err
		}
	}

	// 기본 알림 채널 ID 유효성 검사 (참조 무결성)
	if !slices.Contains(notifierIDs, c.DefaultNotifierID) {
		return apperrors.New(apperrors.NotFound, fmt.Sprintf("알림 설정 내 기본 알림 채널로 설정된 ID('%s')가 정의되지 않았습니다. 등록된 알림 채널 목록을 확인해주세요", c.DefaultNotifierID))
	}

	return nil
}

// GetIDs 등록된 모든 알림 채널의 ID 목록을 반환합니다.
func (c *NotifierConfig) GetIDs() []string {
	var ids []string
	for _, telegram := range c.Telegrams {
		ids = append(ids, telegram.ID)
	}
	return ids
}

// TelegramConfig 텔레그램 봇 토큰 및 채팅 ID 정보를 담는 설정 구조체
type TelegramConfig struct {
	ID       string `json:"id" validate:"required"`
	BotToken string `json:"bot_token" validate:"required,telegram_bot_token"`
	ChatID   int64  `json:"chat_id" validate:"required"`
}

func (c TelegramConfig) String() string {
	return fmt.Sprintf("{ID:%s BotToken:%s ChatID:%d}", c.ID, strutil.Mask(c.BotToken), c.ChatID)
}

// TaskConfig 주기적으로 실행하거나 특정 조건에 따라 수행할 작업을 정의하는 구조체
type TaskConfig struct {
	ID       string                 `json:"id" validate:"required"`
	Title    string                 `json:"title"`
	Commands []CommandConfig        `json:"commands" validate:"required,min=1,unique=ID"`
	Data     map[string]interface{} `json:"data"`
}

// CommandConfig 작업(Task) 내에서 실제로 실행되는 개별 명령을 정의하는 구조체
type CommandConfig struct {
	ID                string                 `json:"id" validate:"required"`
	Title             string                 `json:"title"`
	Description       string                 `json:"description"`
	Scheduler         SchedulerConfig        `json:"scheduler"`
	Notifier          CommandNotifierConfig  `json:"notifier"`
	DefaultNotifierID string                 `json:"default_notifier_id"`
	Data              map[string]interface{} `json:"data"`
}

// SchedulerConfig 작업 스케줄링 설정을 정의하는 구조체
type SchedulerConfig struct {
	Runnable bool   `json:"runnable"`
	TimeSpec string `json:"time_spec"`
}

// CommandNotifierConfig 작업 완료 후 알림 발송 여부를 정의하는 구조체
type CommandNotifierConfig struct {
	Usable bool `json:"usable"`
}

// NotifyAPIConfig 알림 발송을 위한 REST API 서버 및 웹소켓 설정 구조체
type NotifyAPIConfig struct {
	WS           WSConfig            `json:"ws"`
	CORS         CORSConfig          `json:"cors"`
	Applications []ApplicationConfig `json:"applications" validate:"unique=ID"`
}

func (c *NotifyAPIConfig) validate(v *validator.Validate, notifierIDs []string) error {
	if err := c.WS.validate(v); err != nil {
		return err
	}

	if err := c.CORS.validate(v); err != nil {
		return err
	}

	// Application ID 중복 검사
	if err := checkStruct(v, c, "알림 API 설정", "Applications"); err != nil {
		return err
	}

	for _, app := range c.Applications {
		// Application 설정 상세 검증
		if err := checkStruct(v, app, fmt.Sprintf("알림 API 설정 내 애플리케이션(ID: %s)", app.ID)); err != nil {
			return err
		}

		// 알림 채널(Notifier) 참조 무결성 검사
		if !slices.Contains(notifierIDs, app.DefaultNotifierID) {
			return apperrors.New(apperrors.NotFound, fmt.Sprintf("알림 API 설정 내 애플리케이션(ID: %s)에서 참조하는 알림 채널(ID: '%s')이 정의되지 않았습니다", app.ID, app.DefaultNotifierID))
		}
	}

	return nil
}

func (c *NotifyAPIConfig) lint() []string {
	return c.WS.lint()
}

// WSConfig 웹 서비스의 포트 및 TLS(HTTPS) 보안 설정을 정의하는 구조체
type WSConfig struct {
	TLSServer   bool   `json:"tls_server"`
	TLSCertFile string `json:"tls_cert_file" validate:"required_if=TLSServer true,omitempty,file"`
	TLSKeyFile  string `json:"tls_key_file" validate:"required_if=TLSServer true,omitempty,file"`
	ListenPort  int    `json:"listen_port" validate:"min=1,max=65535"`
}

func (c *WSConfig) validate(v *validator.Validate) error {
	// 웹 서비스(포트, TLS) 설정 유효성 검사
	if err := checkStruct(v, c, "웹 서비스 설정"); err != nil {
		return err
	}
	return nil
}

func (c *WSConfig) lint() []string {
	var warnings []string

	// 시스템 예약 포트(1024 미만) 사용 경고
	if c.ListenPort < 1024 {
		warnings = append(warnings, fmt.Sprintf("시스템 예약 포트(1-1023)를 사용하도록 설정되었습니다(포트: %d). 이 경우 서버 구동 시 관리자 권한이 필요할 수 있습니다", c.ListenPort))
	}

	return warnings
}

// CORSConfig 웹 브라우저의 교차 출처 리소스 공유(CORS) 정책을 설정하는 구조체
type CORSConfig struct {
	AllowOrigins []string `json:"allow_origins" validate:"dive,cors_origin"`
}

func (c *CORSConfig) validate(v *validator.Validate) error {
	if len(c.AllowOrigins) == 0 {
		return apperrors.New(apperrors.InvalidInput, "CORS 허용 도메인(allow_origins) 목록이 비어있습니다")
	}

	for _, origin := range c.AllowOrigins {
		if origin == "*" {
			if len(c.AllowOrigins) > 1 {
				return apperrors.New(apperrors.InvalidInput, "CORS 허용 도메인(allow_origins)에서 와일드카드(*)는 다른 도메인과 함께 사용할 수 없습니다. 모든 도메인을 허용하려면 와일드카드만 설정하세요")
			}

			// 와일드카드만 있는 경우는 유효함 (validator skip)
			continue
		}
	}

	// 개별 Origin 형식 유효성 검사
	if err := checkStruct(v, c, "CORS 설정"); err != nil {
		return err
	}
	return nil
}

// ApplicationConfig 알림 API를 사용할 수 있는 클라이언트 어플리케이션의 인증 정보를 정의하는 구조체
type ApplicationConfig struct {
	ID                string `json:"id" validate:"required"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	DefaultNotifierID string `json:"default_notifier_id" validate:"required"`
	AppKey            string `json:"app_key" validate:"required"`
}

func (c ApplicationConfig) String() string {
	return fmt.Sprintf("{ID:%s Title:%s Description:%s DefaultNotifierID:%s AppKey:%s}",
		c.ID, c.Title, c.Description, c.DefaultNotifierID, strutil.Mask(c.AppKey))
}
