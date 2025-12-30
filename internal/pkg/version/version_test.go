package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Documentation Examples (GoDoc)
// =============================================================================

func Example() {
	// 1. 빌드 정보 조회
	// 실제 환경에서 버전 정보는 링커 플래그(-ldflags)를 통해 주입됩니다.
	// 따라서 별도의 설정 없이 Get() 함수를 호출하여 안전하게 정보를 조회할 수 있습니다.
	current := Get()

	// 예시 출력을 위한 가상 데이터 설정 (실제 코드에서는 불필요)
	// 이 부분은 문서화된 예제 실행을 위해 임의로 값을 보여주는 것입니다.
	fmt.Printf("App Version: %s\n", current.Version)

	// Output:
	// App Version: unknown
}

// =============================================================================
// Unit Tests
// =============================================================================

// TestInfo_FieldValidation은 Info 구조체 필드 검증을 수행합니다.
func TestInfo_FieldValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     Info
		wantStr   string
		expectNil bool
	}{
		{
			name: "Complete Info",
			input: Info{
				Version:     "v1.0.0",
				Commit:      "1234567890abcdef",
				BuildDate:   "2025-01-01",
				BuildNumber: "1",
				GoVersion:   "go1.21",
				OS:          "linux",
				Arch:        "amd64",
			},
			wantStr: "v1.0.0 (commit: 1234567, build: 1, date: 2025-01-01, go_version: go1.21, os: linux, arch: amd64)",
		},
		{
			name: "Dirty Info",
			input: Info{
				Version:    "v1.0.0",
				DirtyBuild: true,
				GoVersion:  "go1.21",
				OS:         "linux",
				Arch:       "amd64",
			},
			// Commit이 없으면 unknown으로 표시
			wantStr: "v1.0.0-dirty (commit: unknown, build: , date: , go_version: go1.21, os: linux, arch: amd64)",
		},
		{
			name:    "Empty Info",
			input:   Info{},
			wantStr: unknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantStr, tt.input.String())
		})
	}
}

// TestSetGet_RuntimeInfo는 Set 호출 시 런타임 정보가 자동 주입되는지 검증합니다.
func TestSetGet_RuntimeInfo(t *testing.T) {
	// Set은 전역 상태를 변경하므로 Parallel 불가
	// Cleanup을 통해 테스트 종료 후 상태 복구 보장
	original := Get()
	t.Cleanup(func() { set(original) })

	// Reset global state for this test
	globalBuildInfo.Store(Info{})

	input := Info{Version: "v1.0.0"}
	set(input)

	got := Get()
	assert.Equal(t, "v1.0.0", got.Version)
	assert.Equal(t, "v1.0.0", got.Version)
	assert.Equal(t, unknown, got.Commit, "Commit should default to unknown if not provided")
	assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion should be auto-populated")
	assert.Equal(t, runtime.GOOS, got.OS, "OS should be auto-populated")
	assert.Equal(t, runtime.GOARCH, got.Arch, "Arch should be auto-populated")
}

// TestGitTreeState_Integration은 ldflags로 주입된 gitTreeState 변수가
// DirtyBuild 필드에 올바르게 반영되는지 검증합니다.
func TestGitTreeState_Integration(t *testing.T) {
	// 전역 변수 gitTreeState를 조작해야 하므로 병렬 실행 불가
	// 테스트 종료 후 원복 보장
	originalState := gitTreeState
	originalInfo := Get()
	t.Cleanup(func() {
		gitTreeState = originalState
		set(originalInfo)
	})

	tests := []struct {
		name      string
		treeState string // ldflags 주입 시뮬레이션
		wantDirty bool
	}{
		{
			name:      "Clean Build",
			treeState: "clean",
			wantDirty: false,
		},
		{
			name:      "Dirty Build",
			treeState: "dirty",
			wantDirty: true,
		},
		{
			name:      "Empty State (Default)",
			treeState: "",
			wantDirty: false,
		},
		{
			name:      "Unknown State",
			treeState: "unknown",
			wantDirty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 1. ldflags 주입 시뮬레이션
			gitTreeState = tt.treeState

			// 2. set 호출 (내부적으로 gitTreeState 확인)
			set(Info{Version: "v1.0.0"})

			// 3. 결과 검증
			got := Get()
			assert.Equal(t, tt.wantDirty, got.DirtyBuild, "gitTreeState=%q should result in DirtyBuild=%v", tt.treeState, tt.wantDirty)
		})
	}
}

