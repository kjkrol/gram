package site

import (
	"fmt"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
)

// Ground is what a structure is dug under: a board, its heights and what stands on it.
type Ground interface {
	Grid() grid.Grid
	// Heights are the ground's heights at c's outline, in CellOutline's order.
	Heights(c cell.ID, dst []float32) []float32
	// Takes reports whether a tunnel's head may open on c: ground its road may take.
	Takes(c cell.ID) bool
	// Built is what stands on c and lies under it, structures planned and not yet dug among it.
	Built(c cell.ID) cell.Built
}

// Rules are where a structure may lie: a tunnel through Longest cells at most (0: any), the roof
// Cover under the ground at least, the floor climbing no steeper than Grade (rise over run; 0:
// any), a head's ground moved no more than Reshape to lie flat at the floor.
type Rules struct {
	Longest int
	Cover   float32
	Grade   float32
	Reshape float32
}

// Tunnel is a tunnel planned from one head to another: the cells it runs under in order, its
// floor's height under each and at the heads, and the storey it takes on every one.
type Tunnel struct {
	From, To cell.ID
	Cells    []cell.ID
	Floors   []float32
	Ends     [2]float32
	Storey   uint8
}

// Chamber is a chamber planned under a rectangle of cells: the cells, its floor's height and the
// storey it takes.
type Chamber struct {
	From, To cell.ID
	Cells    []cell.ID
	Floor    float32
	Storey   uint8
}

// Plan is the tunnel clear high from the head from to the head to through the cells via, or a
// Refusal saying why none may lie there. A head on a cell of a chamber opens into it.
func Plan(g Ground, r Rules, clear float32, from, to cell.ID, via ...cell.ID) (Tunnel, error) {
	gr := g.Grid()
	t := Tunnel{From: from, To: to}
	turns := append(append([]cell.ID{from}, via...), to)
	for k := 1; k < len(turns); k++ {
		between, err := line(gr, turns[k-1], turns[k])
		if err != nil {
			return Tunnel{}, err
		}
		t.Cells = append(t.Cells, between...)
		if k < len(turns)-1 {
			t.Cells = append(t.Cells, turns[k])
		}
	}
	switch {
	case len(t.Cells) == 0:
		return Tunnel{}, refusal(gr, from, "runs under no cell: its heads lie side by side")
	case r.Longest > 0 && len(t.Cells) > r.Longest:
		return Tunnel{}, refusal(gr, from, "runs under %d cells, %d at most", len(t.Cells),
			r.Longest)
	}
	for i, h := range [2]cell.ID{from, to} {
		end, err := head(g, r, h)
		if err != nil {
			return Tunnel{}, err
		}
		t.Ends[i] = end
	}
	run := float32(len(t.Cells) + 1)
	if rise := f32.Abs(t.Ends[1] - t.Ends[0]); r.Grade > 0 &&
		rise > r.Grade*run*gr.CellSpan() {
		return Tunnel{}, refusal(gr, from, "climbs %.1f in %d cells, steeper than %.2f", rise,
			len(t.Cells)+1, r.Grade)
	}
	free := uint8(1<<cell.Storeys - 1)
	var buf [6]float32
	for k, c := range t.Cells {
		floor := t.Ends[0] + (t.Ends[1]-t.Ends[0])*float32(k+1)/run
		t.Floors = append(t.Floors, floor)
		free &= vacant(g, c)
		low := middle(g.Heights(c, buf[:0]))
		// a mouth lies open to the sky: the ground closes over the tunnel within its first cell
		cover := r.Cover
		if k == 0 && !sunk(g, from) || k == len(t.Cells)-1 && !sunk(g, to) {
			cover = 0
		}
		if low < floor+clear+cover {
			return Tunnel{}, refusal(gr, c,
				"the ground lies %.1f over a floor at %.1f, its roof %.1f up and %.1f of rock "+
					"over it at least", low-floor, floor, clear, cover)
		}
	}
	if free == 0 {
		return Tunnel{}, refusal(gr, from, "crosses floors on every storey of a cell")
	}
	t.Storey = first(free)
	return t, nil
}

