package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/pkg/notify"
)

// ────────────────────────────────────────────────────────────
// 헬퍼
// ────────────────────────────────────────────────────────────

// newTestClient는 테스트용 Config 기본값으로 *notify.Client를 생성합니다.
// 실패 시 즉시 t.Fatal을 호출합니다.
func newTestClient(t *testing.T, url string, opts ...notify.Option) *notify.Client {
	t.Helper()
	c, err := notify.NewClient(&notify.Config{
		URL:           url,
		AppKey:        "test-key",
		ApplicationID: "test-app",
	}, opts...)
	if err != nil {
		t.Fatalf("NewClient 생성 실패: %v", err)
	}
	return c
}

// notifyPayload는 클라이언트가 서버로 전송하는 JSON 바디 구조체입니다.
type notifyPayload struct {
	ApplicationID string `json:"application_id"`
	Message       string `json:"message"`
	ErrorOccurred bool   `json:"error_occurred"`
}

// decodePayload는 요청 바디를 notifyPayload로 역직렬화합니다.
func decodePayload(t *testing.T, r *http.Request) notifyPayload {
	t.Helper()
	var p notifyPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		t.Errorf("요청 바디 역직렬화 실패: %v", err)
	}
	return p
}

// ────────────────────────────────────────────────────────────
// TestNewClient
// ────────────────────────────────────────────────────────────

func TestNewClient(t *testing.T) {
	t.Run("nil Config 전달 시 에러 반환", func(t *testing.T) {
		c, err := notify.NewClient(nil)
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		if c != nil {
			t.Fatal("nil Client가 반환되어야 합니다")
		}
	})

	validBase := func() *notify.Config {
		return &notify.Config{
			URL:           "http://localhost",
			AppKey:        "key",
			ApplicationID: "id",
		}
	}

	cases := []struct {
		name    string
		mutate  func(*notify.Config)
		wantErr bool
	}{
		{
			name:    "정상 설정",
			mutate:  func(c *notify.Config) {},
			wantErr: false,
		},
		{
			name:    "URL 누락",
			mutate:  func(c *notify.Config) { c.URL = "" },
			wantErr: true,
		},
		{
			name:    "URL 스킴이 ftp인 경우",
			mutate:  func(c *notify.Config) { c.URL = "ftp://localhost" },
			wantErr: true,
		},
		{
			name:    "URL이 경로만 있고 호스트 없는 경우",
			mutate:  func(c *notify.Config) { c.URL = "/api/v1/notifications" },
			wantErr: true,
		},
		{
			name:    "AppKey 누락",
			mutate:  func(c *notify.Config) { c.AppKey = "" },
			wantErr: true,
		},
		{
			name:    "ApplicationID 누락",
			mutate:  func(c *notify.Config) { c.ApplicationID = "" },
			wantErr: true,
		},
		{
			name:    "Timeout 음수",
			mutate:  func(c *notify.Config) { c.Timeout = -1 * time.Second },
			wantErr: true,
		},
		{
			name: "앞뒤 공백 포함 필드도 TrimSpace 후 정상 생성",
			mutate: func(c *notify.Config) {
				c.URL = "  http://localhost  "
				c.AppKey = "  key  "
				c.ApplicationID = "  id  "
			},
			wantErr: false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBase()
			tc.mutate(cfg)
			client, err := notify.NewClient(cfg)
			if (err != nil) != tc.wantErr {
				t.Fatalf("wantErr=%v, got err=%v", tc.wantErr, err)
			}
			if !tc.wantErr && client == nil {
				t.Fatal("성공 케이스에서 nil Client가 반환되었습니다")
			}
		})
	}

	t.Run("Timeout=0이면 기본값 10s 적용", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		// Timeout을 명시하지 않으면(=0) 기본 10s가 적용됩니다.
		// 실제 Timeout 값을 직접 읽을 수 없으므로 Client가 정상 생성됨으로 검증합니다.
		cfg := &notify.Config{
			URL:           ts.URL,
			AppKey:        "key",
			ApplicationID: "id",
			Timeout:       0,
		}
		c, err := notify.NewClient(cfg)
		if err != nil {
			t.Fatalf("에러가 없어야 합니다: %v", err)
		}
		if c == nil {
			t.Fatal("Client가 반환되어야 합니다")
		}
	})

	t.Run("Timeout 명시 시 해당 값 사용 (요청 성공으로 간접 검증)", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		cfg := &notify.Config{
			URL:           ts.URL,
			AppKey:        "key",
			ApplicationID: "id",
			Timeout:       5 * time.Second,
		}
		c, err := notify.NewClient(cfg)
		if err != nil {
			t.Fatalf("에러가 없어야 합니다: %v", err)
		}
		// 실제로 요청을 보내 Timeout이 적용된 클라이언트가 정상 동작하는지 확인합니다.
		if err := c.Notify(context.Background(), "ping"); err != nil {
			t.Fatalf("알림 전송 실패: %v", err)
		}
	})

	t.Run("Config 불변성: NewClient 이후 원본 Config 변경이 클라이언트에 미영향", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			appID := r.Header.Get("X-Application-Id")
			if appID != "original-id" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		cfg := &notify.Config{
			URL:           ts.URL,
			AppKey:        "key",
			ApplicationID: "original-id",
		}
		c, err := notify.NewClient(cfg)
		if err != nil {
			t.Fatalf("에러가 없어야 합니다: %v", err)
		}

		// 원본 Config를 수정합니다.
		cfg.ApplicationID = "CHANGED-id"

		// 클라이언트는 여전히 "original-id"를 사용해야 합니다.
		if err := c.Notify(context.Background(), "test"); err != nil {
			t.Errorf("Config 불변성 위반: 원본 Config 수정이 클라이언트에 영향을 미쳤습니다: %v", err)
		}
	})
}

