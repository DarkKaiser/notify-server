package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	apperrors "github.com/darkkaiser/notify-server/internal/pkg/errors"
	"github.com/darkkaiser/notify-server/pkg/cronx"
	"github.com/go-playground/validator/v10"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

const (
	// AppName 애플리케이션의 전역 고유 식별자입니다.
	AppName string = "notify-server"

	// DefaultFilename 애플리케이션 초기화 시 참조하는 기본 설정 파일명입니다.
	// 실행 인자를 통해 명시적인 경로가 제공되지 않을 경우, 시스템은 이 파일을 탐색하여 구성을 로드합니다.
	DefaultFilename = AppName + ".json"

	// ------------------------------------------------------------------------------------------------
	// HTTP 재시도 정책 기본값
	// ------------------------------------------------------------------------------------------------

	// DefaultMaxRetries HTTP 요청 실패 시 최대 재시도 횟수 기본값
	DefaultMaxRetries = 3

	// DefaultRetryDelay 재시도 사이의 대기 시간 기본값
	DefaultRetryDelay = "2s"
)

// AppConfig 애플리케이션의 모든 설정을 관장하는 최상위 루트 구조체
type AppConfig struct {
	Debug     bool            `json:"debug"`
	HTTPRetry HTTPRetryConfig `json:"http_retry"`
	Notifiers NotifierConfig  `json:"notifiers"`
	Tasks     []TaskConfig    `json:"tasks" validate:"unique=ID"`
	NotifyAPI NotifyAPIConfig `json:"notify_api"`
}

// validate 설정 파일 로드 직후, 각 설정 항목의 정합성과 필수 값의 유효성을 검증합니다.
func (c *AppConfig) validate() error {
	// HTTP 재시도 정책 유효성 검사
	if err := c.HTTPRetry.validate(); err != nil {
		return err
	}

	// Notifiers 유효성 검사
	notifierIDs, err := c.Notifiers.validate()
	if err != nil {
		return err
	}

	// Tasks 유효성 검사
	if err := c.validateTasks(notifierIDs); err != nil {
		return err
	}

	// NotifyAPI 유효성 검사
	if err := c.NotifyAPI.validate(notifierIDs); err != nil {
		return err
	}

	return nil
}

func (c *AppConfig) validateTasks(notifierIDs []string) error {
	// Tasks 중복 ID 검사
	if err := checkUniqueField(c.Tasks, "ID", "Task"); err != nil {
		return err
	}

	for _, t := range c.Tasks {
		// Task 구조체 유효성 검사
		if err := validateStruct(t, fmt.Sprintf("Task['%s']", t.ID)); err != nil {
			return err
		}

		for _, cmd := range t.Commands {
			// Command 구조체 유효성 검사
			if err := validateStruct(cmd, fmt.Sprintf("Task['%s'] > Command['%s']", t.ID, cmd.ID)); err != nil {
				return err
			}

			// Notifier 존재 여부 확인
			if !slices.Contains(notifierIDs, cmd.DefaultNotifierID) {
				return apperrors.New(apperrors.NotFound, fmt.Sprintf("Task['%s'] > Command['%s']에서 참조하는 NotifierID('%s')가 정의되지 않았습니다", t.ID, cmd.ID, cmd.DefaultNotifierID))
			}

			// Cron 표현식 검증 (Scheduler가 활성화된 경우)
			if cmd.Scheduler.Runnable {
				if err := cronx.Validate(cmd.Scheduler.TimeSpec); err != nil {
					return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("Task['%s'] > Command['%s']의 스케줄러(TimeSpec) 설정이 유효하지 않습니다", t.ID, cmd.ID))
				}
			}
		}
	}

	return nil
}

// VerifyRecommendations 서비스 운영의 안정성과 보안을 위해 권장되는 설정 준수 여부를 진단합니다.
// 강제적인 에러를 발생시키지는 않으나, 잠재적 위험 요소(예: Well-known Port 사용)에 대한 경고 메시지를 반환합니다.
func (c *AppConfig) VerifyRecommendations() []string {
	return c.NotifyAPI.VerifyRecommendations()
}

// HTTPRetryConfig HTTP 요청 실패 시 재시도 횟수와 대기 시간을 정의하는 설정 구조체
type HTTPRetryConfig struct {
	MaxRetries int    `json:"max_retries"`
	RetryDelay string `json:"retry_delay"`
}

func (c *HTTPRetryConfig) validate() error {
	if _, err := time.ParseDuration(c.RetryDelay); err != nil {
		return apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("HTTP 재시도 대기 시간(retry_delay) 설정이 올바르지 않습니다: '%s' (예: 1s, 500ms)", c.RetryDelay))
	}
	return nil
}

// NotifierConfig 텔레그램 등 다양한 알림 채널을 정의하는 설정 구조체
type NotifierConfig struct {
	DefaultNotifierID string           `json:"default_notifier_id"`
	Telegrams         []TelegramConfig `json:"telegrams" validate:"unique=ID"`
}

