package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInfo_Structure(t *testing.T) {
	buildInfo := BuildInfo{
		Version:     "v1.0.0",
		BuildDate:   "2025-12-05",
		BuildNumber: "123",
	}

	assert.Equal(t, "v1.0.0", buildInfo.Version)
	assert.Equal(t, "2025-12-05", buildInfo.BuildDate)
	assert.Equal(t, "123", buildInfo.BuildNumber)
}

func TestBuildInfo_EmptyValues(t *testing.T) {
	buildInfo := BuildInfo{}

	assert.Equal(t, "", buildInfo.Version)
	assert.Equal(t, "", buildInfo.BuildDate)
	assert.Equal(t, "", buildInfo.BuildNumber)
}

func TestBuildInfo_PartialValues(t *testing.T) {
	buildInfo := BuildInfo{
		Version: "dev",
	}

	assert.Equal(t, "dev", buildInfo.Version)
	assert.Equal(t, "", buildInfo.BuildDate)
	assert.Equal(t, "", buildInfo.BuildNumber)
}

func TestBuildInfo_GitCommitHash(t *testing.T) {
	// Git 커밋 해시 형식 테스트
	buildInfo := BuildInfo{
		Version:     "abc123def456",
		BuildDate:   "2025-12-05T11:30:00Z",
		BuildNumber: "456",
	}

	assert.Equal(t, "abc123def456", buildInfo.Version)
	assert.Equal(t, "2025-12-05T11:30:00Z", buildInfo.BuildDate)
	assert.Equal(t, "456", buildInfo.BuildNumber)
}
