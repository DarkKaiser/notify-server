package version

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Info Tests
// =============================================================================

// TestInfo는 Info 구조체의 필드 값이 올바르게 설정되는지 검증합니다.
//
// 검증 항목:
//   - 모든 필드가 설정된 경우
//   - 빈 Info 구조체
//   - 부분적으로 설정된 경우
//   - 다양한 버전 형식 (Semantic Versioning, Git Hash, dev)
//   - 다양한 날짜 형식 (ISO 8601, 간단한 날짜)
//   - 특수 문자 및 공백 처리
func TestInfo(t *testing.T) {
	tests := []struct {
		name        string
		buildInfo   Info
		wantVersion string
		wantDate    string
		wantNumber  string
	}{
		// =================================================================
		// Complete Info
		// =================================================================
		{
			name: "Full build info",
			buildInfo: Info{
				Version:     "v1.0.0",
				BuildDate:   "2025-12-05",
				BuildNumber: "123",
			},
			wantVersion: "v1.0.0",
			wantDate:    "2025-12-05",
			wantNumber:  "123",
		},
		{
			name: "Full build info with ISO 8601 date",
			buildInfo: Info{
				Version:     "v2.3.4",
				BuildDate:   "2025-12-05T11:30:00Z",
				BuildNumber: "456",
			},
			wantVersion: "v2.3.4",
			wantDate:    "2025-12-05T11:30:00Z",
			wantNumber:  "456",
		},

		// =================================================================
		// Empty and Partial Info
		// =================================================================
		{
			name:        "Empty build info",
			buildInfo:   Info{},
			wantVersion: "",
			wantDate:    "",
			wantNumber:  "",
		},
		{
			name: "Partial build info - Version only",
			buildInfo: Info{
				Version: "dev",
			},
			wantVersion: "dev",
			wantDate:    "",
			wantNumber:  "",
		},
		{
			name: "Partial build info - BuildDate only",
			buildInfo: Info{
				BuildDate: "2025-12-05",
			},
			wantVersion: "",
			wantDate:    "2025-12-05",
			wantNumber:  "",
		},
		{
			name: "Partial build info - BuildNumber only",
			buildInfo: Info{
				BuildNumber: "789",
			},
			wantVersion: "",
			wantDate:    "",
			wantNumber:  "789",
		},

		// =================================================================
		// Various Version Formats
		// =================================================================
		{
			name: "Git Commit Hash",
			buildInfo: Info{
				Version:     "abc123def456",
				BuildDate:   "2025-12-05T11:30:00Z",
				BuildNumber: "456",
			},
			wantVersion: "abc123def456",
			wantDate:    "2025-12-05T11:30:00Z",
			wantNumber:  "456",
		},
		{
			name: "Short Git Hash",
			buildInfo: Info{
				Version:     "abc123d",
				BuildDate:   "2025-12-05",
				BuildNumber: "100",
			},
			wantVersion: "abc123d",
			wantDate:    "2025-12-05",
			wantNumber:  "100",
		},
		{
			name: "Semantic Versioning with pre-release",
			buildInfo: Info{
				Version:     "v1.0.0-alpha.1",
				BuildDate:   "2025-12-05",
				BuildNumber: "200",
			},
			wantVersion: "v1.0.0-alpha.1",
			wantDate:    "2025-12-05",
			wantNumber:  "200",
		},
		{
			name: "Semantic Versioning with build metadata",
			buildInfo: Info{
				Version:     "v1.0.0+20251205",
				BuildDate:   "2025-12-05",
				BuildNumber: "300",
			},
			wantVersion: "v1.0.0+20251205",
			wantDate:    "2025-12-05",
			wantNumber:  "300",
		},

		// =================================================================
		// Edge Cases
		// =================================================================
		{
			name: "Version with whitespace",
			buildInfo: Info{
				Version:     "  v1.0.0  ",
				BuildDate:   "2025-12-05",
				BuildNumber: "123",
			},
			wantVersion: "  v1.0.0  ",
			wantDate:    "2025-12-05",
			wantNumber:  "123",
		},
		{
			name: "BuildNumber with leading zeros",
			buildInfo: Info{
				Version:     "v1.0.0",
				BuildDate:   "2025-12-05",
				BuildNumber: "00123",
			},
			wantVersion: "v1.0.0",
			wantDate:    "2025-12-05",
			wantNumber:  "00123",
		},
		{
			name: "Very long version string",
			buildInfo: Info{
				Version:     "v1.0.0-alpha.1+build.20251205.abc123def456",
				BuildDate:   "2025-12-05T11:30:00Z",
				BuildNumber: "999999",
			},
			wantVersion: "v1.0.0-alpha.1+build.20251205.abc123def456",
			wantDate:    "2025-12-05T11:30:00Z",
			wantNumber:  "999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantVersion, tt.buildInfo.Version, "Version 필드가 일치해야 합니다")
			assert.Equal(t, tt.wantDate, tt.buildInfo.BuildDate, "BuildDate 필드가 일치해야 합니다")
			assert.Equal(t, tt.wantNumber, tt.buildInfo.BuildNumber, "BuildNumber 필드가 일치해야 합니다")
		})
	}
}