func (c *NotifierConfig) validate() ([]string, error) {
	// Notifier 중복 ID 검사
	if err := checkUniqueField(c.Telegrams, "ID", "Notifier"); err != nil {
		return nil, err
	}

	// Telegrams 개별 유효성 검사
	for _, telegram := range c.Telegrams {
		if err := validateStruct(telegram, fmt.Sprintf("Telegram Notifier['%s']", telegram.ID)); err != nil {
			return nil, err
		}
	}

	var notifierIDs []string
	for _, telegram := range c.Telegrams {
		notifierIDs = append(notifierIDs, telegram.ID)
	}

	// 기본 Notifier ID 검사
	if !slices.Contains(notifierIDs, c.DefaultNotifierID) {
		return nil, apperrors.New(apperrors.NotFound, fmt.Sprintf("기본 NotifierID('%s')가 정의된 Notifier 목록에 존재하지 않습니다", c.DefaultNotifierID))
	}

	return notifierIDs, nil
}

// TelegramConfig 텔레그램 봇 토큰 및 채팅 ID 정보를 담는 설정 구조체
type TelegramConfig struct {
	ID       string `json:"id" validate:"required"`
	BotToken string `json:"bot_token" validate:"required"`
	ChatID   int64  `json:"chat_id" validate:"required"`
}

// TaskConfig 주기적으로 실행하거나 특정 조건에 따라 수행할 작업을 정의하는 구조체
type TaskConfig struct {
	ID       string                 `json:"id" validate:"required"`
	Title    string                 `json:"title"`
	Commands []CommandConfig        `json:"commands" validate:"unique=ID"`
	Data     map[string]interface{} `json:"data"`
}

// CommandConfig 작업(Task) 내에서 실제로 실행되는 개별 명령을 정의하는 구조체
type CommandConfig struct {
	ID          string `json:"id" validate:"required"`
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

// NotifyAPIConfig 알림 발송을 위한 REST API 서버 및 웹소켓 설정 구조체
type NotifyAPIConfig struct {
	WS           WSConfig            `json:"ws"`
	CORS         CORSConfig          `json:"cors"`
	Applications []ApplicationConfig `json:"applications" validate:"unique=ID"`
}

func (c *NotifyAPIConfig) validate(notifierIDs []string) error {
	// WS 유효성 검사
	if err := c.WS.validate(); err != nil {
		return err
	}

	// CORS 유효성 검사
	if err := c.CORS.validate(); err != nil {
		return err
	}

	// Applications 중복 ID 검사
	if err := checkUniqueField(c.Applications, "ID", "Application"); err != nil {
		return err
	}

	for _, app := range c.Applications {
		if !slices.Contains(notifierIDs, app.DefaultNotifierID) {
			return apperrors.New(apperrors.NotFound, fmt.Sprintf("Application['%s']에서 참조하는 기본 NotifierID('%s')가 정의되지 않았습니다", app.ID, app.DefaultNotifierID))
		}

		if strings.TrimSpace(app.AppKey) == "" {
			return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("Application['%s']의 API 키(APP_KEY)가 설정되지 않았습니다", app.ID))
		}
	}

	return nil
}

func (c *NotifyAPIConfig) VerifyRecommendations() []string {
	return c.WS.VerifyRecommendations()
}

// WSConfig 웹소버의 포트 및 TLS(HTTPS) 보안 설정을 정의하는 구조체
type WSConfig struct {
	TLSServer   bool   `json:"tls_server"`
	TLSCertFile string `json:"tls_cert_file" validate:"required_if=TLSServer true,omitempty,file"`
	TLSKeyFile  string `json:"tls_key_file" validate:"required_if=TLSServer true,omitempty,file"`
	ListenPort  int    `json:"listen_port" validate:"min=1,max=65535"`
}

func (c *WSConfig) validate() error {
	if err := validate.Struct(c); err != nil {
		// Validator 에러를 사용자 친화적인 메시지로 변환한다.
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, fieldErr := range validationErrors {
				switch fieldErr.StructField() {
				case "ListenPort":
					return apperrors.New(apperrors.InvalidInput, "웹 서버 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다")
				case "TLSCertFile":
					switch fieldErr.Tag() {
					case "required_if":
						return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 인증서 파일 경로(tls_cert_file)는 필수입니다")
					case "file":
						return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지정된 TLS 인증서 파일(tls_cert_file)을 찾을 수 없습니다: '%v'", fieldErr.Value()))
					default:
						return apperrors.New(apperrors.InvalidInput, "TLS 인증서 파일 경로(tls_cert_file) 설정이 올바르지 않습니다")
					}
				case "TLSKeyFile":
					switch fieldErr.Tag() {
					case "required_if":
						return apperrors.New(apperrors.InvalidInput, "TLS 서버 활성화 시 키 파일 경로(tls_key_file)는 필수입니다")
					case "file":
						return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("지정된 TLS 키 파일(tls_key_file)을 찾을 수 없습니다: '%v'", fieldErr.Value()))
					default:
						return apperrors.New(apperrors.InvalidInput, "TLS 키 파일 경로(tls_key_file) 설정이 올바르지 않습니다")
					}
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "웹 서버 설정 검증 중 알 수 없는 오류가 발생했습니다")
	}

	return nil
}