// TestCollectRuntimeAndBuildMetadata는 정보 수집 로직의 비즈니스 규칙을 검증합니다.
func TestCollectRuntimeAndBuildMetadata(t *testing.T) {
	// Global state modification requires sequential execution.
	// readBuildInfo is a package-level variable, so changing it is not thread-safe.

	tests := []struct {
		name          string
		input         Info
		mockBuildInfo func() (*debug.BuildInfo, bool)
		wantInfo      Info
		checkRuntime  bool
	}{
		{
			name:  "Scenario: All Missing (Defaults) - No Build Info",
			input: Info{Version: "v1.0.0"},
			mockBuildInfo: func() (*debug.BuildInfo, bool) {
				return nil, false
			},
			wantInfo: Info{
				Version:    "v1.0.0",
				Commit:     unknown,
				DirtyBuild: false,
			},
			checkRuntime: true,
		},
		{
			name:  "Scenario: Version Fallback to Unknown",
			input: Info{Version: ""},
			mockBuildInfo: func() (*debug.BuildInfo, bool) {
				return nil, false
			},
			wantInfo: Info{
				Version:    unknown,
				Commit:     unknown,
				DirtyBuild: false,
			},
			checkRuntime: true,
		},
		{
			name: "Scenario: Pre-filled Info (Optimization)",
			input: Info{
				Version:    "v2.0.0",
				Commit:     "abcdef",
				GoVersion:  "custom-go",
				OS:         "custom-os",
				Arch:       "custom-arch",
				DirtyBuild: true,
			},
			mockBuildInfo: func() (*debug.BuildInfo, bool) {
				// Should not be relevant as optimization skips it
				return nil, false
			},
			wantInfo: Info{
				Version:    "v2.0.0",
				Commit:     "abcdef",
				GoVersion:  "custom-go",
				OS:         "custom-os",
				Arch:       "custom-arch",
				DirtyBuild: true,
			},
			checkRuntime: false,
		},
		{
			name:  "Scenario: 'none' Commit Normalization",
			input: Info{Version: "v3.0.0", Commit: none},
			mockBuildInfo: func() (*debug.BuildInfo, bool) {
				return nil, false
			},
			wantInfo: Info{
				Version: "v3.0.0",
				Commit:  unknown,
			},
			checkRuntime: true,
		},
		{
			name:  "Scenario: VCS Enrichment success",
			input: Info{Version: "v4.0.0"}, // Commit missing
			mockBuildInfo: func() (*debug.BuildInfo, bool) {
				return &debug.BuildInfo{
					Settings: []debug.BuildSetting{
						{Key: "vcs.revision", Value: "git-hash-123"},
						{Key: "vcs.time", Value: "2025-05-05"},
						{Key: "vcs.modified", Value: "true"},
					},
				}, true
			},
			wantInfo: Info{
				Version:    "v4.0.0",
				Commit:     "git-hash-123",
				BuildDate:  "2025-05-05",
				DirtyBuild: true,
			},
			checkRuntime: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			mockReadBuildInfo(t, tt.mockBuildInfo)

			got := collectRuntimeAndBuildMetadata(tt.input)

			// 1. Static Fields Check
			assert.Equal(t, tt.wantInfo.Version, got.Version)
			assert.Equal(t, tt.wantInfo.Commit, got.Commit)
			assert.Equal(t, tt.wantInfo.BuildDate, got.BuildDate)
			assert.Equal(t, tt.wantInfo.DirtyBuild, got.DirtyBuild)

			// 2. Runtime Fields Check
			if tt.checkRuntime {
				assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion should be auto-populated")
				assert.Equal(t, runtime.GOOS, got.OS, "OS should be auto-populated")
				assert.Equal(t, runtime.GOARCH, got.Arch, "Arch should be auto-populated")
			} else {
				if tt.wantInfo.GoVersion != "" {
					assert.Equal(t, tt.wantInfo.GoVersion, got.GoVersion)
				}
				if tt.wantInfo.OS != "" {
					assert.Equal(t, tt.wantInfo.OS, got.OS)
				}
				if tt.wantInfo.Arch != "" {
					assert.Equal(t, tt.wantInfo.Arch, got.Arch)
				}
			}
		})
	}
}

