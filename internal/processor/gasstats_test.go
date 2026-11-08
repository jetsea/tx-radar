package processor

import (
	"sync"
	"testing"
)

func TestNewGasStats_Empty(t *testing.T) {
	g := NewGasStats(100)
	if g.Count() != 0 {
		t.Fatalf("expected 0 samples, got %d", g.Count())
	}
	if v := g.P95(); v != 0 {
		t.Fatalf("expected P95 of empty stats to be 0, got %d", v)
	}
}

func TestGasStats_AddAndCount(t *testing.T) {
	g := NewGasStats(100)
	for _, p := range []uint64{10, 20, 30, 40} {
		g.Add(p)
	}
	if g.Count() != 4 {
		t.Fatalf("expected 4 samples, got %d", g.Count())
	}
}

func TestGasStats_SlidingWindow(t *testing.T) {
	g := NewGasStats(3)
	for i := 0; i < 10; i++ {
		g.Add(uint64(i))
	}
	// maxSize=3, so last 3 samples (7,8,9) survive
	if g.Count() != 3 {
		t.Fatalf("expected 3 samples after windowing, got %d", g.Count())
	}
	if v := g.P95(); v != 9 {
		t.Fatalf("expected P95 to be 9, got %d", v)
	}
}

func TestGasStats_P95SingleSample(t *testing.T) {
	g := NewGasStats(10)
	g.Add(42)
	if v := g.P95(); v != 42 {
		t.Fatalf("expected P95 to be 42, got %d", v)
	}
}

func TestGasStats_P95Basic(t *testing.T) {
	g := NewGasStats(100)
	// 20 samples; idx = int(20*0.95) = 19
	for i := 0; i < 20; i++ {
		g.Add(uint64(i)) // 0..19
	}
	if v := g.P95(); v != 19 {
		t.Fatalf("expected P95 to be 19, got %d", v)
	}
}

func TestGasStats_P95DoesNotReorderInput(t *testing.T) {
	g := NewGasStats(100)
	for i := 0; i < 10; i++ {
		g.Add(uint64(i)) // arrives in ascending order
	}
	// P95 should not mutate internal state: adding another sample still works
	g.Add(50)
	if v := g.P95(); v != 50 { // 11 samples, idx = 10 => max = 50
		t.Fatalf("expected P95 to be 50, got %d", v)
	}
}

func TestGasStats_ConcurrentAccess(t *testing.T) {
	g := NewGasStats(1000)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				g.Add(uint64(j))
			}
			// read to simulate concurrent access
			_ = g.P95()
			_ = g.Count()
		}()
	}
	wg.Wait()

	// 50 goroutines * 20 adds = 1000, but window is 1000 so all remain
	if g.Count() != 1000 {
		t.Fatalf("expected 1000 samples, got %d", g.Count())
	}
}
