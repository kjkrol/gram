package board

import (
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
)

// Map is what a board is drawn by and how its ground lies beyond what its cells say, given by
// WithMap: how it looks, its heights and how steep a step is.
type Map interface {
	// Look is how the cells lie on the screen through a camera.
	Look() look.Look
	// Heights is the height of the ground at any point; nil on a flat map.
	Heights() ground.Heights
	// Dressing lays over the tiles what lies on them beyond their sprites; nil for nothing.
	Dressing() look.Dressing
	// Top is the height of c's corners as drawn (top-left, top-right, bottom-left, bottom-right)
	// with height, its kind's, standing on them, and of its ground; zero on a flat map.
	Top(c cell.ID, height float32) (corners [4]float32, level float32)
	// Rise is how steep the step from one cell to its neighbour is: the ground's rise between
	// their middles over the run, negative downhill; 0 on a flat map.
	Rise(from, to cell.ID) float64
}

// MapRead is a board's Map as a BoardRead reads it: a look.Map, a cell's kind read by the
// BoardRead, and how steep a step is.
type MapRead interface {
	look.Map
	Rise(from, to cell.ID) float64
}

// readMap is the board's Map as it is when asked, so WithMap after the atlas closed still counts
type readMap struct{ r *BoardRead }

func (m readMap) Look() look.Look         { return m.r.b.mapping.Look() }
func (m readMap) Heights() ground.Heights { return m.r.b.mapping.Heights() }
func (m readMap) Dressing() look.Dressing { return m.r.b.mapping.Dressing() }

func (m readMap) Top(c cell.ID) (corners [4]float32, level float32) {
	return m.r.b.mapping.Top(c, m.r.Kind(c).Height)
}

func (m readMap) Rise(from, to cell.ID) float64 { return m.r.b.mapping.Rise(from, to) }

type simpleMap struct {
	look     look.Look
	dressing *draw.Bands
}

func newSimpleMap(b drawnBoard) *simpleMap {
	return &simpleMap{look: look.Flat, dressing: draw.NewBands(b)}
}

func (m *simpleMap) Look() look.Look                                        { return m.look }
func (*simpleMap) Heights() ground.Heights                                  { return nil }
func (m *simpleMap) Dressing() look.Dressing                                { return m.dressing }
func (*simpleMap) Top(cell.ID, float32) (corners [4]float32, level float32) { return corners, 0 }
func (*simpleMap) Rise(cell.ID, cell.ID) float64                            { return 0 }
