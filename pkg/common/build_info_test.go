package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildInfo(t *testing.T) {
	tests := []struct {
		name        string
		buildInfo   BuildInfo
		wantVersion string
		wantDate    string
		wantNumber  string
	}{
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
			name:        "Empty build info",
			buildInfo:   BuildInfo{},
			wantVersion: "",
			wantDate:    "",
			wantNumber:  "",
		},
		{
			name: "Partial build info",
			buildInfo: BuildInfo{
				Version: "dev",
			},
			wantVersion: "dev",
			wantDate:    "",
			wantNumber:  "",
		},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantVersion, tt.buildInfo.Version)
			assert.Equal(t, tt.wantDate, tt.buildInfo.BuildDate)
			assert.Equal(t, tt.wantNumber, tt.buildInfo.BuildNumber)
		})
	}
}