// TestSetGet은 전역 Info 설정 및 조회를 검증합니다.
func TestSetGet(t *testing.T) {
	expected := Info{
		Version:     "v1.2.3",
		BuildDate:   "2025-01-01",
		BuildNumber: "999",
	}

	Set(expected)
	got := Get()

	assert.Equal(t, expected.Version, got.Version, "Version은 일치해야 합니다")
	assert.Equal(t, expected.BuildDate, got.BuildDate, "BuildDate는 일치해야 합니다")
	assert.Equal(t, expected.BuildNumber, got.BuildNumber, "BuildNumber는 일치해야 합니다")

	// 런타임 정보 자동 주입 검증
	assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion이 자동으로 주입되어야 합니다")
	assert.Equal(t, runtime.GOOS, got.OS, "OS가 자동으로 주입되어야 합니다")
	assert.Equal(t, runtime.GOARCH, got.Arch, "Arch가 자동으로 주입되어야 합니다")
}

// TestInfo_String은 Stringer 인터페이스 구현을 검증합니다.
func TestInfo_String(t *testing.T) {
	tests := []struct {
		name      string
		buildInfo Info
		want      string
	}{
		{
			name: "Full info",
			buildInfo: Info{
				Version:     "v1.0.0",
				BuildDate:   "2025-01-01",
				BuildNumber: "100",
				GoVersion:   "go1.21.0",
			},
			want: "v1.0.0 (build: 100, date: 2025-01-01, go_version: go1.21.0, os: , arch: )",
		},
		{
			name:      "Empty info",
			buildInfo: Info{},
			want:      "unknown",
		},
		{
			name: "Partial info",
			buildInfo: Info{
				Version:   "dev",
				GoVersion: "go1.21.0",
			},
			want: "dev (build: , date: , go_version: go1.21.0, os: , arch: )",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.buildInfo.String())
		})
	}
}

// TestRuntimeInfoPopulation은 Set 호출 시 런타임 정보가 자동 주입되는지 검증합니다.
func TestRuntimeInfoPopulation(t *testing.T) {
	// given
	info := Info{
		Version: "v1.0.0",
	}

	// when
	Set(info)
	got := Get()

	// then
	assert.Equal(t, "v1.0.0", got.Version)
	assert.NotEmpty(t, got.GoVersion, "GoVersion이 자동으로 채워져야 합니다")
	assert.Equal(t, runtime.Version(), got.GoVersion)
	assert.NotEmpty(t, got.OS, "OS가 자동으로 채워져야 합니다")
	assert.Equal(t, runtime.GOOS, got.OS)
	assert.NotEmpty(t, got.Arch, "Arch가 자동으로 채워져야 합니다")
	assert.Equal(t, runtime.GOARCH, got.Arch)
}

// TestJSONMarshaling은 Info 구조체의 JSON 태그가 올바른지 검증합니다.
func TestJSONMarshaling(t *testing.T) {
	// given
	info := Info{
		Version:     "v1.0.0",
		BuildDate:   "2025-01-01",
		BuildNumber: "100",
		GoVersion:   "go1.21.0",
		OS:          "linux",
		Arch:        "amd64",
	}

	// when
	data, err := json.Marshal(info)
	assert.NoError(t, err)

	// then
	// JSON 키가 snake_case로 올바르게 생성되었는지 확인
	var resultMap map[string]interface{}
	err = json.Unmarshal(data, &resultMap)
	assert.NoError(t, err)

	assert.Equal(t, "v1.0.0", resultMap["version"])
	assert.Equal(t, "2025-01-01", resultMap["build_date"])
	assert.Equal(t, "100", resultMap["build_number"])
	assert.Equal(t, "go1.21.0", resultMap["go_version"])
	assert.Equal(t, "linux", resultMap["os"])
	assert.Equal(t, "amd64", resultMap["arch"])
}
