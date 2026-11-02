package roads

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/roads/site"
	"github.com/kjkrol/uid"
)

// boardGround is the board as site plans a road over it, what stands on it as last gathered
type boardGround struct {
	p        *Plugin
	query    *goke.Query // nil before the ECS is set up
	on       goke.Comp[cell.On]
	standing map[cell.ID]struct{}
	outline  []geom.Vec
}

var _ site.Ground = (*boardGround)(nil)

// readied is the ground the plans read, what stands on the board gathered anew
func (p *Plugin) readied() *boardGround {
	g := &p.ground
	clear(g.standing)
	if g.query == nil {
		return g
	}
	for g.query.All(); g.query.Next(); {
		for _, on := range g.on.Slice(g.query.Cursor()) {
			g.standing[on.Cell] = struct{}{}
		}
	}
	return g
}

func (g *boardGround) board() *board.BoardRead { return g.p.board.Read(0).Board() }

func (g *boardGround) Grid() grid.Grid { return g.board().Grid }

func (g *boardGround) Heights(c cell.ID, dst []float32) []float32 {
	g.outline = g.board().CellOutline(c, g.outline[:0])
	heights := g.p.board.Read(0).Heights()
	for _, at := range g.outline {
		if heights == nil {
			dst = append(dst, 0)
		} else {
			dst = append(dst, heights.At(at))
		}
	}
	return dst
}

// Takes is what a road may run over: land, a ford's and a deck's included, never what is solid.
func (g *boardGround) Takes(c cell.ID) bool {
	k := g.board().Kind(c)
	return k.Allows&cell.Land != 0 && !k.Solid
}

// Wet is water a road may not take: a ford, a deck over water, lets land over it too.
func (g *boardGround) Wet(c cell.ID) bool {
	k := g.board().Kind(c)
	return k.Allows&cell.Water != 0 && k.Allows&cell.Land == 0
}

// Road is a way land is crossed by, a track's too; a river is no road.
func (g *boardGround) Road(c cell.ID) cell.Links {
	if w := g.board().Way(c); w.Runs() && w.Kind.Allows&cell.Land != 0 {
		return w.Links
	}
	return 0
}

func (g *boardGround) Decked(c cell.ID) (cell.Links, bool) { return g.deck(c) }

// deck is the links of the bridge's deck over c, the way it runs; false where none runs
func (g *boardGround) deck(c cell.ID) (cell.Links, bool) {
	f := g.board().Built(c).Floor
	return f.Links, f.Graded && f.Width > 0 && f.Links != 0 && f.Allows&cell.Land != 0
}

// Ford is water land crosses, a kind of cell's or a floor's laid over it, where no deck runs.
func (g *boardGround) Ford(c cell.ID) bool {
	k := g.board().Kind(c)
	_, decked := g.deck(c)
	return k.Allows&cell.Water != 0 && k.Allows&cell.Land != 0 && !decked
}

// Fixed is a ford's bed too. A road standing is not: a plan extending it levels it under its new
// links, and the shaping takes back whatever breaks one the plan does not lay anew.
func (g *boardGround) Fixed(c cell.ID) bool {
	_, stands := g.standing[c]
	return stands || g.board().Kind(c).Allows&cell.Water != 0
}

func (g *boardGround) Held(c cell.ID) bool {
	held := false
	g.p.board.Read(0).Occupancy().Holders(c, func(_ uid.UID64, _ cell.Domain, deck bool) {
		held = held || !deck
	})
	return held
}

func (g *boardGround) Price(cell.ID) float32 { return 1 }

// joins are the ways out of c toward the neighbours whose roads end toward it
func (g *boardGround) joins(c cell.ID) cell.Links {
	gr := g.Grid()
	var links cell.Links
	for i := range gr.Directions() {
		if n, ok := gr.Toward(c, i); ok && g.Road(n).Has(gr.Opposite(i)) {
			links |= 1 << i
		}
	}
	return links
}

// corner is a corner of the ground by where it lies, the cells round it sharing it
type corner [2]int32

// cornerAt is the corner at p, of a grid whose cells are span across: its place to a thousandth
// of a cell, as site's levelling tells corners apart
func cornerAt(p geom.Vec, span float32) corner {
	q := 1e-3 * span
	return corner{int32(f32.Round(p.X / q)), int32(f32.Round(p.Y / q))}
}

// movesOf is the heights e's levelling leaves its corners at, in moves' room: none where it moves
// none
func (p *Plugin) movesOf(e site.Estimate, moves map[corner]float32) map[corner]float32 {
	clear(moves)
	if len(e.Corners) == 0 {
		return moves
	}
	if moves == nil {
		moves = make(map[corner]float32, len(e.Corners))
	}
	span := p.board.Read(0).Board().CellSpan()
	for _, m := range e.Corners {
		moves[cornerAt(m.At, span)] = m.To
	}
	return moves
}

// heightsOf is c's outline, in outline's room, and the heights its corners lie at once moves are
// made, in heights': the ground's where moves move none
func (p *Plugin) heightsOf(c cell.ID, moves map[corner]float32, outline []geom.Vec,
	heights []float32) ([]geom.Vec, []float32) {
	brd := p.board.Read(0).Board()
	ground := p.board.Read(0).Heights()
	span := brd.CellSpan()
	outline, heights = brd.CellOutline(c, outline[:0]), heights[:0]
	for _, at := range outline {
		h, moved := moves[cornerAt(at, span)]
		if !moved && ground != nil {
			h = ground.At(at)
		}
		heights = append(heights, h)
	}
	return outline, heights
}
