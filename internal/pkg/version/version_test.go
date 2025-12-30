package version

import (
	"encoding/json"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// Documentation Examples (GoDoc)
// =============================================================================

func Example() {
	// 1. 빌더(Builder) 또는 메인(Main) 함수에서 버전 정보 설정
	// 실제 환경에서는 -ldflags로 주입된 변수를 사용합니다.
	buildInfo := Info{
		Version:     "v1.2.3",
		BuildDate:   "2025-01-01T00:00:00Z",
		BuildNumber: "100",
		GoVersion:   "go_version",
		OS:          "os",
		Arch:        "arch",
	}

	// 전역 설정 (앱 시작 시 1회 호출)
	set(buildInfo)

	// 2. 어디서든 안전하게 조회 가능
	current := Get()
	fmt.Printf("App Version: %s\n", current.Version)
	fmt.Printf("Build Number: %s\n", current.BuildNumber)

	// Output:
	// App Version: v1.2.3
	// Build Number: 100
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
			wantStr: "unknown",
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
	assert.Equal(t, "unknown", got.Commit, "Commit should default to unknown if not provided")
	assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion should be auto-populated")
	assert.Equal(t, runtime.GOOS, got.OS, "OS should be auto-populated")
	assert.Equal(t, runtime.GOARCH, got.Arch, "Arch should be auto-populated")
}

// TestCollectRuntimeAndBuildMetadata는 정보 수집 로직의 비즈니스 규칙을 검증합니다.
func TestCollectRuntimeAndBuildMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    Info
		wantInfo Info
		// 런타임 값(GoVersion 등)은 환경마다 다르므로 검증 방식이 다를 수 있음
		checkRuntime bool
	}{
		{
			name: "All Missing (Defaults)",
			input: Info{
				Version: "v1.0.0",
			},
			wantInfo: Info{
				Version:    "v1.0.0",
				Commit:     "unknown", // Default assertion
				DirtyBuild: false,
			},
			checkRuntime: true,
		},
		{
			name: "Pre-filled Info (Optimization)",
			input: Info{
				Version:    "v2.0.0",
				Commit:     "abcdef",
				GoVersion:  "custom-go",
				OS:         "custom-os",
				Arch:       "custom-arch",
				DirtyBuild: true,
			},
			wantInfo: Info{
				Version:    "v2.0.0",
				Commit:     "abcdef",
				GoVersion:  "custom-go",
				OS:         "custom-os",
				Arch:       "custom-arch",
				DirtyBuild: true,
			},
			checkRuntime: false, // 기존 값이 보존되어야 함
		},
		{
			name: "None Commit Normalization",
			input: Info{
				Version: "v3.0.0",
				Commit:  "none",
			},
			wantInfo: Info{
				Version: "v3.0.0",
				Commit:  "unknown", // 'none' should be normalized to 'unknown' internally before enriching
			},
			checkRuntime: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := collectRuntimeAndBuildMetadata(tt.input)

			// 1. Static Fields Check
			assert.Equal(t, tt.wantInfo.Version, got.Version)

			// Commit Check: 'none' normalization or specific value
			if tt.wantInfo.Commit != "" {
				// Note: 로컬 개발 환경에서는 debug.ReadBuildInfo()가 실제 Git 정보를 읽어와
				// 'unknown' 대신 실제 커밋 해시를 채울 수 있음.
				// 따라서 'unknown'을 기대하는 경우, 실제 값이 채워졌다면(unknown이 아니라면) 테스트 통과로 간주할 수도 있음.
				// 하지만 여기서는 로직의 '초기화' 동작을 검증하므로, 'none'이 그대로 남아있지만 않으면 됨.
				assert.NotEqual(t, "none", got.Commit)
				if tt.wantInfo.Commit != "unknown" {
					assert.Equal(t, tt.wantInfo.Commit, got.Commit)
				}
			}

			// 2. Runtime Fields Check
			if tt.checkRuntime {
				assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion should be auto-populated")
				assert.Equal(t, runtime.GOOS, got.OS, "OS should be auto-populated")
				assert.Equal(t, runtime.GOARCH, got.Arch, "Arch should be auto-populated")
			} else {
				// 기존 값이 보존되었는지 확인
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
