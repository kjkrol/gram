package site_test

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/roads/site"
)

// fakeGround is a Ground of maps: a cell missing from heights lies level at 0, one missing from
// takes takes a road, one missing from price costs 1, one in decks has a deck running its links.
type fakeGround struct {
	grid    grid.Grid
	heights map[cell.ID][]float32
	takes   map[cell.ID]bool
	wet     map[cell.ID]bool
	roads   map[cell.ID]cell.Links
	decks   map[cell.ID]cell.Links
	fords   map[cell.ID]bool
	fixed   map[cell.ID]bool
	held    map[cell.ID]bool
	price   map[cell.ID]float32
	outline []geom.Vec
}

var _ site.Ground = (*fakeGround)(nil)

func newFakeGround(g grid.Grid) *fakeGround {
	return &fakeGround{
		grid:    g,
		heights: map[cell.ID][]float32{},
		takes:   map[cell.ID]bool{},
		wet:     map[cell.ID]bool{},
		roads:   map[cell.ID]cell.Links{},
		decks:   map[cell.ID]cell.Links{},
		fords:   map[cell.ID]bool{},
		fixed:   map[cell.ID]bool{},
		held:    map[cell.ID]bool{},
		price:   map[cell.ID]float32{},
	}
}

func (f *fakeGround) Grid() grid.Grid { return f.grid }

func (f *fakeGround) Heights(c cell.ID, dst []float32) []float32 {
	if h, ok := f.heights[c]; ok {
		return append(dst, h...)
	}
	f.outline = f.grid.CellOutline(c, f.outline[:0])
	for range f.outline {
		dst = append(dst, 0)
	}
	return dst
}

func (f *fakeGround) Takes(c cell.ID) bool {
	t, ok := f.takes[c]
	return !ok || t
}

func (f *fakeGround) Wet(c cell.ID) bool        { return f.wet[c] }
func (f *fakeGround) Road(c cell.ID) cell.Links { return f.roads[c] }
func (f *fakeGround) Ford(c cell.ID) bool       { return f.fords[c] }
func (f *fakeGround) Fixed(c cell.ID) bool      { return f.fixed[c] }
func (f *fakeGround) Held(c cell.ID) bool       { return f.held[c] }

func (f *fakeGround) Decked(c cell.ID) (cell.Links, bool) {
	l, ok := f.decks[c]
	return l, ok
}

func (f *fakeGround) Price(c cell.ID) float32 {
	if p, ok := f.price[c]; ok {
		return p
	}
	return 1
}
