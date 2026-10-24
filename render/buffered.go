package render

// Buffered is what a source draws, kept twice: Back is written by its publisher while Front is
// drawn, and Swap, while nothing draws, trades them.
type Buffered[T any] struct {
	bufs  [2]T
	front int
}

// Back is what is drawn next, being written.
func (b *Buffered[T]) Back() *T { return &b.bufs[1-b.front] }

// Front is what is drawn.
func (b *Buffered[T]) Front() *T { return &b.bufs[b.front] }

// Swap has the back drawn and the front written next.
func (b *Buffered[T]) Swap() { b.front = 1 - b.front }
