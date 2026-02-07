package fetcher

import (
	"bytes"
	"io"
	"testing"
	"time"
)

// BenchmarkDrainAndCloseBody 다양한 크기의 페이로드에 대한 drainAndCloseBody 성능 측정
func BenchmarkDrainAndCloseBody(b *testing.B) {
	sizes := []struct {
		name string
		size int
	}{
		{"Small_1KB", 1024},
		{"Medium_32KB", 32 * 1024},
		{"Large_64KB", 64 * 1024},       // maxDrainBytes와 동일
		{"ExtraLarge_1MB", 1024 * 1024}, // maxDrainBytes 초과
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			data := make([]byte, size.size)
			// 데이터 초기화는 벤치마크 시간에서 제외
			b.ResetTimer()

			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				// 매 반복마다 Reader 생성 (비용이 적으므로 포함)
				body := io.NopCloser(bytes.NewReader(data))
				drainAndCloseBody(body)
			}
		})
	}
}

// BenchmarkTransportCreation Transport 생성 및 복제 비용 측정
func BenchmarkTransportCreation(b *testing.B) {
	key := transportConfig{
		proxyURL:            nil,
		maxIdleConns:        intPtr(100),
		idleConnTimeout:     durationPtr(90 * time.Second),
		tlsHandshakeTimeout: durationPtr(10 * time.Second),
	}

	b.Run("NewTransport", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			// 매번 새로운 Transport 생성 (기본값 복제 + 설정 적용)
			_, _ = newTransport(nil, key)
		}
	})

	b.Run("CloneTransport", func(b *testing.B) {
		base, _ := newTransport(nil, key)
		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_ = base.Clone()
		}
	})
}

// BenchmarkTransportCache Transport 캐시 조회 및 동시성 성능 측정
func BenchmarkTransportCache(b *testing.B) {
	// 테스트용 키 (transportConfig)
	cfg := transportConfig{
		proxyURL:            stringPtr("http://proxy.example.com:8080"),
		maxIdleConns:        intPtr(100),
		idleConnTimeout:     durationPtr(90 * time.Second),
		tlsHandshakeTimeout: durationPtr(10 * time.Second),
	}

	// 캐시에 미리 항목 추가
	_, _ = getSharedTransport(cfg)

	b.Run("Sequential_Hit", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = getSharedTransport(cfg)
		}
	})

	b.Run("Concurrent_Hit", func(b *testing.B) {
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				_, _ = getSharedTransport(cfg)
			}
		})
	})

	// 다양한 키를 사용하여 캐시 경합 및 LRU 갱신 테스트
	b.Run("Concurrent_MixedKeys", func(b *testing.B) {
		configs := []transportConfig{
			cfg,
			{maxIdleConns: intPtr(50)},
			{idleConnTimeout: durationPtr(60 * time.Second)},
			{tlsHandshakeTimeout: durationPtr(5 * time.Second)},
		}

		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				_, _ = getSharedTransport(configs[i%len(configs)])
				i++
			}
		})
	})
}
