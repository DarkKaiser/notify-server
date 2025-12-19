package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// BuildInfo Tests
// =============================================================================

// TestBuildInfo는 BuildInfo 구조체의 필드 값이 올바르게 설정되는지 검증합니다.
//
// 검증 항목:
//   - 모든 필드가 설정된 경우
//   - 빈 BuildInfo 구조체
//   - 부분적으로 설정된 경우
//   - 다양한 버전 형식 (Semantic Versioning, Git Hash, dev)
//   - 다양한 날짜 형식 (ISO 8601, 간단한 날짜)
//   - 특수 문자 및 공백 처리
func TestBuildInfo(t *testing.T) {
	tests := []struct {
		name        string
		buildInfo   BuildInfo
		wantVersion string
		wantDate    string
		wantNumber  string
	}{
		// =================================================================
		// Complete BuildInfo
		// =================================================================
		{
			name: "Full build info",
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
				Version:     "v2.3.4",
				BuildDate:   "2025-12-05T11:30:00Z",
				BuildNumber: "456",
			},
			wantVersion: "v2.3.4",
			wantDate:    "2025-12-05T11:30:00Z",
			wantNumber:  "456",
		},

		// =================================================================
		// Empty and Partial BuildInfo
		// =================================================================
		{
			name:        "Empty build info",
			buildInfo:   BuildInfo{},
			wantVersion: "",
			wantDate:    "",
			wantNumber:  "",
		},
		{
			name: "Partial build info - Version only",
			buildInfo: BuildInfo{
				Version: "dev",
			},
			wantVersion: "dev",
			wantDate:    "",
			wantNumber:  "",
		},
		{
			name: "Partial build info - BuildDate only",
			buildInfo: BuildInfo{
				BuildDate: "2025-12-05",
			},
			wantVersion: "",
			wantDate:    "2025-12-05",
			wantNumber:  "",
		},
		{
			name: "Partial build info - BuildNumber only",
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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
			buildInfo: BuildInfo{
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

// TestBuildInfo_StructFields는 BuildInfo 구조체의 필드가 올바르게 정의되어 있는지 검증합니다.
func TestBuildInfo_StructFields(t *testing.T) {
	t.Run("모든 필드가 string 타입", func(t *testing.T) {
		bi := BuildInfo{
			Version:     "test",
			BuildDate:   "test",
			BuildNumber: "test",
		}

		// 필드 타입 검증 (컴파일 타임에 검증되지만 명시적으로 확인)
		assert.IsType(t, "", bi.Version, "Version은 string 타입이어야 합니다")
		assert.IsType(t, "", bi.BuildDate, "BuildDate는 string 타입이어야 합니다")
		assert.IsType(t, "", bi.BuildNumber, "BuildNumber는 string 타입이어야 합니다")
	})

	t.Run("Zero Value", func(t *testing.T) {
		var bi BuildInfo

		// Zero value는 모든 필드가 빈 문자열
		assert.Equal(t, "", bi.Version, "Zero value의 Version은 빈 문자열이어야 합니다")
		assert.Equal(t, "", bi.BuildDate, "Zero value의 BuildDate는 빈 문자열이어야 합니다")
		assert.Equal(t, "", bi.BuildNumber, "Zero value의 BuildNumber는 빈 문자열이어야 합니다")
	})
}
