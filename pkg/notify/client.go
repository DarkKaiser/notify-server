package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Config notify-server REST API 클라이언트를 생성할 때 필요한 모든 설정을 담는 구조체입니다.
type Config struct {
	// URL notify-server의 알림 전송 REST API 엔드포인트 주소입니다.
	// 반드시 "http://" 또는 "https://" 스킴으로 시작해야 하며, 유효한 호스트를 포함해야 합니다.
	// 예: "https://notify.example.com/api/v1/notifications"
	URL string

	// AppKey notify-server가 요청의 신뢰성을 검증하는 데 사용하는 인증 키입니다.
	// notify-server 측 설정에서 발급받은 값을 그대로 입력하세요.
	// 이 값이 일치하지 않으면 서버가 401 Unauthorized 등의 오류를 반환합니다.
	AppKey string

	// ApplicationID 알림을 발송하는 애플리케이션의 고유 식별자입니다.
	// notify-server는 이 값을 통해 어떤 애플리케이션이 보낸 알림인지 구분하고,
	// 알림을 적절한 수신자에게 라우팅합니다.
	ApplicationID string

	// Timeout HTTP 요청 하나에 허용되는 최대 대기 시간입니다.
	// 0으로 설정하면 기본값인 10초가 적용됩니다.
	// 음수는 허용되지 않으며, NewClient에서 오류를 반환합니다.
	Timeout time.Duration
}

// validate Config의 각 필드가 유효한 값으로 채워져 있는지 검사합니다.
func (c *Config) validate() error {
	if c.URL == "" {
		return errors.New("유효하지 않은 설정입니다: notify-server URL이 누락되었습니다")
	}
	u, err := url.ParseRequestURI(c.URL)
	if err != nil {
		return fmt.Errorf("유효하지 않은 설정입니다: notify-server URL 형식이 올바르지 않습니다: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("유효하지 않은 설정입니다: notify-server URL은 'http://' 또는 'https://' 프로토콜을 사용해야 합니다")
	}
	if u.Host == "" {
		return errors.New("유효하지 않은 설정입니다: notify-server URL에 호스트 정보가 포함되어 있지 않습니다")
	}

	if c.AppKey == "" {
		return errors.New("유효하지 않은 설정입니다: notify-server AppKey가 누락되었습니다")
	}

	if c.ApplicationID == "" {
		return errors.New("유효하지 않은 설정입니다: notify-server ApplicationID가 누락되었습니다")
	}

	// 음수 Timeout은 http.Client 내부에서 즉시 타임아웃을 유발하는 잘못된 값입니다.
	// 호출자가 실수로 음수를 전달했을 때 조용히 넘어가지 않도록 명시적으로 거부합니다.
	if c.Timeout < 0 {
		return errors.New("유효하지 않은 설정입니다: notify-server Timeout 값은 0 이상이어야 합니다")
	}

	return nil
}

// Client notify-server REST API와 통신하는 HTTP 클라이언트 래퍼입니다.
// 반드시 NewClient 함수를 통해 생성해야 하며, 직접 구조체를 초기화하면 안 됩니다.
// 생성된 Client는 고루틴 간 공유해도 안전합니다(내부 상태 변경 없음).
//
// 기본 사용 예:
//
//	client, err := notify.NewClient(&notify.Config{
//	    URL:           "https://notify.example.com/api/v1/notifications",
//	    AppKey:        "my-app-key",
//	    ApplicationID: "my-app",
//	})
//	if err != nil {
//	    // 설정이 잘못된 경우 여기서 오류가 반환됩니다. 반드시 처리하세요.
//	    log.Fatal(err)
//	}
//
//	// 일반 이벤트 알림 (error_occurred: false)
//	client.Notify(ctx, "배포가 완료되었습니다.")
//
//	// 장애/에러 알림 (error_occurred: true)
//	client.NotifyError(ctx, "DB 연결에 실패했습니다.")
type Client struct {
	config     *Config
	httpClient *http.Client
}

// Option NewClient 호출 시 Client의 기본 동작을 변경하는 함수 타입입니다.
type Option func(*Client)

// WithHTTPClient 외부에서 미리 생성한 *http.Client를 이 Client에 주입하는 옵션입니다.
// 기본 HTTP 클라이언트 대신 커스텀 Transport(예: 프록시, mTLS)나
// 테스트용 mock transport를 사용하고 싶을 때 활용하세요.
//
// Timeout 처리 규칙:
//   - 주입된 클라이언트의 Timeout이 0(무제한)이면, Config.Timeout(> 0인 경우) 또는
//     기본값(10초)을 폴백으로 자동 적용합니다.
//   - 주입된 클라이언트의 Timeout이 이미 0보다 크면, Config.Timeout 설정은 완전히 무시되며
//     주입된 클라이언트의 Timeout이 그대로 유지됩니다.
//
// [얕은 복사 주의]
// Timeout이 0인 경우, 원본 *http.Client를 직접 수정하지 않고 얕은 복사(shallow copy) 후
// Timeout 필드만 덮어씁니다. 따라서 Transport, Jar 등 포인터 필드는 원본과 공유됩니다.
// 이는 http.Transport를 여러 클라이언트가 공유하도록 권장하는 Go 표준 패턴과 일치합니다.
//
// [CookieJar 경고]
// 주입한 *http.Client에 CookieJar(Jar 필드)가 설정된 경우, 얕은 복사로 인해
// 원본 클라이언트와 쿠키 상태가 공유됩니다. 독립적인 쿠키 세션이 필요하다면
// CookieJar 없이 별도의 *http.Client 인스턴스를 생성하여 주입하세요.
//
// [nil 전달 금지]
// nil을 전달하면 즉시 패닉이 발생합니다. 이는 조용히 기본 클라이언트로 대체될 경우
// 발생할 수 있는 디버깅하기 어려운 문제를 방지하기 위한 의도적인 설계입니다.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc == nil {
			// nil *http.Client를 조용히 무시하면 기본 클라이언트가 사용되어,
			// 호출자가 의도한 Transport 설정이 적용되지 않았음을 인지하기 어렵습니다.
			// 즉시 패닉을 발생시켜 호출자가 실수를 바로 인지할 수 있도록 합니다.
			panic("WithHTTPClient: 유효하지 않은 인자입니다: *http.Client 객체는 nil일 수 없습니다")
		}

		c.httpClient = hc
	}
}

