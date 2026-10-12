package bridges

import (
	"math/bits"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges/site"
)

// boardGround is the board and its heights as a bridge is planned over them: the bridges built
// read off the decks over the cells, and those laid besides — a level's, before its entities are.
type boardGround struct {
	brd     *board.Board
	heights func(p geom.Vec) float64 // nil: flat at 0
	allows  cell.Domain
	hex     bool
	laid    []site.Bridge
	outline []geom.Vec
}

var _ site.Ground = (*boardGround)(nil)

func newGround(brd *board.Board, heights func(p geom.Vec) float64,
	allows cell.Domain) *boardGround {
	shape, _ := grid.ShapeOf(brd.Grid)
	return &boardGround{brd: brd, heights: heights, allows: allows, hex: shape.Hex}
}

func (g *boardGround) Grid() grid.Grid { return g.brd.Grid }

func (g *boardGround) Heights(c cell.ID, dst []float64) []float64 {
	if g.hex || g.heights == nil {
		return append(dst, g.at(g.brd.CellCenter(c)))
	}
	g.outline = g.brd.CellOutline(c, g.outline[:0])
	for _, p := range g.outline {
		dst = append(dst, g.at(p))
	}
	return dst
}

func (g *boardGround) at(p geom.Vec) float64 {
	if g.heights == nil {
		return 0
	}
	return g.heights(p)
}

// level is c's ground level: its heights' mean, as a relief has it.
func (g *boardGround) level(c cell.ID) float64 {
	var buf [4]float64
	hs := g.Heights(c, buf[:0])
	sum := 0.0
	for _, h := range hs {
		sum += h
	}
	return sum / float64(len(hs))
}

func (g *boardGround) Takes(c cell.ID) bool {
	k := g.brd.Kind(c)
	return k.Allows&g.allows != 0 && !k.Solid
}

func (g *boardGround) Wet(c cell.ID) bool { return g.brd.Kind(c).Allows&cell.Water != 0 }

func (g *boardGround) Road(c cell.ID) cell.Links {
	if w := g.brd.Way(c); w.Runs() && w.Kind.Graded {
		return w.Links
	}
	return 0
}

// Bridged reads a bridge's part on c off the laid ones, else off the band over c: a Graded deck
// a link long is a head, longer a span.
func (g *boardGround) Bridged(c cell.ID) (site.Part, bool) {
	gr := g.brd.Grid
	for _, b := range g.laid {
		switch {
		case c == b.From:
			return site.Part{Way: b.Way, Head: true}, true
		case c == b.To:
			return site.Part{Way: gr.Opposite(b.Way), Head: true}, true
		case slices.Contains(b.Spans, c):
			return site.Part{Way: b.Way}, true
		}
	}
	f := g.brd.Built(c).Floor
	if !f.Graded || f.Width <= 0 || f.Links == 0 {
		return site.Part{}, false
	}
	links := uint8(f.Links)
	return site.Part{Way: bits.TrailingZeros8(links), Head: bits.OnesCount8(links) == 1}, true
}
