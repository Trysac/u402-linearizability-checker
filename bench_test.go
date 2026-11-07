package linearizabilitychecker_test

import (
	"context"
	"testing"

	"github.com/Trysac/u402-linearizability-checker"
)

func BenchmarkRegisterSequential(b *testing.B) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	history := make([]linearizabilitychecker.Op, 20)
	for i := 0; i < 20; i++ {
		if i%2 == 0 {
			history[i] = linearizabilitychecker.Op{
				ID:         i,
				Process:    0,
				Kind:       linearizabilitychecker.KindOk,
				Input:      i,
				CallTime:   int64(i * 2),
				ReturnTime: int64(i*2 + 1),
			}
		} else {
			history[i] = linearizabilitychecker.Op{
				ID:         i,
				Process:    0,
				Kind:       linearizabilitychecker.KindOk,
				Output:     i - 1,
				CallTime:   int64(i * 2),
				ReturnTime: int64(i*2 + 1),
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	}
}

func BenchmarkRegisterConcurrent(b *testing.B) {
	ctx := context.Background()
	model := linearizabilitychecker.RegisterModel()
	history := make([]linearizabilitychecker.Op, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			history[i] = linearizabilitychecker.Op{
				ID:         i,
				Process:    i,
				Kind:       linearizabilitychecker.KindOk,
				Input:      i,
				CallTime:   0,
				ReturnTime: 10,
			}
		} else {
			history[i] = linearizabilitychecker.Op{
				ID:         i,
				Process:    i,
				Kind:       linearizabilitychecker.KindOk,
				Output:     i - 1,
				CallTime:   0,
				ReturnTime: 10,
			}
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = linearizabilitychecker.Check(ctx, model, history, linearizabilitychecker.Options{})
	}
}