// NewClient 주어진 Config 설정으로 새로운 Client를 생성하여 반환합니다.
//
// config가 nil이거나 필드 값이 유효하지 않으면 오류를 반환합니다.
// 성공 시 즉시 Notify / NotifyError를 호출할 수 있는 *Client가 반환됩니다.
func NewClient(config *Config, opts ...Option) (*Client, error) {
	if config == nil {
		return nil, errors.New("유효하지 않은 설정입니다: 클라이언트 구성(Config) 정보가 제공되지 않았습니다")
	}

	// 원본 Config 구조체를 값 복사합니다.
	// 이렇게 하면 호출자가 NewClient 호출 이후 외부에서 Config 필드를 변경하더라도
	// 이미 생성된 클라이언트의 동작에는 영향을 주지 않습니다.
	cfg := *config
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.AppKey = strings.TrimSpace(cfg.AppKey)
	cfg.ApplicationID = strings.TrimSpace(cfg.ApplicationID)

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("유효하지 않은 설정입니다: 클라이언트 구성 정보가 올바르지 않습니다: %w", err)
	}

	// cfg.Timeout이 0이면 기본값 10초를 사용합니다.
	// 이 값은 이후 WithHTTPClient 옵션의 Timeout 폴백에도 동일하게 활용됩니다.
	timeout := 10 * time.Second
	if cfg.Timeout > 0 {
		timeout = cfg.Timeout
	}

	// httpClient 필드를 nil로 두고 옵션을 먼저 적용합니다.
	// WithHTTPClient 옵션이 전달된 경우 httpClient가 채워지고,
	// 전달되지 않은 경우에는 nil 상태가 유지되어 아래에서 기본 클라이언트를 생성합니다.
	// 이 방식 덕분에 WithHTTPClient를 쓰지 않을 때 불필요한 http.Client 인스턴스를 만들지 않습니다.
	client := &Client{config: &cfg}
	for _, opt := range opts {
		opt(client)
	}

	if client.httpClient == nil {
		// WithHTTPClient 옵션이 사용되지 않은 경우: 기본 HTTP 클라이언트를 생성합니다.
		client.httpClient = &http.Client{Timeout: timeout}
	} else if client.httpClient.Timeout == 0 {
		// WithHTTPClient로 주입된 클라이언트의 Timeout이 0(무제한)인 경우,
		// 응답 대기 시간이 무한정 늘어질 수 있으므로 안전한 기본값을 폴백으로 적용합니다.
		// - cfg.Timeout > 0 이면 해당 값 사용
		// - cfg.Timeout == 0 이면 기본값(10초) 사용
		// 위 두 경우 모두 timeout 변수에 이미 반영되어 있습니다.
		//
		// 원본 *http.Client를 직접 수정하면 외부 코드에 부작용을 일으킬 수 있으므로,
		// 얕은 복사(shallow copy) 후 Timeout 필드만 교체합니다.
		// Transport 등 포인터 필드는 원본과 공유되며, 이는 Go에서 권장하는 패턴입니다.
		hc := *client.httpClient
		hc.Timeout = timeout
		client.httpClient = &hc
	}

	return client, nil
}

