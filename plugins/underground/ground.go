package underground

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/underground/site"
)

type boardGround struct {
	brd     *board.BoardRead
	heights func(p geom.Vec) float32 // nil: flat at 0
	allows  cell.Domain
	planned map[cell.ID]cell.Built // a level's, before its entities are
	unlaid  bool                   // the board's cells are not laid yet: any ground takes a head
	outline []geom.Vec
}

var _ site.Ground = (*boardGround)(nil)

func newGround(brd *board.BoardRead, heights func(p geom.Vec) float32,
	allows cell.Domain) *boardGround {
	return &boardGround{brd: brd, heights: heights, allows: allows}
}

func (g *boardGround) Grid() grid.Grid { return g.brd.Grid }

func (g *boardGround) Heights(c cell.ID, dst []float32) []float32 {
	g.outline = g.brd.CellOutline(c, g.outline[:0])
	for _, p := range g.outline {
		dst = append(dst, g.at(p))
	}
	return dst
}

func (g *boardGround) at(p geom.Vec) float32 {
	if g.heights == nil {
		return 0
	}
	return g.heights(p)
}

func (g *boardGround) level(c cell.ID) float32 {
	var buf [6]float32
	hs := g.Heights(c, buf[:0])
	sum := float32(0)
	for _, h := range hs {
		sum += h
	}
	return sum / float32(len(hs))
}

func (g *boardGround) Takes(c cell.ID) bool {
	if g.unlaid {
		return true
	}
	k := g.brd.Kind(c)
	return k.Allows&g.allows != 0 && !k.Solid
}

func (g *boardGround) Built(c cell.ID) cell.Built {
	b := g.brd.Built(c)
	if p, ok := g.planned[c]; ok {
		for s := range uint8(cell.Storeys) {
			if f := p.Storey(s); *f != (cell.Floor{}) {
				*b.Storey(s) = *f
			}
		}
	}
	return b
}

// plan notes the floor a structure planned lays under c, for the plans after it
func (g *boardGround) plan(c cell.ID, f cell.Floor) {
	if g.planned == nil {
		g.planned = map[cell.ID]cell.Built{}
	}
	b := g.planned[c]
	*b.Storey(f.Storey) = f
	g.planned[c] = b
}
