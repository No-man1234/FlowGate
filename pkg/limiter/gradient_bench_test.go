package limiter

import (
	"context"
	"testing"
	"time"
)

func BenchmarkGradientLimiter_AcquireDone(b *testing.B) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 50000
	cfg.MaxLimit = 100000
	l := NewGradientLimiter(cfg)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tok, ok := l.Acquire(ctx, PriorityStandard)
			if ok {
				tok.Done(15*time.Millisecond, false)
			}
		}
	})
}

func BenchmarkGradientLimiter_ShedPath(b *testing.B) {
	cfg := DefaultConfig()
	cfg.InitialLimit = 1
	cfg.MinLimit = 1
	l := NewGradientLimiter(cfg)
	ctx := context.Background()

	// Saturate the limiter
	tok, _ := l.Acquire(ctx, PriorityCritical)
	defer tok.Release()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, ok := l.Acquire(ctx, PriorityStandard)
			if ok {
				b.Fatalf("expected acquire to be shed")
			}
		}
	})
}
