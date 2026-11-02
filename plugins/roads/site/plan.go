package site

import (
	"math"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
)

// Plan is the Estimate of a road of the kind named kind, width wide and Graded as graded says,
// laid through cells in order over g under r, each linked to the next where they neighbour and out
// as extra says (nil, or one entry a cell: a lone piece's way out) and, with levelling, levelled
// with the ground round it following within step (the shaping's MaxStep, 0 for none). On a Refusal
// of the first rule it breaks the Estimate holds what was worked out before it.
func Plan(g Ground, r Rules, cells []cell.ID, kind string, width float32, graded, levelling bool,
	step float32, extra []cell.Links) (Estimate, error) {
	if len(cells) == 0 {
		return Estimate{}, nil
	}
	gr := g.Grid()
	if r.Longest > 0 && len(cells) > r.Longest {
		return Estimate{}, Refuse(gr, cells[r.Longest], "runs %d cells, %d at most", len(cells),
			r.Longest)
	}
	d := &draft{g: g, gr: gr, r: r, at: make(map[cell.ID]int, len(cells))}
	if err := d.route(cells, extra); err != nil {
		return d.e, err
	}
	if err := d.take(); err != nil {
		return d.e, err
	}
	if err := d.ends(); err != nil {
		return d.e, err
	}
	for i, c := range d.e.Cells {
		if d.laid[i] {
			d.e.Ways = append(d.e.Ways, cell.WayEntry{Kind: kind, Cell: c, Width: width,
				Links: d.e.Links[i]})
			if d.e.Links[i]&^g.Road(c) != 0 {
				d.e.Laid++
			}
		}
	}
	switch {
	case !graded:
		return d.e, d.track(cells)
	case !levelling:
		return d.e, d.fits()
	}
	return d.e, d.grade(step)
}

// the road worked out cell by cell before the ground under it is
type draft struct {
	g  Ground
	gr grid.Grid
	r  Rules
	e  Estimate
	at map[cell.ID]int
	// by e.Cells: a way laid on it, a road standing beside joined
	laid, joined []bool
}

func (d *draft) add(c cell.ID) {
	if _, ok := d.at[c]; ok {
		return
	}
	d.at[c] = len(d.e.Cells)
	d.e.Cells = append(d.e.Cells, c)
	d.e.Links = append(d.e.Links, 0)
	d.laid = append(d.laid, false)
	d.joined = append(d.joined, false)
}

// route links every cell to the next where the two neighbour, a road network's legs lying apart
// left unlinked, and out as extra says; the length counts the steps no road standing runs already
func (d *draft) route(cells []cell.ID, extra []cell.Links) error {
	gr := d.gr
	for i, c := range cells {
		if _, ok := gr.Ordinal(c); !ok {
			return Refuse(gr, c, "lies off the board")
		}
		d.add(c)
		if i == 0 {
			continue
		}
		a := cells[i-1]
		if way, ok := grid.Way(gr, a, c); ok {
			back := gr.Opposite(way)
			d.e.Links[d.at[a]] |= 1 << way
			d.e.Links[d.at[c]] |= 1 << back
			if !d.g.Road(a).Has(way) || !d.g.Road(c).Has(back) {
				st := gr.Step(way)
				d.e.Length += f32.Hypot(st.X, st.Y)
			}
		}
	}
	for i, links := range extra[:min(len(extra), len(cells))] {
		c := cells[i]
		for way := range 8 {
			if _, ok := gr.Toward(c, way); links.Has(way) && !ok {
				return Refuse(gr, c, "runs off the board")
			}
		}
		d.e.Links[d.at[c]] |= links
	}
	return nil
}