func (c *WSConfig) VerifyRecommendations() []string {
	var warnings []string

	// 시스템 예약 포트(1024 미만) 사용 경고
	if c.ListenPort < 1024 {
		warnings = append(warnings, fmt.Sprintf("시스템 예약 포트(1-1023)를 사용하도록 설정되었습니다(port: %d). 이 경우 서버 구동 시 관리자 권한이 필요할 수 있습니다", c.ListenPort))
	}

	return warnings
}

// CORSConfig 웹 브라우저의 교차 출처 리소스 공유(CORS) 정책을 설정하는 구조체
type CORSConfig struct {
	AllowOrigins []string `json:"allow_origins" validate:"dive,cors_origin"`
}

func (c *CORSConfig) validate() error {
	if len(c.AllowOrigins) == 0 {
		return apperrors.New(apperrors.InvalidInput, "CORS 허용 도메인(allow_origins) 목록이 비어있습니다")
	}

	for _, origin := range c.AllowOrigins {
		if origin == "*" {
			if len(c.AllowOrigins) > 1 {
				return apperrors.New(apperrors.InvalidInput, "와일드카드(*)는 다른 도메인과 함께 사용할 수 없습니다. 모든 도메인을 허용하려면 와일드카드만 설정하세요")
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
					return apperrors.New(apperrors.InvalidInput, fmt.Sprintf("CORS Origin 형식이 올바르지 않습니다: '%v' (형식: Scheme://Host[:Port], 예: https://example.com)", fieldErr.Value()))
				}
			}
		}
		return apperrors.Wrap(err, apperrors.InvalidInput, "CORS 설정 검증 중 알 수 없는 오류가 발생했습니다")
	}
	return nil
}

// ApplicationConfig 알림 API를 사용할 수 있는 클라이언트 어플리케이션의 인증 정보를 정의하는 구조체
type ApplicationConfig struct {
	ID                string `json:"id"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	DefaultNotifierID string `json:"default_notifier_id"`
	AppKey            string `json:"app_key"`
}

// Load 기본 설정 파일을 읽어 애플리케이션 설정을 로드합니다.
func Load() (*AppConfig, error) {
	return LoadWithFile(DefaultFilename)
}

// LoadWithFile 지정된 경로의 설정 파일을 읽어 AppConfig 객체를 생성합니다.
func LoadWithFile(filename string) (*AppConfig, error) {
	k := koanf.New(".")

	// 1. 기본값 로드 (가장 낮은 우선순위)
	err := k.Load(confmap.Provider(map[string]interface{}{
		"http_retry.max_retries": DefaultMaxRetries,
		"http_retry.retry_delay": DefaultRetryDelay,
	}, "."), nil)
	if err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "애플리케이션 기본 설정 로드에 실패했습니다")
	}

	// 2. JSON 설정 파일 로드 (기본값 덮어쓰기)
	if err := k.Load(file.Provider(filename), json.Parser()); err != nil {
		if os.IsNotExist(err) {
			return nil, apperrors.Wrap(err, apperrors.System, fmt.Sprintf("설정 파일을 찾을 수 없습니다: '%s'", filename))
		}
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("설정 파일 로드 중 오류가 발생했습니다: '%s'", filename))
	}

	// 3. 환경 변수 로드 (최우선 순위, JSON 설정 덮어쓰기)
	// 접두사: NOTIFY_
	// 구분자: 이중 언더스코어(__)를 점(.)으로 변환 (계층 구조 표현)
	// 예: NOTIFY_HTTP_RETRY__MAX_RETRIES -> http_retry.max_retries
	if err := k.Load(env.Provider("NOTIFY_", ".", func(s string) string {
		s = strings.TrimPrefix(s, "NOTIFY_")
		s = strings.ToLower(s)
		return strings.ReplaceAll(s, "__", ".")
	}), nil); err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "환경 변수 로드에 실패했습니다")
	}

	// 4. 구조체 언마샬링 (Strict Validation 적용)
	unmarshalConf := koanf.UnmarshalConf{
		Tag: "json",
		DecoderConfig: &mapstructure.DecoderConfig{
			ErrorUnused:      true, // 파일에 존재하지만 구조체에 없는 필드가 있을 경우 에러를 발생시킴
			WeaklyTypedInput: true,
		},
	}
	var appConfig AppConfig
	if err := k.UnmarshalWithConf("", &appConfig, unmarshalConf); err != nil {
		return nil, apperrors.Wrap(err, apperrors.System, "설정 데이터를 애플리케이션 구조체로 변환하는데 실패했습니다")
	}

	// 5. 유효성 검사 수행 (정합성 체크)
	if err := appConfig.validate(); err != nil {
		return nil, apperrors.Wrap(err, apperrors.InvalidInput, fmt.Sprintf("설정 파일('%s')의 유효성 검증에 실패했습니다", filename))
	}

	return &appConfig, nil
}
