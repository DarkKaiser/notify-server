package config

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Unit Tests: Helper Functions (checkUniqueField, checkStruct)
// =============================================================================

// TestCheckStruct는 구조체 유효성 검증 로직과 에러 메시지 포맷팅을 검증합니다.
// checkStruct 함수가 validator 라이브러리를 올바르게 래핑하고 있는지 확인합니다.
func TestCheckStruct(t *testing.T) {
	t.Parallel()

	// 테스트용 구조체 정의
	type SubConfig struct {
		ID string `json:"id" validate:"required"`
	}

	type TestConfig struct {
		Name     string      `json:"name" validate:"required"`
		Age      int         `json:"age" validate:"min=18"`
		Commands []string    `json:"commands" validate:"min=1"` // Min 태그 메시지 테스트용
		Tasks    []SubConfig `json:"tasks" validate:"unique=ID"`
	}

	tests := []struct {
		name          string
		input         TestConfig
		contextName   string
		fields        []string // 부분 검증(Partial Validation) 대상 필드
		shouldError   bool
		errorContains string
	}{
		// 1. 기본 유효성 검증 (Happy Path & Basic Validations)
		{
			name:        "Valid Struct",
			input:       TestConfig{Name: "John", Age: 20, Commands: []string{"cmd"}, Tasks: []SubConfig{{ID: "t1"}}},
			contextName: "User",
			shouldError: false,
		},
		{
			name:          "Missing Required Field (Name)",
			input:         TestConfig{Age: 20, Commands: []string{"cmd"}}, // Name 누락
			contextName:   "User",
			shouldError:   true,
			errorContains: "User의 설정이 올바르지 않습니다: name (조건: required)",
		},
		{
			name:          "Validation Failed (Min Age)",
			input:         TestConfig{Name: "John", Age: 16, Commands: []string{"cmd"}}, // Age < 18
			contextName:   "User",
			shouldError:   true,
			errorContains: "User의 설정이 올바르지 않습니다: age (조건: min)",
		},

		// 2. 부분 검증 (Partial Validation)
		{
			name:        "Partial Validation: Ignore Missing Required Field",
			input:       TestConfig{Age: 20}, // Name 누락되었지만 Age만 검사
			contextName: "User",
			fields:      []string{"Age"},
			shouldError: false, // Age는 유효하므로 패스
		},
		{
			name:          "Partial Validation: Catch Invalid Field",
			input:         TestConfig{Name: "John", Age: 10}, // Age < 18
			contextName:   "User",
			fields:        []string{"Age"},
			shouldError:   true,
			errorContains: "User의 설정이 올바르지 않습니다: age (조건: min)",
		},

		// 3. 커스텀 에러 메시지 핸들링 (checkStruct 내부 로직)
		{
			name:          "Duplicate ID in Tasks (unique tag)",
			input:         TestConfig{Name: "John", Age: 20, Commands: []string{"c"}, Tasks: []SubConfig{{ID: "dup"}, {ID: "dup"}}},
			contextName:   "User",
			shouldError:   true,
			errorContains: "User 내에 중복된 작업(Task) ID가 존재합니다", // target: 'tasks' -> '작업(Task)' 매핑 확인
		},
		{
			name:          "Commands Min Validation Custom Message",
			input:         TestConfig{Name: "John", Age: 20, Commands: []string{}},
			contextName:   "User",
			shouldError:   true,
			errorContains: "작업(Task)은 최소 1개 이상의 명령(Command)를 포함해야 합니다",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := checkStruct(v, tt.input, tt.contextName, tt.fields...)

			if tt.shouldError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestCheckStruct_CommandsMessageVerify는 checkStruct 내부의 "Commands" 필드에 대한 특수 처리 로직을 검증합니다.
// 실제 AppConfig 구조체와 유사한 상황을 시뮬레이션합니다.
func TestCheckStruct_CommandsMessageVerify(t *testing.T) {
	t.Parallel()

	type MockTaskConfig struct {
		Commands []string `json:"commands" validate:"min=1"`
	}

	input := MockTaskConfig{Commands: []string{}} // Empty commands
	v := newValidator()
	err := checkStruct(v, input, "MockTask")

	require.Error(t, err)
	// validator.go: case "Commands" -> if tag == "min" -> "작업(Task)은 최소 1개 이상의 명령(Command)를 포함해야 합니다"
	assert.Equal(t, "[InvalidInput] 작업(Task)은 최소 1개 이상의 명령(Command)를 포함해야 합니다", err.Error())
}

// =============================================================================
// Unit Tests: Infrastructure (JSON Tag Name Func)
// =============================================================================

// TestValidate_Infrastructure_JSONTagName은 에러 메시지에 구조체 필드명 대신 JSON 태그명이 사용되는지 확인합니다.
func TestValidate_Infrastructure_JSONTagName(t *testing.T) {
	t.Parallel()

	type TestStruct struct {
		RequiredField string `json:"required_field" validate:"required"`
		OmitField     string `json:"omit_field,omitempty" validate:"required"`
		NoTagField    string `validate:"required"`
		DashTagField  string `json:"-" validate:"required"`
	}

	tests := []struct {
		name          string
		input         TestStruct
		expectedValid bool
		errorContains string
	}{
		{
			name:          "Required Field Missing (JSON Tag)",
			input:         TestStruct{},
			expectedValid: false,
			errorContains: "required_field", // json tag name
		},
		{
			name:          "No JSON Tag (Fallback to Field Name)",
			input:         TestStruct{RequiredField: "valid", OmitField: "valid"},
			expectedValid: false,
			errorContains: "NoTagField", // field name
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := checkStruct(v, tt.input, "TestStruct")
			if !tt.expectedValid {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// Unit Tests: Custom Validators (Telegram Bot Token, CORS)
// =============================================================================

// TestValidate_Unit_TelegramBotToken은 텔레그램 봇 토큰 유효성 검증 로직을 테스트합니다.
func TestValidate_Unit_TelegramBotToken(t *testing.T) {
	t.Parallel()

	type BotTokenStruct struct {
		Token string `validate:"telegram_bot_token"`
	}

	tests := []struct {
		name  string
		token string
		valid bool
	}{
		// Valid cases
		{"Valid Token", "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true},
		{"Valid Token (Minimum Length)", "123:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true}, // ID 3자리
		{"Valid Token (Long ID)", "12345678901234567890:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", true},

		// Invalid cases
		{"Empty Token", "", false},
		{"No Separator", "123456789ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},                 // 콜론 없음
		{"ID Too Short", "12:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},                       // ID 2자리 (최소 3자리)
		{"ID Not Numeric", "ABC:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", false},                    // ID가 숫자가 아님
		{"Secret Too Short", "123456789:ShortSecret", false},                                   // Secret < 30자
		{"Secret Contains Special Char", "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123@#$", false}, // 허용되지 않은 특수문자
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := v.Struct(BotTokenStruct{Token: tt.token})
			if tt.valid {
				assert.NoError(t, err, "Token '%s' should be valid", tt.token)
			} else {
				assert.Error(t, err, "Token '%s' should be invalid", tt.token)
			}
		})
	}
}

// TestValidate_Unit_CORSOrigin은 CORS Origin 유효성 검증 로직을 집중적으로 테스트합니다.
func TestValidate_Unit_CORSOrigin(t *testing.T) {
	t.Parallel()

	type CORSStruct struct {
		Origin string `validate:"cors_origin"`
	}

	tests := []struct {
		name   string
		origin string
		valid  bool
	}{
		// Valid cases
		{"Wildcard", "*", true},
		{"HTTP Localhost", "http://localhost", true},
		{"HTTPS Example", "https://example.com", true},
		{"HTTP with Port", "http://localhost:8080", true},
		{"HTTPS with Port", "https://example.com:8443", true},
		{"Subdomain", "https://api.example.com", true},
		{"IP Address", "http://127.0.0.1", true},
		{"IP with Port", "http://192.168.0.1:3000", true},

		// Invalid cases
		{"Empty String", "", false},
		{"Missing Scheme", "example.com", false},
		{"Unsupported Scheme (FTP)", "ftp://example.com", false},
		{"Just Scheme", "http://", false},
		{"Leading Whitespace", " https://example.com", false},
		{"Trailing Whitespace", "https://example.com ", false},
		{"Trailing Slash", "https://example.com/", false},           // Origin은 경로를 포함하면 안 됨 (Trailing Slash 포함)
		{"Path Included", "https://example.com/api", false},         // 경로 포함
		{"Query String Included", "https://example.com?q=1", false}, // 쿼리 포함
		{"Fragment Included", "https://example.com#hash", false},    // 프래그먼트 포함
		{"Invalid Port", "http://example.com:999999", false},        // 포트 범위 초과 (엄밀히는 net/url 파싱에 의존)
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := newValidator()
			err := v.Struct(CORSStruct{Origin: tt.origin})
			if tt.valid {
				assert.NoError(t, err, "Origin '%s' should be valid", tt.origin)
			} else {
				assert.Error(t, err, "Origin '%s' should be invalid", tt.origin)
			}
		})
	}
}

// TestCheckStruct_TelegramBotToken_NoLeak는 에러 메시지에 민감한 토큰 정보가 노출되지 않는지 확인합니다.
func TestCheckStruct_TelegramBotToken_NoLeak(t *testing.T) {
	t.Parallel()

	type BotTokenStruct struct {
		Token string `json:"bot_token" validate:"telegram_bot_token"`
	}

	invalidToken := "123456789:SHORT-SECRET" // 유효하지 않은 토큰
	input := BotTokenStruct{Token: invalidToken}

	v := newValidator()
	err := checkStruct(v, input, "TestBot")

	require.Error(t, err)
	// 기대 메시지: "텔레그램 BotToken 형식이 올바르지 않습니다 ..."
	// 실제 토큰 값이 포함되면 안 됨
	assert.Contains(t, err.Error(), "텔레그램 BotToken 형식이 올바르지 않습니다")
	assert.NotContains(t, err.Error(), invalidToken, "에러 메시지에 민감한 토큰 값이 포함되면 안 됩니다")
}

// TestCheckStruct_FieldSpecificErrors는 checkStruct 함수 내 switch 문으로 처리되는 필드별 커스텀 에러를 검증합니다.
func TestCheckStruct_FieldSpecificErrors(t *testing.T) {
	t.Parallel()

	// MaxRetries 테스트
	type RetryConfig struct {
		MaxRetries int `json:"max_retries" validate:"gte=0"`
	}
	err := checkStruct(newValidator(), RetryConfig{MaxRetries: -1}, "Retry")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 최대 재시도 횟수(max_retries)는 0 이상이어야 합니다")

	// RetryDelay 테스트
	type DelayConfig struct {
		RetryDelay int `json:"retry_delay" validate:"gt=0"`
	}
	err = checkStruct(newValidator(), DelayConfig{RetryDelay: 0}, "Delay")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 재시도 대기 시간(retry_delay)은 0보다 커야 합니다")

	// ListenPort 테스트 (Web Service context)
	type PortConfig struct {
		ListenPort int `json:"listen_port" validate:"min=1,max=65535"`
	}
	err = checkStruct(newValidator(), PortConfig{ListenPort: 70000}, "Port")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "웹 서비스 포트(listen_port)는 1에서 65535 사이의 값이어야 합니다")
}

// TestNormalizeContextName은 에러 메시지 생성 시 구조체 이름이나 필드명이 아닌,
// 사용자가 전달한 contextName이 올바르게 사용되는지 확인합니다.
func TestNormalizeContextName(t *testing.T) {
	t.Parallel()

	type Simple struct {
		Val string `validate:"required"`
	}

	// 1. 일반적인 Context Name ("User")
	err := checkStruct(newValidator(), Simple{}, "User")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "User의 설정이 올바르지 않습니다")

	// 2. 한글 Context Name ("사용자 설정")
	err = checkStruct(newValidator(), Simple{}, "사용자 설정")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "사용자 설정의 설정이 올바르지 않습니다")
}

// TestFormatter_UniqueTag_Messages는 unique 태그 위반 시 타겟 필드명에 따른 한글 메시지 변환을 테스트합니다.
func TestFormatter_UniqueTag_Messages(t *testing.T) {
	t.Parallel()

	v := newValidator()

	// 1. Tasks -> 작업(Task)
	type TasksCfg struct {
		Tasks []struct {
			ID string `json:"id" validate:"required"`
		} `json:"tasks" validate:"unique=ID"`
	}
	inputTasks := TasksCfg{Tasks: []struct {
		ID string `json:"id" validate:"required"`
	}{{ID: "a"}, {ID: "a"}}}

	err := checkStruct(v, inputTasks, "Root")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Root 내에 중복된 작업(Task) ID가 존재합니다")

	// 2. Applications -> 애플리케이션(Application)
	type AppsCfg struct {
		Applications []struct {
			ID string `json:"id" validate:"required"`
		} `json:"applications" validate:"unique=ID"`
	}
	inputApps := AppsCfg{Applications: []struct {
		ID string `json:"id" validate:"required"`
	}{{ID: "b"}, {ID: "b"}}}

	err = checkStruct(v, inputApps, "Root")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Root 내에 중복된 애플리케이션(Application) ID가 존재합니다")

	// 3. Telegrams -> 알림 채널
	type TeleCfg struct {
		Telegrams []struct {
			ID string `json:"id"`
		} `json:"telegrams" validate:"unique=ID"`
	}
	inputTele := TeleCfg{Telegrams: []struct {
		ID string `json:"id"`
	}{{ID: "c"}, {ID: "c"}}}

	err = checkStruct(v, inputTele, "Root")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Root 내에 중복된 알림 채널 ID가 존재합니다")
}

// TestLargeStruct performance bench-like test (Simple sanity check only)
func TestLargeStruct_Sanity(t *testing.T) {
	// 1000개의 아이템이 있는 슬라이스 검증 (unique)
	// 성능보다는 스택 오버플로우나 타임아웃 없이 도는지 확인
	type Item struct {
		ID string `json:"id"`
	}
	type LargeCfg struct {
		Items []Item `json:"items" validate:"unique=ID"`
	}

	items := make([]Item, 1000)
	for i := 0; i < 1000; i++ {
		items[i] = Item{ID: fmt.Sprintf("item-%d", i)}
	}
	// Make duplicates at the end
	items[999].ID = "item-0"

	input := LargeCfg{Items: items}
	err := checkStruct(newValidator(), input, "Large")
	require.Error(t, err)
	// unique 태그는 일반적인 경우 필드명 그대로 출력 (매핑에 없으므로)
	// validator.go의 switch case에 "items"는 없으므로 default로 빠질 것임?
	// 아니면 unique 태그 핸들러에서 target default?
	// validator.go를 보면 default일 경우 target 변수 그대로(json name) 사용.
	assert.Contains(t, err.Error(), "Large 내에 중복된 items ID가 존재합니다")
}
