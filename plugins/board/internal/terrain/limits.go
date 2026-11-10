package terrain

import "github.com/kjkrol/gram/plugins/board/cell"

// limit marks the cells of sites whose floor bears a Load or whose wall has a Gap
func (t *Cells) limit(sites []site) {
	for _, i := range t.limited {
		t.limits[i] = false
	}
	t.limited = t.limited[:0]
	for _, s := range sites {
		b := &s.built
		if b.Floor.Load <= 0 && b.More[0].Load <= 0 && b.More[1].Load <= 0 &&
			b.Wall.Gap.Width <= 0 {
			continue
		}
		i, ok := t.Ordinal(s.cell)
		if !ok {
			continue
		}
		if len(t.limits) != t.grid.CellCount() {
			t.limits = make([]bool, t.grid.CellCount())
		}
		t.limits[i] = true
		t.limited = append(t.limited, i)
	}
}

// Limited reports whether a floor's Load or a wall's Gap limits whom a cell takes anywhere.
func (r *Read) Limited() bool { return len(r.t.limited) > 0 }

// Limits are what c limits a body moving in d by beyond its kind, on level:
// the Load of the floor carrying it where its ground would not, and the Gap of the wall keeping d
// off; zero where nothing does.
func (r *Read) Limits(c cell.ID, d cell.Domain, level cell.Level) (load float32, gap cell.Gap) {
	t := r.t
	if len(t.limited) == 0 || r.store == nil {
		return 0, cell.Gap{}
	}
	i, ok := t.Ordinal(c)
	if !ok || !t.limits[i] {
		return 0, cell.Gap{}
	}
	b := r.store.builtOf(i)
	if b == nil {
		return 0, cell.Gap{}
	}
	f := b.On(level)
	if b.Wall.Keeps&d != 0 && !f.Sunk() {
		gap = b.Wall.Gap
	}
	switch {
	case f.Load <= 0 || f.Allows&d == 0:
	case level != cell.OnGround || !r.store.wayOf(i).Over(r.store.groundOf(i).Kind).Admits(d):
		load = f.Load
	}
	return load, gap
}