// ────────────────────────────────────────────────────────────
// TestWithHTTPClient
// ────────────────────────────────────────────────────────────

func TestWithHTTPClient(t *testing.T) {
	t.Run("nil 전달 시 패닉 발생", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("패닉이 발생해야 합니다")
			}
		}()
		// NewClient 내부에서 옵션을 적용하므로 이 시점에 패닉이 발생합니다.
		_, _ = notify.NewClient(
			&notify.Config{
				URL:           "http://localhost",
				AppKey:        "key",
				ApplicationID: "id",
			},
			notify.WithHTTPClient(nil),
		)
	})

	t.Run("Timeout=0인 외부 클라이언트 주입 → Config.Timeout 폴백 적용", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		// Timeout=0(무제한)인 외부 클라이언트
		externalHC := &http.Client{Timeout: 0}

		c, err := notify.NewClient(
			&notify.Config{
				URL:           ts.URL,
				AppKey:        "key",
				ApplicationID: "id",
				Timeout:       3 * time.Second, // 이 값이 폴백으로 적용되어야 합니다.
			},
			notify.WithHTTPClient(externalHC),
		)
		if err != nil {
			t.Fatalf("에러가 없어야 합니다: %v", err)
		}

		// 외부 클라이언트의 원본 Timeout은 그대로입니다(얕은 복사 확인).
		if externalHC.Timeout != 0 {
			t.Errorf("외부 클라이언트의 원본 Timeout이 변경되었습니다: got %v", externalHC.Timeout)
		}

		// 실제 요청이 성공해야 합니다.
		if err := c.Notify(context.Background(), "ping"); err != nil {
			t.Fatalf("알림 전송 실패: %v", err)
		}
	})

	t.Run("Timeout>0인 외부 클라이언트 주입 → Config.Timeout 무시", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		// Timeout이 설정된 외부 클라이언트
		externalHC := &http.Client{Timeout: 7 * time.Second}

		c, err := notify.NewClient(
			&notify.Config{
				URL:           ts.URL,
				AppKey:        "key",
				ApplicationID: "id",
				Timeout:       1 * time.Second, // 이 값은 무시되어야 합니다.
			},
			notify.WithHTTPClient(externalHC),
		)
		if err != nil {
			t.Fatalf("에러가 없어야 합니다: %v", err)
		}

		// 외부 클라이언트의 Timeout이 1s로 변경되지 않아야 합니다.
		if externalHC.Timeout != 7*time.Second {
			t.Errorf("외부 클라이언트의 Timeout이 변경되었습니다: got %v", externalHC.Timeout)
		}

		if err := c.Notify(context.Background(), "ping"); err != nil {
			t.Fatalf("알림 전송 실패: %v", err)
		}
	})
}

