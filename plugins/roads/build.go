package roads

import (
	"log"
	"slices"
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/roads/site"
)

// laidWay is a way the road lays across a cell, and the way that ran there before
type laidWay struct {
	at       cell.ID
	way, was cell.Way
}

// lay builds the road o ordered and tells Built and keeps its receipt. The ways across the roads
// it extends are laid first: the shaping keeps a road standing as it fits its links, and an end
// turned into a ramp fits only its new ones. Then every cell of its ways is graded in turn, as
// site.Plan foresaw (the cells only rippled to follow), their fit checked and the rest laid.
func (p *Plugin) lay(o Order, step time.Duration) {
	ways, ok := p.waysOf(o.Estimate)
	if !ok {
		return
	}
	corners, ok := p.cornersOf(o.Estimate, ways)
	if !ok {
		return
	}
	write := p.board.Write().Board()
	for _, w := range ways {
		if w.was.Runs() {
			write.SetWay(w.at, w.way)
		}
	}
	if !p.graded(ways, corners) || !p.fits(ways) {
		p.restore(ways)
		return
	}
	for _, w := range ways {
		if !w.was.Runs() {
			write.SetWay(w.at, w.way)
		}
	}
	if !p.built.Empty() {
		p.built.Run(p.worldPlugin.Read(0).Tick(step),
			Built{Self: p.Entity(), Estimate: o.Estimate, Price: o.Price})
	}
	p.Res.Receipts = append(p.Res.Receipts, Receipt(o))
}

// waysOf are the ways e lays, as the plan gave them: none on a bridge's span or a ford, which
// the road leads onto as a map's road does, a head's leading onto the span. A cell the road runs
// through twice runs out every way it leaves by.
func (p *Plugin) waysOf(e site.Estimate) ([]laidWay, bool) {
	kinds := p.board.Read(0).CellKinds()
	brd := p.board.Read(0).Board()
	ways := make([]laidWay, 0, len(e.Ways))
	for _, w := range e.Ways {
		if k := slices.IndexFunc(ways, func(l laidWay) bool { return l.at == w.Cell }); k >= 0 {
			ways[k].way.Links |= w.Links
			continue
		}
		k, ok := kinds.Get(w.Kind)
		if !ok {
			log.Printf("roads: not built: no kind of cell is called %q", w.Kind)
			return nil, false
		}
		ways = append(ways, laidWay{at: w.Cell, way: w.Way(k), was: brd.Way(w.Cell)})
	}
	return ways, true
}

// cornersOf is where e has every cell of ways put its corners, in CellOutline's order, read off
// the ground before anything is laid; none where e levels nothing, false where it levels and no
// ground is given to level
func (p *Plugin) cornersOf(e site.Estimate, ways []laidWay) ([][]float32, bool) {
	if !e.Levels() {
		return nil, true
	}
	if p.shaper == nil {
		log.Print("roads: not built: the road levels the ground, and no ground is given to " +
			"level (WithShaper)")
		return nil, false
	}
	g := p.board.Read(0).Board().Grid
	corners := make([][]float32, len(ways))
	for k, w := range ways {
		corners[k] = e.Levelled(g, w.at, p.ground.Heights(w.at, nil))
	}
	return corners, true
}

// graded grades the cell of every way at its corners, in turn; false where the shaping took a
// grade back
func (p *Plugin) graded(ways []laidWay, corners [][]float32) bool {
	for k, c := range corners {
		if !p.shaper.Grade(ways[k].at, c) {
			x, y, _ := p.board.Read(0).Board().Coords(ways[k].at)
			log.Printf("roads: not built: the ground at [%d, %d] could not be levelled without "+
				"breaking a road", x, y)
			return false
		}
	}
	return true
}

// fits reports whether the ground lies under every Graded way as Transport Tycoon lays a road:
// the board keeps no road that does not
func (p *Plugin) fits(ways []laidWay) bool {
	for _, w := range ways {
		if !p.fit(w.at, w.way) {
			x, y, _ := p.board.Read(0).Board().Coords(w.at)
			log.Printf("roads: not built: the ground at [%d, %d] lies as no road may", x, y)
			return false
		}
	}
	return true
}

func (p *Plugin) fit(c cell.ID, w cell.Way) bool {
	if !w.Kind.Graded || p.board.Read(0).Heights() == nil {
		return true
	}
	p.outline, p.heights = p.heightsOf(c, nil, p.outline, p.heights)
	return ground.RoadFits(p.board.Read(0).Board().Grid, c, p.heights, w.Links)
}

// restore lays back the roads a build that failed extended, each where it still fits the ground
// as the grades left it: else the extended one, which the shaping kept fitting
func (p *Plugin) restore(ways []laidWay) {
	write := p.board.Write().Board()
	for _, w := range ways {
		if w.was.Runs() && p.fit(w.at, w.was) {
			write.SetWay(w.at, w.was)
		}
	}
}
