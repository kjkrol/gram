// Package parallel shares a job of many tasks among the goroutines of a pool kept for the whole
// game: the caller's own and the pool's, each known by its number.
package parallel

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// Pool is Size goroutines sharing the tasks of one Batch at a time: worker 0, the caller of Wait,
// and Size−1 of the pool's own, waiting between batches. Size and Parts take a nil Pool for one
// goroutine.
type Pool struct {
	size  int
	mu    sync.Mutex
	more  sync.Cond // tasks added, or the batch closed
	left  sync.Cond // the pool's last worker left the batch
	begun uint64
	in    int // the pool's workers not yet out of the batch
	batch Batch
}

// Batch is the tasks of one Begin; Add and Wait are its caller's.
type Batch struct {
	pool   *Pool
	task   func(w, i int)
	idle   func(w int)
	added  atomic.Int64
	next   atomic.Int64
	closed bool
}

// New is a pool of size goroutines, the caller's among them; 0 or less as many as there are CPUs.
func New(size int) *Pool {
	if size <= 0 {
		size = runtime.GOMAXPROCS(0)
	}
	p := &Pool{size: size}
	p.more.L, p.left.L = &p.mu, &p.mu
	p.batch.pool = p
	for w := 1; w < size; w++ {
		go p.work(w)
	}
	return p
}

// Size is how many goroutines share a batch, the caller's among them.
func (p *Pool) Size() int {
	if p == nil {
		return 1
	}
	return p.size
}

// Parts is how many tasks n items make, each of at least least of them: no more than limit — 0
// for the pool's Size — nor the Size, and never fewer than 1.
func (p *Pool) Parts(n, least, limit int) int {
	if limit <= 0 || limit > p.Size() {
		limit = p.Size()
	}
	return max(1, min(limit, n/max(least, 1)))
}

// Part is the items [from, to) of the i-th of k tasks sharing n items in order.
func Part(n, k, i int) (from, to int) { return n * i / k, n * (i + 1) / k }

// Begin starts a batch: task(w, i) runs on worker w for every task i added; idle(w), if not nil,
// once on each worker the first time it finds no task left. Functions made once allocate nothing.
func (p *Pool) Begin(task func(w, i int), idle func(w int)) *Batch {
	b := &p.batch
	p.mu.Lock()
	b.task, b.idle, b.closed = task, idle, false
	b.added.Store(0)
	b.next.Store(0)
	p.in = p.size - 1
	p.begun++
	p.mu.Unlock()
	return b
}

// Add makes n more tasks, numbered on from the last; the pool's workers take them at once.
func (b *Batch) Add(n int) {
	p := b.pool
	p.mu.Lock()
	b.added.Add(int64(n))
	p.mu.Unlock()
	p.more.Broadcast()
}

// Wait closes the batch to tasks and works on it as worker 0 till every task is done.
func (b *Batch) Wait() {
	p := b.pool
	p.mu.Lock()
	b.closed = true
	p.mu.Unlock()
	p.more.Broadcast()
	b.run(0)
	p.mu.Lock()
	for p.in > 0 {
		p.left.Wait()
	}
	p.mu.Unlock()
}

func (p *Pool) work(w int) {
	var seen uint64
	p.mu.Lock()
	for {
		for p.begun == seen {
			p.more.Wait()
		}
		seen = p.begun
		p.mu.Unlock()
		p.batch.run(w)
		p.mu.Lock()
		if p.in--; p.in == 0 {
			p.left.Signal()
		}
	}
}

// run takes tasks on w till the batch is closed and none is left
func (b *Batch) run(w int) {
	p, idled := b.pool, false
	for {
		if i, ok := b.take(); ok {
			b.task(w, i)
			continue
		}
		if !idled {
			if idled = true; b.idle != nil {
				b.idle(w)
			}
			continue
		}
		p.mu.Lock()
		for b.next.Load() >= b.added.Load() && !b.closed {
			p.more.Wait()
		}
		done := b.next.Load() >= b.added.Load()
		p.mu.Unlock()
		if done {
			return
		}
	}
}

func (b *Batch) take() (int, bool) {
	for {
		i := b.next.Load()
		if i >= b.added.Load() {
			return 0, false
		}
		if b.next.CompareAndSwap(i, i+1) {
			return int(i), true
		}
	}
}
