package parallel

import (
	"sync/atomic"
	"testing"
)

func TestPool_Parts_NeverMoreThanTheItemsAllow(t *testing.T) {
	p := New(4)
	for _, c := range []struct{ n, least, limit, want int }{
		{0, 64, 4, 1}, {63, 64, 4, 1}, {128, 64, 4, 2}, {10000, 64, 0, 4}, {10000, 64, 1, 1},
		{10000, 64, 9, 4},
	} {
		if got := p.Parts(c.n, c.least, c.limit); got != c.want {
			t.Errorf("Parts(%d, %d, %d) = %d, want %d", c.n, c.least, c.limit, got, c.want)
		}
	}
	var none *Pool
	if got := none.Parts(10000, 1, 0); got != 1 || none.Size() != 1 {
		t.Errorf("a nil pool makes %d parts of %d goroutines, want one of one", got, none.Size())
	}
}

func TestPart_CoversTheItemsInOrder(t *testing.T) {
	next := 0
	for i := range 7 {
		from, to := Part(100, 7, i)
		if from != next || to < from {
			t.Fatalf("part %d is [%d, %d), want it from %d", i, from, to, next)
		}
		next = to
	}
	if next != 100 {
		t.Errorf("the parts end at %d, want 100", next)
	}
}

func TestBatch_EveryTaskOnceOnAWorkerOfThePool(t *testing.T) {
	for _, size := range []int{1, 2, 3, 8} {
		p := New(size)
		const n = 1000
		var seen [n]atomic.Int32
		var idle [8]atomic.Int32
		bad := atomic.Bool{}
		for range 20 {
			for i := range seen {
				seen[i].Store(0)
			}
			for w := range idle {
				idle[w].Store(0)
			}
			b := p.Begin(func(w, i int) {
				if w < 0 || w >= size {
					bad.Store(true)
				}
				seen[i].Add(1)
			}, func(w int) { idle[w].Add(1) })
			for added := 0; added < n; added += 100 {
				b.Add(100)
			}
			b.Wait()
			for i := range seen {
				if seen[i].Load() != 1 {
					t.Fatalf("size %d: task %d ran %d times, want once", size, i, seen[i].Load())
				}
			}
			for w := range idle {
				if idle[w].Load() > 1 || w >= size && idle[w].Load() != 0 {
					t.Fatalf("size %d: worker %d idled %d times", size, w, idle[w].Load())
				}
			}
		}
		if bad.Load() {
			t.Errorf("size %d: a task ran on a worker outside the pool", size)
		}
	}
}

func TestBatch_NoTaskStillEnds(t *testing.T) {
	p := New(4)
	p.Begin(func(int, int) { t.Error("a task ran in a batch of none") }, nil).Wait()
}

type counter struct{ n atomic.Int64 }

func (c *counter) task(_, i int) { c.n.Add(int64(i)) }

func TestBatch_AllocatesNothing(t *testing.T) {
	p := New(4)
	c := &counter{}
	task := c.task
	allocs := testing.AllocsPerRun(100, func() {
		b := p.Begin(task, nil)
		b.Add(64)
		b.Add(64)
		b.Wait()
	})
	if allocs != 0 {
		t.Errorf("a batch allocates %v times, want none", allocs)
	}
}