// take sorts the cells into those the road is laid on and the decks and fords it runs over, and
// joins it to the roads standing; a bridge's head, its deck running one way out of it, takes the
// road as ground does
func (d *draft) take() error {
	g, gr, r := d.g, d.gr, d.r
	for i, c := range d.e.Cells {
		deck, decked := g.Decked(c)
		if decked && !r.Decks {
			return Refuse(gr, c, "runs over a bridge's deck")
		}
		switch {
		case spans(g, c):
			if d.e.Links[i]&^deck != 0 {
				return Refuse(gr, c, "leaves a bridge's deck sideways")
			}
			d.e.Decks = append(d.e.Decks, c)
			continue
		case g.Ford(c):
			if !r.Fords {
				return Refuse(gr, c, "runs through a ford")
			}
			d.e.Fords = append(d.e.Fords, c)
			continue
		case g.Wet(c):
			return Refuse(gr, c, "runs into water")
		case !g.Takes(c):
			return Refuse(gr, c, "runs over ground that takes no road")
		}
		d.laid[i] = true
		if decked {
			d.e.Links[i] |= deck // a head: the road leads onto the bridge
		}
		meets := false
		if old := g.Road(c); old != 0 {
			if !r.Crossings {
				return Refuse(gr, c, "crosses a road")
			}
			d.e.Links[i] |= old
			meets = true
		}
		if r.Crossings {
			for way := range gr.Directions() {
				n, ok := gr.Toward(c, way)
				if ok && !d.e.Links[i].Has(way) && g.Road(n).Has(gr.Opposite(way)) {
					d.e.Links[i] |= 1 << way
					d.joined[i], meets = true, true
				}
			}
		}
		if meets {
			d.e.Crossings = append(d.e.Crossings, c)
		}
	}
	return nil
}

// ends refuses a road beginning loose, unless the Rules let it; the last cell is its target
func (d *draft) ends() error {
	c := d.e.Cells[0]
	if _, decked := d.g.Decked(c); d.r.Ends || decked || d.g.Road(c) != 0 || d.joined[0] {
		return nil
	}
	return Refuse(d.gr, c, "begins loose, on no road nor deck")
}

// track refuses a track climbing steeper than the Rules let it from a cell's middle to the next
// one's, the middle at the mean of the cell's corners; a span's ground lies under its deck
func (d *draft) track(cells []cell.ID) error {
	g, gr := d.g, d.gr
	eps := 1e-4 * gr.CellSpan()
	var buf []float32
	for i := 1; i < len(cells); i++ {
		a, b := cells[i-1], cells[i]
		way, ok := grid.Way(gr, a, b)
		if !ok || spans(g, a) || spans(g, b) {
			continue
		}
		buf = g.Heights(a, buf[:0])
		ha := midHeight(buf)
		buf = g.Heights(b, buf[:0])
		hb := midHeight(buf)
		st := gr.Step(way)
		s := math.Atan2(float64(max(0, f32.Abs(hb-ha)-eps)), float64(f32.Hypot(st.X, st.Y)))
		if s > d.r.Slope {
			return Refuse(gr, b, "climbs %.1f°, %.1f° at most", inDegrees(s), inDegrees(d.r.Slope))
		}
	}
	return nil
}

// fits refuses a Graded road on ground it does not fit as it lies
func (d *draft) fits() error {
	var buf []float32
	for i, c := range d.e.Cells {
		if !d.laid[i] {
			continue
		}
		buf = d.g.Heights(c, buf[:0])
		if !ground.RoadFits(d.gr, c, buf, d.e.Links[i]) {
			return Refuse(d.gr, c, "lies on ground it does not fit, and nothing is levelled")
		}
		if err := d.climbs(c, buf, d.e.Links[i]); err != nil {
			return err
		}
	}
	return nil
}

// the steepest a Graded road may climb: the Rules' Slope, RoadSlope at most
func (d *draft) slope() float64 { return max(0, min(d.r.Slope, ground.RoadSlope)) }

// climbs refuses a road straight through c whose corners make a ramp steeper than the Rules let it
func (d *draft) climbs(c cell.ID, corners []float32, links cell.Links) error {
	gr := d.gr
	way, ok := grid.Straight(gr, links)
	if !ok || gr.Corner(way) {
		return nil
	}
	in, _ := grid.Rim(gr, c, gr.Opposite(way))
	out, _ := grid.Rim(gr, c, way)
	st := gr.Step(way)
	rise := max(0, f32.Abs(corners[out]-corners[in])-1e-4*gr.CellSpan())
	if s := math.Atan2(float64(rise), float64(f32.Hypot(st.X, st.Y))); s > d.slope() {
		return Refuse(gr, c, "climbs %.1f°, %.1f° at most", inDegrees(s), inDegrees(d.slope()))
	}
	return nil
}

// spans reports whether a bridge's span lies over c: its deck running more than one way out
func spans(g Ground, c cell.ID) bool {
	deck, decked := g.Decked(c)
	return decked && deck&(deck-1) != 0
}

func midHeight(heights []float32) float32 {
	sum := 0.0
	for _, h := range heights {
		sum += float64(h)
	}
	return float32(sum / float64(max(len(heights), 1)))
}

func inDegrees(radians float64) float64 { return radians * 180 / math.Pi }
