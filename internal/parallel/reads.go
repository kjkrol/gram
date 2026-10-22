package parallel

// Forkable reads state on one goroutine at a time: Fork is one more, for another.
type Forkable[R any] interface {
	Fork() R
}

// Reads are a Forkable's reads for the workers of a pool, by worker: the first given, a Fork of
// it each other.
type Reads[R Forkable[R]] []R

// NewReads is first, worker 0's, and a Fork of it for every other worker of p.
func NewReads[R Forkable[R]](first R, p *Pool) Reads[R] {
	rs := make(Reads[R], p.Size())
	rs[0] = first
	for w := 1; w < len(rs); w++ {
		rs[w] = first.Fork()
	}
	return rs
}