// Room is the chamber clear high under the rectangle of cells from from to to, its floor depth
// under the lowest ground over it, or a Refusal saying why none may lie there.
func Room(g Ground, r Rules, clear, depth float32, from, to cell.ID) (Chamber, error) {
	gr := g.Grid()
	x0, y0, ok0 := gr.Coords(from)
	x1, y1, ok1 := gr.Coords(to)
	if !ok0 || !ok1 {
		return Chamber{}, refusal(gr, from, "lies off the board")
	}
	ch := Chamber{From: from, To: to}
	lowest := f32.Inf(1)
	free := uint8(1<<cell.Storeys - 1)
	var buf [6]float32
	for y := min(y0, y1); y <= max(y0, y1); y++ {
		for x := min(x0, x1); x <= max(x0, x1); x++ {
			c := gr.At(x, y)
			ch.Cells = append(ch.Cells, c)
			free &= vacant(g, c)
			for _, h := range g.Heights(c, buf[:0]) {
				lowest = min(lowest, h)
			}
		}
	}
	ch.Floor = lowest - depth
	if depth < clear+r.Cover {
		return Chamber{}, refusal(gr, from,
			"lies %.1f under the ground, its roof %.1f up and %.1f of rock over it at least",
			depth, clear, r.Cover)
	}
	if free == 0 {
		return Chamber{}, refusal(gr, from, "crosses floors on every storey of a cell")
	}
	ch.Storey = first(free)
	return ch, nil
}

// head is the height a tunnel's floor has at its head h: the floor of the chamber it opens into,
// else the ground's, which lies flat there
func head(g Ground, r Rules, h cell.ID) (float32, error) {
	gr := g.Grid()
	var buf [6]float32
	hs := g.Heights(h, buf[:0])
	level := mean(hs)
	b := g.Built(h)
	for s := range uint8(cell.Storeys) {
		if f := b.Storey(s); f.Sunk() && f.Width == 0 {
			return level + f.Height, nil
		}
	}
	if !g.Takes(h) {
		return 0, refusal(gr, h, "a head opens on ground its road may not take")
	}
	if b.Levelled() {
		return 0, refusal(gr, h, "a head opens under a bridge or over a tunnel")
	}
	for _, v := range hs {
		if d := f32.Abs(v - level); d > r.Reshape+1e-6*max(1, f32.Abs(level)) {
			return 0, refusal(gr, h, "a head's ground lies %.1f off level, %.1f at most", d,
				r.Reshape)
		}
	}
	return level, nil
}

// sunk says whether a chamber lies under c, a tunnel's head opening into it
func sunk(g Ground, c cell.ID) bool {
	b := g.Built(c)
	for s := range uint8(cell.Storeys) {
		if f := b.Storey(s); f.Sunk() && f.Width == 0 {
			return true
		}
	}
	return false
}

// vacant are the storeys of c a structure may take, by bit: those no floor is built on, the
// first left to what is raised
func vacant(g Ground, c cell.ID) uint8 {
	b := g.Built(c)
	free := uint8(0)
	for s := uint8(1); s < cell.Storeys; s++ {
		if *b.Storey(s) == (cell.Floor{}) {
			free |= 1 << s
		}
	}
	return free
}

func first(free uint8) uint8 {
	for s := range uint8(cell.Storeys) {
		if free&(1<<s) != 0 {
			return s
		}
	}
	return 0
}

// line is the cells strictly between a and b, which lie along one of the grid's ways
func line(g grid.Grid, a, b cell.ID) ([]cell.ID, error) {
	if a == b {
		return nil, refusal(g, a, "turns on the cell it stands on")
	}
	for way := range g.Directions() {
		var between []cell.ID
		at := a
		for range g.CellCount() {
			n, ok := g.Toward(at, way)
			if !ok || n == a {
				break
			}
			if n == b {
				return between, nil
			}
			between = append(between, n)
			at = n
		}
	}
	return nil, refusal(g, b, "lies off every line from the turn before it")
}

// a square cell's top is folded along one of its diagonals, its middle halfway along the lower;
// a hex's middle lies at the mean of its corners
func middle(heights []float32) float32 {
	if len(heights) == 4 {
		return min((heights[0]+heights[2])/2, (heights[1]+heights[3])/2)
	}
	return mean(heights)
}

func mean(heights []float32) float32 {
	sum := float32(0)
	for _, h := range heights {
		sum += h
	}
	return sum / float32(len(heights))
}

// Refusal is why a structure may not lie where it was planned, of the cell At.
type Refusal struct {
	At   cell.ID
	why  string
	args []any
	g    grid.Grid
}

// Why is the rule broken.
func (r *Refusal) Why() string { return fmt.Sprintf(r.why, r.args...) }

func (r *Refusal) Error() string {
	x, y, _ := r.g.Coords(r.At)
	return fmt.Sprintf("the structure at [%d, %d] %s", x, y, r.Why())
}

func refusal(g grid.Grid, c cell.ID, why string, args ...any) *Refusal {
	return &Refusal{At: c, why: why, args: args, g: g}
}
