package fetcher

import (
	"bytes"
	"io"
	"testing"
)

// BenchmarkDrainAndCloseBody 최적화된 drainAndCloseBody의 성능 측정
func BenchmarkDrainAndCloseBody(b *testing.B) {
	// 64KB (MaxDrainBytes) 크기의 데이터 생성
	data := make([]byte, maxDrainBytes)
	for i := range data {
		data[i] = 'a'
	}

	b.Run("OptimizedWithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			body := io.NopCloser(bytes.NewReader(data))
			drainAndCloseBody(body)
		}
	})
}