// ────────────────────────────────────────────────────────────
// TestClient_Notify
// ────────────────────────────────────────────────────────────

func TestClient_Notify(t *testing.T) {
	t.Run("nil context 전달 시 패닉 발생", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("패닉이 발생해야 합니다")
			}
		}()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		//nolint:staticcheck // nil context 패닉 테스트 의도
		_ = c.Notify(nil, "message") //nolint:SA1012
	})

	t.Run("빈 메시지 전송 차단", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		c := newTestClient(t, ts.URL)

		for _, msg := range []string{"", "   ", "\t\n"} {
			if err := c.Notify(context.Background(), msg); err == nil {
				t.Errorf("빈 메시지 %q에 대해 에러가 반환되어야 합니다", msg)
			}
		}
	})

	t.Run("context 취소 시 에러 반환", func(t *testing.T) {
		// 서버가 응답을 지연하는 상황을 시뮬레이션합니다.
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 요청이 취소되기를 기다립니다.
			<-r.Context().Done()
		}))
		defer ts.Close()
		c := newTestClient(t, ts.URL)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // 즉시 취소

		err := c.Notify(ctx, "message")
		if err == nil {
			t.Fatal("취소된 context에 대해 에러가 반환되어야 합니다")
		}
		if !errors.Is(err, context.Canceled) {
			t.Logf("에러 메시지: %v", err) // errors.Is 불일치 시 로그만 남깁니다.
		}
	})

	t.Run("정상 전송 - 요청 메서드/헤더/바디 전체 검증", func(t *testing.T) {
		var capturedReq *http.Request
		var capturedPayload notifyPayload

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedReq = r
			capturedPayload = decodePayload(t, r)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		cfg := &notify.Config{
			URL:           ts.URL,
			AppKey:        "my-app-key",
			ApplicationID: "my-app-id",
		}
		c, err := notify.NewClient(cfg)
		if err != nil {
			t.Fatalf("NewClient 실패: %v", err)
		}

		if err := c.Notify(context.Background(), "배포가 완료되었습니다."); err != nil {
			t.Fatalf("Notify 실패: %v", err)
		}

		// HTTP 메서드 검증
		if capturedReq.Method != http.MethodPost {
			t.Errorf("Method: got %s, want POST", capturedReq.Method)
		}
		// 헤더 검증
		for header, want := range map[string]string{
			"Content-Type":     "application/json",
			"Cache-Control":    "no-cache",
			"X-App-Key":        "my-app-key",
			"X-Application-Id": "my-app-id",
		} {
			if got := capturedReq.Header.Get(header); got != want {
				t.Errorf("Header[%s]: got %q, want %q", header, got, want)
			}
		}
		// JSON 바디 검증
		if capturedPayload.ApplicationID != "my-app-id" {
			t.Errorf("payload.application_id: got %q, want %q", capturedPayload.ApplicationID, "my-app-id")
		}
		if capturedPayload.Message != "배포가 완료되었습니다." {
			t.Errorf("payload.message: got %q, want %q", capturedPayload.Message, "배포가 완료되었습니다.")
		}
		if capturedPayload.ErrorOccurred {
			t.Error("payload.error_occurred: got true, want false (Notify는 false여야 합니다)")
		}
	})

	t.Run("2xx 응답 코드 모두 성공으로 처리", func(t *testing.T) {
		for _, statusCode := range []int{200, 201, 204} {
			statusCode := statusCode
			t.Run(fmt.Sprintf("HTTP %d", statusCode), func(t *testing.T) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(statusCode)
				}))
				defer ts.Close()

				c := newTestClient(t, ts.URL)
				if err := c.Notify(context.Background(), "message"); err != nil {
					t.Errorf("HTTP %d는 성공으로 처리되어야 합니다: %v", statusCode, err)
				}
			})
		}
	})

	t.Run("4xx/5xx 응답 → 에러 반환 + 상태 코드 포함", func(t *testing.T) {
		for _, statusCode := range []int{400, 401, 403, 404, 500, 503} {
			statusCode := statusCode
			t.Run(fmt.Sprintf("HTTP %d", statusCode), func(t *testing.T) {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(statusCode)
				}))
				defer ts.Close()

				c := newTestClient(t, ts.URL)
				err := c.Notify(context.Background(), "message")
				if err == nil {
					t.Fatalf("HTTP %d는 에러를 반환해야 합니다", statusCode)
				}
				if !strings.Contains(err.Error(), fmt.Sprintf("%d", statusCode)) {
					t.Errorf("에러 메시지에 상태 코드 %d가 포함되어야 합니다: %v", statusCode, err)
				}
			})
		}
	})

	t.Run("에러 응답 바디 있음 → 에러 메시지에 바디 내용 포함", func(t *testing.T) {
		const errMsg = "잘못된 요청입니다"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, errMsg)
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		err := c.Notify(context.Background(), "message")
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		if !strings.Contains(err.Error(), errMsg) {
			t.Errorf("에러 메시지에 응답 바디 내용이 포함되어야 합니다. got: %v", err)
		}
	})

	t.Run("에러 응답 바디 없음 → 상태 코드만 포함된 에러 반환", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			// 바디 없음
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		err := c.Notify(context.Background(), "message")
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("에러 메시지에 상태 코드 500이 포함되어야 합니다: %v", err)
		}
	})

	t.Run("에러 응답 바디 1024바이트 초과 → 읽기 제한 후 나머지 drain", func(t *testing.T) {
		// 2048바이트 응답을 내려도 클라이언트가 hang 없이 정상 처리해야 합니다.
		largeBody := strings.Repeat("x", 2048)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, largeBody)
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		err := c.Notify(context.Background(), "message")
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		// 1024바이트만 읽혔으므로 에러 메시지 내에 2048개의 'x'가 전부 포함되지 않아야 합니다.
		if strings.Contains(err.Error(), largeBody) {
			t.Error("에러 메시지에 전체 largeBody가 포함되지 않아야 합니다 (LimitReader 미적용 의심)")
		}
	})

	t.Run("에러 응답 바디의 한국어 256 rune 초과 → '...(생략)' 포함", func(t *testing.T) {
		// 한국어 한 글자는 3바이트, 257 rune이므로 maxErrBodyDisplayRunes(256)을 초과합니다.
		koreanBody := strings.Repeat("가", 257)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, koreanBody)
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		err := c.Notify(context.Background(), "message")
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		if !strings.Contains(err.Error(), "...(생략)") {
			t.Errorf("에러 메시지에 '...(생략)'이 포함되어야 합니다: %v", err)
		}
	})

	t.Run("에러 응답 바디의 한국어 256 rune 이하 → 잘리지 않고 전체 포함", func(t *testing.T) {
		koreanBody := strings.Repeat("나", 256)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, koreanBody)
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		err := c.Notify(context.Background(), "message")
		if err == nil {
			t.Fatal("에러가 반환되어야 합니다")
		}
		if strings.Contains(err.Error(), "...(생략)") {
			t.Errorf("256 rune 이하에서는 생략 표시가 없어야 합니다: %v", err)
		}
		if !strings.Contains(err.Error(), koreanBody) {
			t.Errorf("에러 메시지에 한국어 바디가 전체 포함되어야 합니다: %v", err)
		}
	})

	t.Run("네트워크 오류 (서버 즉시 종료) → 에러 반환", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		ts.Close() // 즉시 종료하여 연결 불가 상태로 만듭니다.

		c := newTestClient(t, ts.URL)
		if err := c.Notify(context.Background(), "message"); err == nil {
			t.Fatal("네트워크 오류 시 에러가 반환되어야 합니다")
		}
	})
}

