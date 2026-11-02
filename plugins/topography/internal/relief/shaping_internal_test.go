package relief

import (
	"slices"
	"testing"

	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/grid"
)

func TestRelief_GradePutsASquareCellsCornersInItsOutlinesOrder(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	r := onBoard(g)
	c := g.At(1, 1)
	heights := []float32{1, 2, 3, 4}
	version := r.Version()
	if !r.Grade(c, heights, 0) || r.Version() == version {
		t.Fatal("grading level ground reports it unmoved or counts no change")
	}
	// the outline runs round the cell, Corners lists its bottom ones left to right
	if got := r.Corners(c); got != (Corners{1, 2, 4, 3}) {
		t.Errorf("the cell's Corners %v, want {1, 2, 4, 3}", got)
	}
	for k, p := range g.CellOutline(c, nil) {
		if got := r.GroundAt(p); got != heights[k] {
			t.Errorf("the ground at the outline's corner %d is %v, want %v", k, got, heights[k])
		}
	}
	for _, n := range []struct {
		x, y uint32
		want Corners
	}{
		{2, 1, Corners{2, 0, 3, 0}},
		{1, 2, Corners{4, 3, 0, 0}},
		{2, 2, Corners{3, 0, 0, 0}},
		{0, 0, Corners{0, 0, 0, 1}},
	} {
		if got := r.Corners(g.At(n.x, n.y)); got != n.want {
			t.Errorf("the neighbour (%d, %d) stands at %v, want %v, the cell's where they meet",
				n.x, n.y, got, n.want)
		}
	}
	moved := 0
	for _, h := range r.values {
		if h != 0 {
			moved++
		}
	}
	if moved != 4 {
		t.Errorf("%d corners moved with a MaxStep of 0, want the cell's four", moved)
	}
}

func TestRelief_GradeAsTheCornersStandMovesNothing(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	r := onBoard(g)
	c := g.At(1, 1)
	heights := []float32{5, 6, 7, 8}
	r.Grade(c, heights, 2)
	before := slices.Clone(r.values)
	version := r.Version()
	if r.Grade(c, heights, 2) {
		t.Error("graded again as it stands, the ground reports it moved")
	}
	if !slices.Equal(r.values, before) || r.Version() != version {
		t.Error("graded again as it stands, the ground moved")
	}
}

func TestRelief_GradePanicsForHeightsOfAnotherCount(t *testing.T) {
	g := grid.DefaultGrids{}.Square(4, 4, 32)
	r := onBoard(g)
	defer func() {
		if recover() == nil {
			t.Error("a square cell graded to six heights did not panic")
		}
	}()
	r.Grade(g.At(1, 1), []float32{1, 2, 3, 4, 5, 6}, 0)
}

func TestRelief_TheGroundRoundAGradedCellFollowsWithinMaxStep(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 32)
	c := g.At(2, 2)
	r := onBoard(g)
	r.Grade(c, []float32{8, 8, 8, 8}, 2)
	// the cell's corners are the lattice's (2..3, 2..3); each corner farther off stands 2 lower
	for y := range 7 {
		for x := range 7 {
			far := max(0, 2-x, x-3) + max(0, 2-y, y-3)
			want := float32(max(0, 8-2*far))
			if got := r.heightAt(vertex{x, y}); got != want {
				t.Errorf("the corner (%d, %d) stands at %v, want %v", x, y, got, want)
			}
		}
	}

	r = onBoard(g)
	r.Grade(c, []float32{8, 8, 8, 8}, 0)
	for y := range 7 {
		for x := range 7 {
			want := float32(0)
			if x >= 2 && x <= 3 && y >= 2 && y <= 3 {
				want = 8
			}
			if got := r.heightAt(vertex{x, y}); got != want {
				t.Errorf("with a MaxStep of 0 the corner (%d, %d) stands at %v, want %v", x, y,
					got, want)
			}
		}
	}
}

// a ramp steeper than MaxStep along the cell's own edges, its low corners in its high ones' ripple
func TestRelief_AGradedCellsCornersStayWhereTheyArePut(t *testing.T) {
	g := grid.DefaultGrids{}.Square(6, 6, 32)
	r := onBoard(g)
	c := g.At(2, 2)
	if !r.Grade(c, []float32{0, 4, 4, 0}, 1) {
		t.Fatal("grading a ramp into level ground reports it unmoved")
	}
	if got := r.Corners(c); got != (Corners{0, 4, 0, 4}) {
		t.Errorf("the ramp's corners stand at %v, want 0 on the west and 4 on the east", got)
	}
	for _, v := range []struct {
		x, y int
		want float32
	}{
		{4, 2, 3},
		{5, 2, 2},
		{6, 2, 1},
		{3, 1, 3},
		{2, 1, 2},
		{1, 1, 1},
		{1, 2, 0},
	} {
		if got := r.heightAt(vertex{v.x, v.y}); got != v.want {
			t.Errorf("the corner (%d, %d) stands at %v, want %v", v.x, v.y, got, v.want)
		}
	}
	vs, n, _ := r.outline(c)
	own := vs[:n]
	for y := range 7 {
		for x := range 7 {
			a := vertex{x, y}
			for _, b := range []vertex{{x + 1, y}, {x, y + 1}} {
				if b.x > 6 || b.y > 6 || slices.Contains(own, a) || slices.Contains(own, b) {
					continue
				}
				if d := f32.Abs(r.heightAt(a) - r.heightAt(b)); d > 1+shapeTolerance {
					t.Errorf("the corners %v and %v differ by %v along their edge, more than 1",
						a, b, d)
				}
			}
		}
	}
}

func TestRelief_GradeAllocatesNothingWithoutARipple(t *testing.T) {
	for name, g := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(4, 4, 32),
		"hex":    grid.DefaultGrids{}.Hex(4, 4, 16),
	} {
		r := New(g, nil)
		c := g.At(1, 1)
		up := make([]float32, len(g.CellOutline(c, nil)))
		down := make([]float32, len(up))
		for k := range up {
			up[k] = float32(k + 1)
		}
		flip := false
		grade := func() {
			flip = !flip
			if flip {
				r.Grade(c, up, 0)
			} else {
				r.Grade(c, down, 0)
			}
		}
		if n := testing.AllocsPerRun(100, grade); n != 0 {
			t.Errorf("%s: Grade allocates %v a call, want none", name, n)
		}
	}
}
