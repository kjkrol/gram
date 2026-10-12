package painter

import (
	"github.com/kjkrol/gram/render"
)

type tileMemo struct {
	has   uint8
	light render.Shade
	lit   [4]float32
	base  *cellTop
}

const (
	memoLight = 1 << iota
	memoLit
	memoBase
)

func (t *tile) baseTop() *cellTop {
	if t.memo.has&memoBase == 0 {
		t.memo.base, t.memo.has = t.r.base(t.ID), t.memo.has|memoBase
	}
	return t.memo.base
}