// notifyRequest notify-server REST API로 전송하는 HTTP 요청 바디의 JSON 스키마를 정의합니다.
type notifyRequest struct {
	ApplicationID string `json:"application_id"`
	Message       string `json:"message"`
	ErrorOccurred bool   `json:"error_occurred"`
}

// Notify 정보성 알림 메시지를 notify-server로 전송합니다.
//
// ctx를 통해 요청의 취소(cancel)나 데드라인(deadline)을 제어할 수 있습니다.
// nil ctx를 전달하면 패닉이 발생합니다. context.Background()를 사용하세요.
//
// message가 공백 문자로만 이루어진 경우(빈 메시지) 오류를 반환합니다.
func (c *Client) Notify(ctx context.Context, message string) error {
	return c.notify(ctx, message, false)
}

// NotifyError 에러/장애 알림 메시지를 notify-server로 전송합니다.
//
// ctx를 통해 요청의 취소(cancel)나 데드라인(deadline)을 제어할 수 있습니다.
// nil ctx를 전달하면 패닉이 발생합니다. context.Background()를 사용하세요.
//
// message가 공백 문자로만 이루어진 경우(빈 메시지) 오류를 반환합니다.
func (c *Client) NotifyError(ctx context.Context, message string) error {
	return c.notify(ctx, message, true)
}

