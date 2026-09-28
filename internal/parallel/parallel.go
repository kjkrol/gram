// Package parallel shares a job of many alike items among a few goroutines: how many to use for
// the items in hand, and running them over contiguous ranges of the items.
package parallel

import (
	"runtime"
	"sync"
)

// Workers is how many goroutines share n items so that each has at least least of them: no more
// than limit — 0 for as many as there are CPUs — and never fewer than 1.
func Workers(n, least, limit int) int {
	if limit <= 0 {
		limit = runtime.GOMAXPROCS(0)
	}
	if least < 1 {
		least = 1
	}
	return max(1, min(limit, n/least))
}

// Run has k goroutines, the caller's among them, work on n items in contiguous ranges — fn(w, from,
// to) on the w-th — and returns when all are done. With k of 1 or less fn runs once, here.
func Run(k, n int, fn func(w, from, to int)) {
	if k <= 1 {
		fn(0, 0, n)
		return
	}
	var wg sync.WaitGroup
	for w := 1; w < k; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn(w, n*w/k, n*(w+1)/k)
		}()
	}
	fn(0, 0, n/k)
	wg.Wait()
}
