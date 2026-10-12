package terrain

import (
	"fmt"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Bridges is what a bridge — cells whose Graded decks run as bands joined along their links —
// keeps to, and what the board reads the ground with to tell: its deck Clearance over the ground
// under each cell between its heads at least.
type Bridges struct {
	Clearance float64
	Level     func(c cell.ID) float64  // a cell's ground level; nil, 0
	Ground    func(p geom.Vec) float64 // the ground at a point; nil, flat at 0
	Where     func(c cell.ID) string   // the file and line the entity on c was laid by
	Log       func(format string, args ...any)
}

// SetBridges has the bridges keep to b.
func (t *Cells) SetBridges(b Bridges) { t.bridges = b }

// keepBridges takes the decks and the railings off every bridge among sites that breaks a rule:
// straight, level, from the middle of a flat tile at its deck to another, its deck over the ground
// under it; bands not Graded are no bridge. The first time — the board as it was laid — a broken
// one panics, saying where; after it, it is logged.
func (t *Cells) keepBridges(sites []site, first bool) {
	if !slices.ContainsFunc(sites, func(s site) bool { return banded(&s.built) }) {
		return
	}
	at := map[cell.ID]int{}
	for k, s := range sites {
		if b := s.built; banded(&b) {
			at[s.cell] = k
		}
	}
	seen := map[cell.ID]bool{}
	for k := range sites {
		c := sites[k].cell
		if _, ok := at[c]; !ok || seen[c] {
			continue
		}
		span := t.span(c, sites, at, seen)
		if !slices.ContainsFunc(span, func(c cell.ID) bool {
			return sites[at[c]].built.Floor.Graded
		}) {
			continue
		}
		why := t.broken(span, sites, at)
		if why == "" {
			continue
		}
		where := ""
		if t.bridges.Where != nil {
			if w := t.bridges.Where(span[0]); w != "" {
				where = w + ": "
			}
		}
		msg := fmt.Sprintf("board: %s%s", where, why)
		if first {
			panic(msg)
		}
		if t.bridges.Log != nil {
			t.bridges.Log("%s; its decks and railings are not built", msg)
		}
		for _, c := range span {
			s := &sites[at[c]]
			s.built.Floor, s.built.Rail = cell.Floor{}, cell.Rail{}
		}
	}
}

// span is the bridge c is a cell of: every cell a band links to, or whose band links back, from
// c on, c first.
func (t *Cells) span(c cell.ID, sites []site, at map[cell.ID]int, seen map[cell.ID]bool) []cell.ID {
	out := []cell.ID{c}
	seen[c] = true
	for k := 0; k < len(out); k++ {
		links := sites[at[out[k]]].built.Floor.Links
		for i := range t.grid.Directions() {
			n, ok := t.grid.Toward(out[k], i)
			j, decked := at[n]
			if !ok || !decked || seen[n] ||
				!links.Has(i) && !sites[j].built.Floor.Links.Has(t.grid.Opposite(i)) {
				continue
			}
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// broken is the rule span breaks, said; "" for none: a bridge runs straight and level from a head
// to a head — a band from the middle of a flat tile at its deck into the bridge — its deck
// Clearance over the ground under the middle of every cell between.
func (t *Cells) broken(span []cell.ID, sites []site, at map[cell.ID]int) string {
	floor := func(c cell.ID) cell.Floor { return sites[at[c]].built.Floor }
	eps := 1e-3 * float64(t.grid.CellSpan())
	deck := t.level(span[0]) + floor(span[0]).Height
	x0, y0, _ := t.grid.Coords(span[0])
	var axis cell.Links
	heads := 0
	for _, c := range span {
		f := floor(c)
		x, y, _ := t.grid.Coords(c)
		d, through, ok := t.run(f)
		if ok && axis == 0 {
			axis = 1<<d | 1<<t.grid.Opposite(d)
		}
		if !ok || f.Links&^axis != 0 {
			return fmt.Sprintf("the bridge at [%d, %d] turns: a bridge runs straight", x, y)
		}
		if h := t.level(c) + f.Height; math.Abs(h-deck) > eps {
			return fmt.Sprintf("the bridge's deck stands at %.2f at [%d, %d], at %.2f at "+
				"[%d, %d]: a bridge is level", deck, x0, y0, h, x, y)
		}
		if through {
			if over := deck - t.ground(t.grid.CellCenter(c)); over < t.bridges.Clearance-eps {
				return fmt.Sprintf("the bridge's deck at [%d, %d] stands %.2f over the ground: "+
					"a bridge clears it by %.2f", x, y, over, t.bridges.Clearance)
			}
			continue
		}
		heads++
		if n, ok := t.grid.Toward(c, d); !ok || !slices.Contains(span, n) {
			return fmt.Sprintf("the bridge's head at [%d, %d] runs out of it: a head's band "+
				"runs into its bridge", x, y)
		}
		if off := t.offFlat(c, deck); off > eps {
			return fmt.Sprintf("the bridge's head at [%d, %d] stands %.2f off its deck: a "+
				"bridge begins and ends on a flat tile at its deck", x, y, off)
		}
	}
	if heads != 2 {
		return fmt.Sprintf("the bridge at [%d, %d] has %d heads: a bridge begins and ends in "+
			"the middle of a tile, a head at either end", x0, y0, heads)
	}
	return ""
}

// offFlat is how far the ground anywhere on c — its middle and its corners — lies off top.
func (t *Cells) offFlat(c cell.ID, top float64) float64 {
	m := t.grid.CellCenter(c)
	off := math.Abs(t.ground(m) - top)
	t.outline = t.grid.CellOutline(c, t.outline[:0])
	for _, p := range t.outline {
		in := geom.NewVec(p.X+(m.X-p.X)*1e-3, p.Y+(m.Y-p.Y)*1e-3)
		off = max(off, math.Abs(t.ground(in)-top))
	}
	return off
}

func (t *Cells) level(c cell.ID) float64 {
	if t.bridges.Level == nil {
		return 0
	}
	return t.bridges.Level(c)
}

func (t *Cells) ground(p geom.Vec) float64 {
	if t.bridges.Ground == nil {
		return 0
	}
	return t.bridges.Ground(p)
}