// notify Notify와 NotifyError의 공통 구현입니다.
// 메시지를 JSON으로 직렬화한 뒤 HTTP POST 요청으로 notify-server에 전송하고,
// 서버로부터 받은 응답 상태 코드를 기반으로 성공 여부를 판단합니다.
// 2xx 응답은 성공, 그 외는 모두 오류로 처리합니다.
func (c *Client) notify(ctx context.Context, message string, errorOccurred bool) error {
	if ctx == nil {
		panic("유효하지 않은 인자입니다: Context 객체는 nil일 수 없습니다")
	}

	// 앞뒤 공백만 있는 메시지는 의미 없는 알림이므로 거부합니다.
	message = strings.TrimSpace(message)
	if message == "" {
		return errors.New("유효하지 않은 인자입니다: 전송할 알림 메시지가 비어 있습니다")
	}

	body, err := json.Marshal(notifyRequest{
		ApplicationID: c.config.ApplicationID,
		Message:       message,
		ErrorOccurred: errorOccurred,
	})
	if err != nil {
		return fmt.Errorf("요청 데이터를 생성(직렬화)하는 중 오류가 발생했습니다: %w", err)
	}

	// bytes.NewReader를 사용하면 http.Request.GetBody가 자동으로 설정됩니다.
	// 덕분에 HTTP 클라이언트가 3xx 리다이렉트 등을 처리할 때 요청 본문을 자동으로 재전송할 수 있습니다.
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("HTTP 요청 인스턴스를 생성하는 중 오류가 발생했습니다: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("X-App-Key", c.config.AppKey)
	req.Header.Set("X-Application-Id", c.config.ApplicationID)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("notify-server로 알림 데이터를 전송하는 중 오류가 발생했습니다: %w", err)
	}
	defer res.Body.Close()

	// HTTP 상태 코드가 200~299 범위이면 성공으로 간주합니다.
	// 성공 응답이라도 Body를 끝까지 소비(drain)해야 net/http가 해당 TCP 커넥션을 커넥션 풀에 반환하여 다음 요청에서 재사용할 수 있습니다.
	// drain 자체의 실패는 비즈니스 로직 성공 여부에 영향을 주지 않으므로 오류를 무시합니다.
	// (drain 실패 시 해당 커넥션은 풀로 반환되지 않고 닫히며, 다음 요청에서 새 커넥션이 생성됩니다.)
	if res.StatusCode/100 == 2 {
		_, _ = io.Copy(io.Discard, res.Body)
		return nil
	}

	// 오류 응답의 경우 응답 바디를 읽어 오류 메시지에 포함시킵니다.
	// 단, 악의적으로 크거나 예상치 못하게 큰 바디로 인해 메모리를 과도하게 사용하지 않도록
	// LimitReader로 읽을 최대 바이트 수를 제한합니다.
	//
	// maxRespBodyBytes: 네트워크에서 읽을 최대 바이트 수입니다.
	//   UTF-8 인코딩에서 한국어 1글자는 최대 3바이트를 차지합니다.
	//   표시 한도인 maxErrBodyDisplayRunes(256) rune을 온전히 커버하려면
	//   최소 256 * 3 = 768바이트가 필요하므로, 이보다 넉넉한 1024를 사용합니다.
	//
	// maxErrBodyDisplayRunes: 오류 메시지에 노출할 최대 문자(rune) 수입니다.
	//   한국어처럼 멀티바이트 문자가 중간에 잘리지 않도록 바이트가 아닌 rune 단위로 제한합니다.
	const (
		maxRespBodyBytes       = 1024
		maxErrBodyDisplayRunes = 256
	)

	respBody, err := io.ReadAll(io.LimitReader(res.Body, maxRespBodyBytes))
	if err != nil {
		// 읽기 자체가 실패했다면 스트림이 이미 오류 상태이므로 추가 drain은 의미가 없습니다.
		// defer로 등록된 res.Body.Close()가 커넥션 정리를 담당합니다.
		return fmt.Errorf("notify-server의 응답 데이터를 읽는 중 오류가 발생했습니다: %w", err)
	}

	// LimitReader로 인해 최대 maxRespBodyBytes 바이트만 읽혔으므로,
	// 그보다 긴 응답 바디가 있을 경우 아직 스트림에 남아 있을 수 있습니다.
	// 커넥션 풀 반환을 위해 남은 데이터를 모두 버립니다.
	_, _ = io.Copy(io.Discard, res.Body)

	if len(respBody) > 0 {
		// 멀티바이트 문자(예: 한국어)가 바이트 경계에서 잘리는 것을 방지하기 위해
		// 바이트 슬라이스를 string으로 변환한 뒤 []rune으로 다시 변환하여 문자 단위로 제한합니다.
		bodyStr := strings.TrimSpace(string(respBody))
		bodyRunes := []rune(bodyStr)
		if len(bodyRunes) > maxErrBodyDisplayRunes {
			bodyStr = string(bodyRunes[:maxErrBodyDisplayRunes]) + "...(생략)"
		}

		return fmt.Errorf("notify-server에서 유효하지 않은 응답을 반환했습니다 (상태 코드: %d, 응답 내용: %s)", res.StatusCode, bodyStr)
	}

	return fmt.Errorf("notify-server에서 유효하지 않은 응답을 반환했습니다 (상태 코드: %d)", res.StatusCode)
}
