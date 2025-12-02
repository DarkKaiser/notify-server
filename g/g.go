package g

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/darkkaiser/notify-server/utils"
)

const (
	AppName    string = "notify-server"
	AppVersion string = "0.0.3"

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
		return nil, fmt.Errorf("%s 파일의 내용이 유효하지 않습니다. %v", filename, err)
	}

	return &appConfig, nil
}

// Validate AppConfig의 유효성을 검사합니다.
func (c *AppConfig) Validate() error {
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
			return fmt.Errorf("TaskID(%s)가 중복되었습니다", t.ID)
		}
		taskIDs = append(taskIDs, t.ID)

		var commandIDs []string
		for _, cmd := range t.Commands {
			if utils.Contains(commandIDs, cmd.ID) {
				return fmt.Errorf("CommandID(%s)가 중복되었습니다", cmd.ID)
			}
			commandIDs = append(commandIDs, cmd.ID)

			if !utils.Contains(notifierIDs, cmd.DefaultNotifierID) {
				return fmt.Errorf("전체 NotifierID 목록에서 %s::%s Task의 기본 NotifierID(%s)가 존재하지 않습니다", t.ID, cmd.ID, cmd.DefaultNotifierID)
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
			return nil, fmt.Errorf("NotifierID(%s)가 중복되었습니다", telegram.ID)
		}
		notifierIDs = append(notifierIDs, telegram.ID)
	}

	if !utils.Contains(notifierIDs, c.DefaultNotifierID) {
		return nil, fmt.Errorf("전체 NotifierID 목록에서 기본 NotifierID(%s)가 존재하지 않습니다", c.DefaultNotifierID)
	}

	return notifierIDs, nil
}

// Validate NotifyAPIConfig의 유효성을 검사합니다.
func (c *NotifyAPIConfig) Validate(notifierIDs []string) error {
	// WS 설정 검사
	if c.WS.TLSServer {
		if strings.TrimSpace(c.WS.TLSCertFile) == "" {
			return fmt.Errorf("웹서버의 Cert 파일 경로가 입력되지 않았습니다")
		}
		if strings.TrimSpace(c.WS.TLSKeyFile) == "" {
			return fmt.Errorf("웹서버의 Key 파일 경로가 입력되지 않았습니다")
		}
	}

	// Applications 설정 검사
	var applicationIDs []string
	for _, app := range c.Applications {
		if utils.Contains(applicationIDs, app.ID) {
			return fmt.Errorf("ApplicationID(%s)가 중복되었습니다", app.ID)
		}
		applicationIDs = append(applicationIDs, app.ID)

		if !utils.Contains(notifierIDs, app.DefaultNotifierID) {
			return fmt.Errorf("전체 NotifierID 목록에서 %s Application의 기본 NotifierID(%s)가 존재하지 않습니다", app.ID, app.DefaultNotifierID)
		}

		if len(app.AppKey) == 0 {
			return fmt.Errorf("%s Application의 APP_KEY가 입력되지 않았습니다", app.ID)
		}
	}

	return nil
}