// ────────────────────────────────────────────────────────────
// TestClient_NotifyError
// ────────────────────────────────────────────────────────────

func TestClient_NotifyError(t *testing.T) {
	t.Run("error_occurred 필드가 true로 전송됨", func(t *testing.T) {
		var capturedPayload notifyPayload

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPayload = decodePayload(t, r)
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		c := newTestClient(t, ts.URL)
		if err := c.NotifyError(context.Background(), "DB 연결 실패"); err != nil {
			t.Fatalf("NotifyError 실패: %v", err)
		}

		if !capturedPayload.ErrorOccurred {
			t.Error("payload.error_occurred: got false, want true (NotifyError는 true여야 합니다)")
		}
		if capturedPayload.Message != "DB 연결 실패" {
			t.Errorf("payload.message: got %q, want %q", capturedPayload.Message, "DB 연결 실패")
		}
	})

	t.Run("빈 메시지 전송 차단", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()
		c := newTestClient(t, ts.URL)

		if err := c.NotifyError(context.Background(), "   "); err == nil {
			t.Error("빈 메시지에 대해 에러가 반환되어야 합니다")
		}
	})

	t.Run("Notify와 JSON 바디의 error_occurred만 다름", func(t *testing.T) {
		results := map[string]bool{}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var p notifyPayload
			_ = json.NewDecoder(r.Body).Decode(&p)

			// 경로로 구분: 실제 notify-server는 단일 엔드포인트이므로 헤더 값으로 구분합니다.
			results[r.Header.Get("X-Test-Type")] = p.ErrorOccurred
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		cfg := &notify.Config{
			URL:           ts.URL,
			AppKey:        "key",
			ApplicationID: "id",
		}

		// Notify 요청
		hc := &http.Client{Transport: &headerInjectTransport{
			wrapped: http.DefaultTransport,
			header:  "X-Test-Type",
			value:   "notify",
		}}
		c1, _ := notify.NewClient(cfg, notify.WithHTTPClient(hc))
		_ = c1.Notify(context.Background(), "msg")

		// NotifyError 요청
		hc2 := &http.Client{Transport: &headerInjectTransport{
			wrapped: http.DefaultTransport,
			header:  "X-Test-Type",
			value:   "notifyError",
		}}
		c2, _ := notify.NewClient(cfg, notify.WithHTTPClient(hc2))
		_ = c2.NotifyError(context.Background(), "msg")

		if results["notify"] {
			t.Error("Notify의 error_occurred는 false여야 합니다")
		}
		if !results["notifyError"] {
			t.Error("NotifyError의 error_occurred는 true여야 합니다")
		}
	})
}

