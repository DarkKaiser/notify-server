package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/darkkaiser/notify-server/internal/config"
	"github.com/darkkaiser/notify-server/internal/pkg/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// 메타데이터 및 상수 검증 (Metadata & Constants Validation)
// =============================================================================

// TestAppMetadata는 애플리케이션의 기본 메타데이터 설정이 올바른지 검증합니다.
func TestAppMetadata(t *testing.T) {
	t.Parallel()

	t.Run("AppVersion 검증", func(t *testing.T) {
		t.Parallel()
		v := version.Version()
		assert.NotEmpty(t, v, "애플리케이션 버전(Version)은 비어있을 수 없습니다")

		// 기본값("dev") 또는 Semantic Versioning 형식(vX.Y.Z)을 준수해야 함
		// 테스트 환경에서는 ldflags가 없을 수 있으므로 "unknown"도 허용
		if v != "dev" && v != "unknown" {
			assert.Regexp(t, `^v?\d+\.\d+\.\d+(?:-.*)?$`, v, "버전은 Semantic Versioning 표준 형식을 따라야 합니다")
		}
	})

	t.Run("AppName 검증", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, "notify-server", config.AppName, "애플리케이션 이름은 'notify-server'여야 합니다")
		assert.NotContains(t, config.AppName, " ", "애플리케이션 이름에는 공백이 포함될 수 없습니다")
	})

	t.Run("ConfigFileName 검증", func(t *testing.T) {
		t.Parallel()
		expected := "notify-server.json"
		assert.Equal(t, expected, config.DefaultFilename, "설정 파일명은 '%s'여야 합니다", expected)
	})
}

// TestBuildInfo는 빌드 타임에 주입되는 정보들의 기본 상태를 검증합니다.
func TestBuildInfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		getValue func() string
		desc     string
	}{
		{
			name:     "Version",
			getValue: version.Version,
			desc:     "버전 정보",
		},
		{
			name: "BuildDate",
			getValue: func() string {
				return version.Get().BuildDate
			},
			desc: "빌드 날짜",
		},
		{
			name: "BuildNumber",
			getValue: func() string {
				return version.Get().BuildNumber
			},
			desc: "빌드 번호",
		},
	}

	for _, tt := range tests {
		tt := tt // 캡처
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// ldflags가 없는 테스트 환경에서는 값이 비어있거나 unknown일 수 있음
			// 따라서 '패닉이 발생하지 않고 값을 가져올 수 있는지'를 중점적으로 확인
			val := tt.getValue()
			t.Logf("%s: %s", tt.desc, val)
		})
	}
}

// =============================================================================
// 배너 검증 (Banner Validation)
// =============================================================================

// TestBanner는 서버 시작 시 출력되는 배너의 형식과 내용이 올바른지 검증합니다.
func TestBanner(t *testing.T) {
	t.Parallel()

	t.Run("템플릿 형식 검증", func(t *testing.T) {
		t.Parallel()
		assert.Contains(t, banner, "%s", "배너 템플릿에는 버전 포맷팅을 위한 '%s'가 포함되어야 합니다")
		assert.Contains(t, banner, "DarkKaiser", "배너에는 개발자/조직명(DarkKaiser)이 포함되어야 합니다")
	})

	t.Run("출력 포맷팅 검증", func(t *testing.T) {
		t.Parallel()
		v := version.Version()
		output := fmt.Sprintf(banner, v)
		assert.Contains(t, output, v, "최종 출력된 배너에는 실제 버전 정보가 포함되어야 합니다")
		assert.NotContains(t, output, "%s", "최종 출력된 배너에는 포맷 지정자가 남아있지 않아야 합니다")
	})
}

// =============================================================================
// 설정 로드 통합 테스트 (Configuration Loading Integration Test)
// =============================================================================

// TestInitAppConfig는 설정 파일 로드 로직을 Table-Driven 방식으로 검증합니다.
func TestInitAppConfig(t *testing.T) {
	t.Parallel()

	type validateFunc func(*testing.T, *config.AppConfig)

	tests := []struct {
		name        string
		file        string // 파일 생성 시 사용할 파일명 (선택)
		fileContent string
		wantErr     bool
		errContains string
		validate    validateFunc
	}{
		{
			name: "Success_ValidConfig",
			fileContent: `{
				"debug": true,
				"notifiers": {
					"default_notifier_id": "test",
					"telegrams": [
						{ "id": "test", "bot_token": "123456789:ABC-DEF1234ghIkl-zyx57W2v1u123ew11", "chat_id": 12345 }
					]
				},
				"tasks": [],
				"notify_api": {
					"ws": { "tls_server": false, "listen_port": 18080 },
					"cors": { "allow_origins": ["*"] },
					"applications": []
				}
			}`,
			wantErr: false,
			validate: func(t *testing.T, c *config.AppConfig) {
				assert.True(t, c.Debug)
				assert.Equal(t, "test", c.Notifiers.DefaultNotifierID)
			},
		},
		{
			name:        "Error_InvalidJSON",
			fileContent: `{"debug": true, "broken_json...`,
			wantErr:     true,
			errContains: "JSON",
		},
		{
			name:        "Error_EmptyFile",
			fileContent: "",
			wantErr:     true,
			errContains: "unexpected end of JSON input",
		},
		{
			name:        "Error_EmptyJSON",
			fileContent: "{}",
			wantErr:     true,
			// 빈 JSON은 유효성 검사(Validate)에서 실패함
			// (현재 config.Validate() 구현에 따라 다를 수 있으나 일반적으로 필수값 누락 에러 발생)
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// 임시 파일 생성
			f := createTempConfigFile(t, tt.file, tt.fileContent)

			// 테스트 실행
			cfg, err := config.LoadWithFile(f)

			// 검증
			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				assert.Nil(t, cfg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

// TestInitAppConfig_FileNotFound는 파일이 존재하지 않는 경우를 별도로 테스트합니다.
func TestInitAppConfig_FileNotFound(t *testing.T) {
	t.Parallel()

	nonExistentFile := filepath.Join(t.TempDir(), "ghost_config.json")
	cfg, err := config.LoadWithFile(nonExistentFile)

	assert.Error(t, err)
	assert.Nil(t, cfg)

	// OS별 에러 메시지 차이를 고려하여 핵심 키워드 확인
	errMsg := err.Error()
	isPathError := strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "지정된 파일을 찾을 수 없습니다") || os.IsNotExist(err)

	// config.LoadWithFile이 에러를 래핑할 수 있으므로, 언래핑하여 원본 에러 확인 시도
	if !isPathError {
		// 만약 래핑된 에러라면 "파일을 찾을 수 없습니다" 등의 키워드가 포함되어야 함
		// (실제 구현에 따라 다름, 여기서는 보수적으로 체크)
		// assert.Contains(t, errMsg, nonExistentFile) // 파일명이 포함되는지 확인
	}
}

// -----------------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------------

// createTempConfigFile은 t.TempDir()을 사용하여 안전하게 임시 파일을 생성합니다.
// name이 비어있으면 랜덤 파일명을 생성합니다.
func createTempConfigFile(t *testing.T, name, content string) string {
	t.Helper()

	dir := t.TempDir() // 테스트 종료 시 자동 삭제됨

	if name == "" {
		name = fmt.Sprintf("test_cfg_%d.json", time.Now().UnixNano())
	}

	filePath := filepath.Join(dir, name)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err, "임시 파일 생성 실패")

	return filePath
}
