package topography

import (
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/render"
)

var _ board.Parallel = (*dresser)(nil)

// Warm works out, on the frame's goroutine, what dressing t reads that tiles share and the dresser
// works out as it is asked for: the shores at its corners, met by the tiles round it too — the
// board.Parallel contract. The rest a Worker works out itself: a tile's light, its shadows and its
// bake are the cell's own, its clouds the worker's.
func (l *dresser) Warm(t *board.Tile) {
	d := l.tileOf(t)
	if l.square && d.baseTop().shine > 0 && d.px() >= shoreCell {
		l.shoreOf(t.X0, t.Y0, t.X1, t.Y1)
	}
}

// reachAround is how many cells beyond a shadow's reach a worker may read the top of: the cells a
// way runs on to under water and the grounds round those.
const reachAround = 4

// Ready readies the workers, on the frame's goroutine: the highest top a shadow may come from, and
// the top of every cell a worker may read — under the camera's bounds and as far round them as a
// shadow, a shore or a way reaches — read now, so the workers read them as they stand and write
// nothing (frozen) — the board.Parallel contract.
func (l *dresser) Ready() {
	if l.stale {
		l.measureHighest(l.camera)
		l.stale = false
	}
	b := l.camera.Bounds()
	reach := float64(shadowReach+reachAround) * max(l.cellW, l.cellH)
	b.TopLeft.X, b.TopLeft.Y = b.TopLeft.X-reach, b.TopLeft.Y-reach
	b.BottomRight.X, b.BottomRight.Y = b.BottomRight.X+reach, b.BottomRight.Y+reach
	l.board.CellsUnder(b, func(c board.CellID) { l.topOf(c) })
}

// Worker is the dresser's k-th worker: a dresser of its own scratch — and its own clouds, worked
// out anew over its tiles — reading the tops as Ready read them and writing, of what is shared,
// only its own cells' light, shadows and bakes — the board.Parallel contract.
func (l *dresser) Worker(k int) board.Dressing {
	for len(l.workers) <= k {
		l.workers = append(l.workers, &dresser{})
	}
	w := l.workers[k]
	pieces, ways, blends := w.pieces[:0], w.ways[:0], w.blends[:0]
	clouds, stamps := w.clouds, w.cloudStamp
	if len(clouds) != len(l.clouds) {
		clouds, stamps = make([]float32, len(l.clouds)), make([]uint32, len(l.clouds))
	}
	*w = *l
	w.tile, w.pieces, w.ways, w.blends = tile{}, pieces, ways, blends
	w.clouds, w.cloudStamp = clouds, stamps
	w.workers, w.canvas, w.scratch, w.bakeTile, w.newest, w.unpainted, w.albedo = nil, render.Frame{}, board.Tile{}, tile{}, nil, nil, nil
	w.frozen = true
	return w
}

var _ board.ParallelLook = boardLook{}

// Worker is the board look drawing by the dresser's worker d — the board.ParallelLook contract.
func (boardLook) Worker(_ int, d board.Dressing) board.Look { return boardLook{d: d.(*dresser)} }