// ────────────────────────────────────────────────────────────
// 테스트 유틸리티 Transport
// ────────────────────────────────────────────────────────────

// headerInjectTransport는 요청에 지정된 헤더를 추가하는 테스트용 Transport입니다.
type headerInjectTransport struct {
	wrapped http.RoundTripper
	header  string
	value   string
}

func (t *headerInjectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 원본 요청을 수정하지 않도록 얕은 복사합니다.
	cloned := req.Clone(req.Context())
	cloned.Header.Set(t.header, t.value)
	return t.wrapped.RoundTrip(cloned)
}

// ────────────────────────────────────────────────────────────
// 커버리지 검증용 - 응답 바디 읽기 실패 시나리오
// ────────────────────────────────────────────────────────────

// errorReader는 Read 호출 시 항상 에러를 반환하는 테스트용 io.Reader입니다.
type errorReader struct{}

func (e *errorReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}
func (e *errorReader) Close() error { return nil }

// brokenBodyTransport는 응답 바디를 errorReader로 교체하는 테스트용 Transport입니다.
// 이를 통해 io.ReadAll 실패 경로를 테스트합니다.
type brokenBodyTransport struct {
	wrapped http.RoundTripper
}

func (t *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.wrapped.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	// 응답 바디를 에러를 반환하는 Reader로 교체합니다.
	resp.Body = io.NopCloser(&errorReader{})
	return resp, nil
}

func TestClient_Notify_ResponseBodyReadError(t *testing.T) {
	t.Run("에러 응답 바디 읽기 실패 → 에러 반환", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, "server error")
		}))
		defer ts.Close()

		hc := &http.Client{
			Transport: &brokenBodyTransport{wrapped: http.DefaultTransport},
		}
		c, err := notify.NewClient(
			&notify.Config{
				URL:           ts.URL,
				AppKey:        "key",
				ApplicationID: "id",
			},
			notify.WithHTTPClient(hc),
		)
		if err != nil {
			t.Fatalf("NewClient 실패: %v", err)
		}

		if err := c.Notify(context.Background(), "message"); err == nil {
			t.Fatal("응답 바디 읽기 실패 시 에러가 반환되어야 합니다")
		}
	})
}