// mockReadBuildInfo safely replaces readBuildInfo for testing and ensures cleanup.
func mockReadBuildInfo(t *testing.T, impl func() (*debug.BuildInfo, bool)) {
	t.Helper()
	// Capture the current value
	original := readBuildInfo
	// Restore it after the test
	t.Cleanup(func() { readBuildInfo = original })
	// Set the mock implementation
	readBuildInfo = impl
}

// TestHelpers는 패키지 레벨 헬퍼 함수들을 검증합니다.
func TestHelpers(t *testing.T) {
	// Global state modification - Restore after test
	original := Get()
	t.Cleanup(func() { set(original) })

	set(Info{
		Version: "v1.5.0",
		Commit:  "deadbeef",
	})

	assert.Equal(t, "v1.5.0", Version())
	assert.Equal(t, "deadbeef", Commit())
}

// TestJSONMarshaling은 JSON 직렬화/역직렬화 호환성을 검증합니다.
func TestJSONMarshaling(t *testing.T) {
	t.Parallel()
	info := Info{
		Version:     "v1.0.0",
		Commit:      "hash123",
		BuildNumber: "123",
		DirtyBuild:  true,
	}

	data, err := json.Marshal(info)
	assert.NoError(t, err)

	var decoded map[string]any
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, "v1.0.0", decoded["version"])
	assert.Equal(t, "hash123", decoded["commit"])
	assert.Equal(t, "123", decoded["build_number"])
	assert.Equal(t, true, decoded["dirty_build"])
}

// TestToMap은 구조적 로깅을 위한 맵 변환을 검증합니다.
func TestToMap(t *testing.T) {
	t.Parallel()

	info := Info{
		Version:     "v1.2.3",
		Commit:      "abcdef",
		BuildDate:   "2025-01-01",
		BuildNumber: "999",
		GoVersion:   "go1.21",
		OS:          "linux",
		Arch:        "amd64",
		DirtyBuild:  true,
	}

	m := info.ToMap()

	assert.Equal(t, "v1.2.3", m["version"])
	assert.Equal(t, "abcdef", m["commit"])
	assert.Equal(t, "2025-01-01", m["build_date"])
	assert.Equal(t, "999", m["build_number"])
	assert.Equal(t, "go1.21", m["go_version"])
	assert.Equal(t, "linux", m["os"])
	assert.Equal(t, "amd64", m["arch"])
	assert.Equal(t, "true", m["dirty_build"])
}

// =============================================================================
// Concurrency Safety Tests
// =============================================================================

// TestConcurrentAccess는 다수의 고루틴이 동시에 Get()을 호출해도 안전한지(Race Free) 검증합니다.
// go test -race 플래그와 함께 실행되어야 효과적입니다.
func TestConcurrentAccess(t *testing.T) {
	const (
		numReaders = 100
		numWriters = 10
		iterations = 1000
	)

	var wg sync.WaitGroup
	wg.Add(numReaders + numWriters)

	// 테스트 종료 후 상태 복구
	original := Get()
	t.Cleanup(func() { set(original) })

	// 초기값 설정
	set(Info{Version: "initial"})

	// Writers: 간헐적으로 버전을 업데이트
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				set(Info{
					Version:     fmt.Sprintf("v1.%d.%d", id, j),
					Commit:      fmt.Sprintf("commit-%d-%d", id, j),
					BuildNumber: fmt.Sprintf("%d", j),
				})
				// Write 빈도를 줄여 Read 위주 부하 생성
				runtime.Gosched()
			}
		}(i)
	}

	// Readers: 지속적으로 버전을 조회
	for i := 0; i < numReaders; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				info := Get()
				// 읽어온 데이터 무결성 체크 (Panic이나 nil dereference가 없어야 함)
				_ = info.String()
				assert.NotNil(t, info.Version) // Zero value일 수는 있어도 필드 접근 시 안전해야 함
			}
		}()
	}

	wg.Wait()
}

// =============================================================================
// Benchmarks
// =============================================================================

// BenchmarkGet은 전역 버전 정보 조회 성능을 측정합니다.
// atomic.Value.Load()의 성능 특성을 확인합니다.
func BenchmarkGet(b *testing.B) {
	// 벤치마크 종료 후 상태 복구
	original := Get()
	b.Cleanup(func() { set(original) })

	set(Info{
		Version:     "v1.0.0",
		Commit:      "benchmark-commit",
		BuildDate:   "2025-01-01",
		BuildNumber: "12345",
	})
	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Get()
		}
	})
}
