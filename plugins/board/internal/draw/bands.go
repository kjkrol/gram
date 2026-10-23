package draw

import (
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/render"
)

// Ways is what Bands read of a board: what runs across each cell and the markers of the effects
// on it, which the covers read.
type Ways interface {
	Way(c cell.ID) cell.Way
	States(c cell.ID) tag.Tags[effect.States]
}

// Bands is the simple map's Dressing: what lies on the cells under an effect (Cover), then the
// ways over the tiles as bands cut out of their kinds' sprites, as wide as the way, faded as far
// as it has faded.
type Bands struct {
	board  Ways
	covers []Cover
}

var _ look.Dressing = (*Bands)(nil)
var _ look.Laid = (*Bands)(nil)

// NewBands lays the ways of b.
func NewBands(b Ways) *Bands { return &Bands{board: b} }

func (*Bands) Begin(*render.Frame, camera.Camera)                  {}
func (*Bands) Base(_ cell.ID, own render.SpriteID) render.SpriteID { return own }

// Way is what runs across c.
func (d *Bands) Way(c cell.ID) cell.Way { return d.board.Way(c) }

// Covers are the covers' sprites in the order they are laid.
func (d *Bands) Covers() []render.SpriteID {
	out := make([]render.SpriteID, len(d.covers))
	for i, c := range d.covers {
		out[i] = c.Sprite
	}
	return out
}

// Covered is which covers lie on c, bit i the i-th's.
func (d *Bands) Covered(c cell.ID) uint8 {
	if len(d.covers) == 0 {
		return 0
	}
	var bits uint8
	states := d.board.States(c)
	for i, cv := range d.covers[:min(len(d.covers), 8)] {
		if states.Has(cv.Mark) {
			bits |= 1 << i
		}
	}
	return bits
}
