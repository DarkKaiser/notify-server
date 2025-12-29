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
	// 1. 빌드라(Builder) 또는 메인(Main) 함수에서 버전 정보 설정
	// 실제 환경에서는 -ldflags로 주입된 변수를 사용합니다.
	buildInfo := Info{
		Version:     "v1.2.3",
		BuildDate:   "2025-01-01T00:00:00Z",
		BuildNumber: "100",
	}

	// 전역 설정 (앱 시작 시 1회 호출)
	Set(buildInfo)

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
				BuildDate:   "2025-01-01",
				BuildNumber: "1",
				GoVersion:   "go1.21",
				OS:          "linux",
				Arch:        "amd64",
			},
			wantStr: "v1.0.0 (build: 1, date: 2025-01-01, go_version: go1.21, os: linux, arch: amd64)",
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
	// Reset global state for this test
	globalInfo.Store(Info{})

	input := Info{Version: "v1.0.0"}
	Set(input)

	got := Get()
	assert.Equal(t, "v1.0.0", got.Version)
	assert.Equal(t, runtime.Version(), got.GoVersion, "GoVersion should be auto-populated")
	assert.Equal(t, runtime.GOOS, got.OS, "OS should be auto-populated")
	assert.Equal(t, runtime.GOARCH, got.Arch, "Arch should be auto-populated")
}

// TestJSONMarshaling은 JSON 직렬화/역직렬화 호환성을 검증합니다.
func TestJSONMarshaling(t *testing.T) {
	t.Parallel()
	info := Info{
		Version:     "v1.0.0",
		BuildNumber: "123",
	}

	data, err := json.Marshal(info)
	assert.NoError(t, err)

	var decoded map[string]any
	err = json.Unmarshal(data, &decoded)
	assert.NoError(t, err)

	assert.Equal(t, "v1.0.0", decoded["version"])
	assert.Equal(t, "123", decoded["build_number"])
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

	// 초기값 설정
	Set(Info{Version: "initial"})

	// Writers: 간헐적으로 버전을 업데이트
	for i := 0; i < numWriters; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				Set(Info{
					Version:     fmt.Sprintf("v1.%d.%d", id, j),
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
	Set(Info{
		Version:     "v1.0.0",
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
