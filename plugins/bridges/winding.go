package bridges

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// winding is the bridge r lays along its path: a piece of deck on every cell, each beside the
// one before, the deck running even from the ground under its first head's axis to that under
// its last's, its piers evenly between its heads, about Spec.Span apart
func (p *Plugin) winding(t Type, r Row, g *boardGround) (kind.Entry, []kind.Entry, error) {
	gr := g.Grid()
	if len(r.Path) < 3 {
		return kind.Entry{}, nil, fmt.Errorf("a bridge's path runs over three cells at least")
	}
	cells := make([]cell.ID, len(r.Path))
	at := make([]geom.Vec, len(r.Path))
	run := make([]float32, len(r.Path))
	sq, square := gr.(interface{ At(x, y uint32) cell.ID })
	if !square {
		return kind.Entry{}, nil, fmt.Errorf("a bridge's path lies on a square grid")
	}
	for i, pt := range r.Path {
		cells[i] = sq.At(uint32(pt[0]), uint32(pt[1]))
		m := gr.CellCenter(cells[i])
		at[i] = geom.NewVec(m.X+pt[2], m.Y+pt[3])
		if i == 0 {
			continue
		}
		if _, ok := grid.Link(gr, cells[i-1], cells[i]); !ok {
			return kind.Entry{}, nil, fmt.Errorf("a bridge's path at [%v, %v] runs on from no "+
				"cell beside it", pt[0], pt[1])
		}
		run[i] = run[i-1] + f32.Hypot(at[i].X-at[i-1].X, at[i].Y-at[i-1].Y)
	}
	last := len(cells) - 1
	from, to := g.at(at[0]), g.at(at[last])
	deck := func(i int) float32 { return from + (to-from)*run[i]/run[last] }
	var out []kind.Entry
	built := Wound{Kind: t.name, Width: t.spec.Width, At: at}
	if r.Width > 0 {
		built.Width = r.Width
	}
	for i := range cells {
		built.Deck = append(built.Deck, deck(i))
	}
	p.Res.Wound = append(p.Res.Wound, built)
	p.structure.stale = true
	for i, c := range cells {
		var links cell.Links
		dir := geom.NewVec(0, 0)
		if i > 0 {
			l, _ := grid.Link(gr, c, cells[i-1])
			links |= l
			dir = geom.NewVec(at[i].X-at[i-1].X, at[i].Y-at[i-1].Y)
		}
		if i < last {
			l, _ := grid.Link(gr, c, cells[i+1])
			links |= l
			dir = geom.NewVec(dir.X+at[i+1].X-at[i].X, dir.Y+at[i+1].Y-at[i].Y)
		}
		if n := f32.Hypot(dir.X, dir.Y); n > 0 {
			dir = geom.NewVec(dir.X/n, dir.Y/n)
		}
		row := deckRow{At: c, Links: links, Dir: dir, Winds: true, Width: r.Width,
			Off: geom.NewVec(r.Path[i][2], r.Path[i][3]), Lanes: r.Lanes}
		if i > 0 && i < last {
			row.Height = max(deck(i)-g.level(c), t.spec.Thickness)
		}
		out = append(out, t.deck.Entry(row))
	}
	// a pier wherever the structure drawn stands one: evenly between the heads
	if g.heights != nil && t.spec.Span > 0 {
		bays := max(int(run[last]/t.spec.Span+0.5), 1)
		for k, i := 1, 1; k < bays && i <= last; i++ {
			far := float32(k) * run[last] / float32(bays)
			if run[i] < far {
				continue
			}
			s := (far - run[i-1]) / max(run[i]-run[i-1], f32.Eps(1))
			mid := geom.NewVec(at[i-1].X+(at[i].X-at[i-1].X)*s, at[i-1].Y+(at[i].Y-at[i-1].Y)*s)
			top := from + (to-from)*far/run[last] - max(t.spec.Thickness, t.spec.Girder)
			if top > g.at(mid) {
				out = append(out, t.pier.Entry(pierRow{At: mid, Bottom: g.at(mid), Top: top}))
			}
			k++
			i--
		}
	}
	chord := geom.NewVec(at[last].X-at[0].X, at[last].Y-at[0].Y)
	n := max(f32.Hypot(chord.X, chord.Y), f32.Eps(1))
	whole := wholeRow{From: at[0], To: at[last], Dir: geom.NewVec(chord.X/n, chord.Y/n),
		Deck: (from + to) / 2}
	return t.whole.Entry(whole), out, nil
}
