package parallel

import (
	"sync/atomic"
	"testing"
)

func TestWorkers_NeverMoreThanTheItemsAllow(t *testing.T) {
	for _, c := range []struct{ n, least, limit, want int }{
		{0, 64, 4, 1}, {63, 64, 4, 1}, {128, 64, 4, 2}, {10000, 64, 4, 4}, {10000, 64, 1, 1},
	} {
		if got := Workers(c.n, c.least, c.limit); got != c.want {
			t.Errorf("Workers(%d, %d, %d) = %d, want %d", c.n, c.least, c.limit, got, c.want)
		}
	}
	if got := Workers(1<<20, 1, 0); got < 1 {
		t.Errorf("Workers with no limit = %d, want at least one", got)
	}
}

func TestRun_CoversEveryItemOnce(t *testing.T) {
	for _, k := range []int{1, 2, 3, 7} {
		const n = 100
		var seen [n]atomic.Int32
		var workers atomic.Int32
		Run(k, n, func(w, from, to int) {
			workers.Add(1)
			for i := from; i < to; i++ {
				seen[i].Add(1)
			}
		})
		for i := range seen {
			if seen[i].Load() != 1 {
				t.Fatalf("k %d: item %d worked on %d times, want once", k, i, seen[i].Load())
			}
		}
		if int(workers.Load()) != k {
			t.Errorf("k %d: %d workers ran, want %d", k, workers.Load(), k)
		}
	}
}
