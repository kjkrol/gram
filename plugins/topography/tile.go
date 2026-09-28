package topography

import (
	"github.com/kjkrol/gram/render"
)

// tileMemo is what a tile has worked out of itself already, so a Look asking twice pays once.
type tileMemo struct {
	has   uint8 // which of the rest hold: memoLight, memoLit, memoBase, memoCloud
	light render.Shade
	lit   [4]float32
	base  *cellTop
	cloud [4]float32
}

const (
	memoLight = 1 << iota
	memoLit
	memoBase
	memoCloud
)

// clouds is the clouds' noise at the tile's corners this frame, worked out once a tile.
func (t *tile) clouds() [4]float32 {
	if t.memo.has&memoCloud == 0 {
		t.memo.cloud, t.memo.has = t.r.cloudsOf(t.Tile), t.memo.has|memoCloud
	}
	return t.memo.cloud
}

// baseTop is Base's cell, worked out once a tile.
func (t *tile) baseTop() *cellTop {
	if t.memo.has&memoBase == 0 {
		t.memo.base, t.memo.has = t.r.base(t.ID), t.memo.has|memoBase
	}
	return t.memo.base
}
