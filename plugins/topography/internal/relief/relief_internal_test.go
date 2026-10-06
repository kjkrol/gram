package relief

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	public "github.com/kjkrol/gram/plugins/topography/relief"
)

var hill = cell.Kind{Name: cell.Named("hill"), Cost: 1, Allows: cell.Land | cell.Air}

// raiseHills puts the hill cells at 12 and the rest at 0, each corner at the mean of its cells.
func raiseHills(r *Relief, grid grid.Grid, hills ...cell.ID) {
	r.SetHeights(public.MeanOfCells(grid, func(c cell.ID) float64 {
		for _, h := range hills {
			if h == c {
				return 12
			}
		}
		return 0
	}))
}

func TestRelief_GroundAtReadsTheReliefAndFollowsIt(t *testing.T) {
	for name, grid := range map[string]grid.Grid{
		"square": grid.DefaultGrids{}.Square(4, 4, 32),
		"hex":    grid.DefaultGrids{}.Hex(4, 4, 16),
	} {
		t.Run(name, func(t *testing.T) {
			brd := board.NewBoard(grid)
			r := New(brd)
			c := grid.CellIndex(2, 1)
			raiseHills(r, grid, c)

			// A lone hill on a square grid is smoothed to its corners' mean, 3; a hex cell stays level.
			want := 12.0
			if name == "square" {
				want = 3
			}
			if got := r.Altitude(c); got != want {
				t.Errorf("the hill's altitude = %v, want %v", got, want)
			}
			if got := r.GroundAt(grid.CellCenter(c)); got != want {
				t.Errorf("ground at the hill's centre = %v, want %v", got, want)
			}
			other := grid.CellIndex(0, 0)
			if got := r.GroundAt(grid.CellCenter(other)); got != 0 {
				t.Errorf("ground on the grass = %v, want 0", got)
			}
			if got := r.GroundAt(geom.NewVec(-100, -100)); got != 0 {
				t.Errorf("ground off the board = %v, want 0", got)
			}
			before, boardBefore := r.Version(), brd.Version()
			raiseHills(r, grid)
			if got := r.GroundAt(grid.CellCenter(c)); got != 0 {
				t.Errorf("ground after the hill was levelled = %v, want 0", got)
			}
			if r.Version() == before || brd.Version() == boardBefore {
				t.Error("levelling the hill left the relief's or the board's Version as it was")
			}
			if r.Step() <= 0 {
				t.Errorf("Step = %v, want the cell's shorter side", r.Step())
			}
		})
	}
}

func TestRelief_GroundSlopesBetweenCellsOnASquareGrid(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(6, 6, 32)
	r := New(board.NewBoard(grid))
	var hills []cell.ID
	for y := uint32(2); y <= 4; y++ {
		for x := uint32(2); x <= 4; x++ {
			c := grid.CellIndex(x, y)
			hills = append(hills, c)
		}
	}
	raiseHills(r, grid, hills...)
	centre := grid.CellIndex(3, 3)
	if got := r.GroundAt(grid.CellCenter(centre)); got != 12 {
		t.Errorf("the plateau's middle stands at %v, want the full 12", got)
	}
	corner := grid.CellIndex(2, 2)
	if got := r.GroundAt(grid.CellCenter(corner)); got != 6 {
		t.Errorf("the plateau's corner cell stands at %v in its middle, want 6: its corners 3, 6, 6 and 12 are drawn split along the diagonal of the two 6s", got)
	}
	if got := r.Altitude(corner); got != 6.75 {
		t.Errorf("the plateau's corner cell's level is %v, want 6.75, the mean of its corners", got)
	}
	last := -1.0
	for x := 40.0; x <= 112; x += 8 { // walking east along row 3 up onto the plateau
		if got := r.GroundAt(geom.NewVec(x, 112)); got < last {
			t.Errorf("the ground drops from %v to %v at x %v on the way up the slope", last, got, x)
		} else {
			last = got
		}
	}
	if hs := r.Corners(centre); hs != (Corners{12, 12, 12, 12}) {
		t.Errorf("Corners of the middle = %v, want four 12s", hs)
	}
}

// A wrapping board's lattice folds: the corners along the seam are one.
func TestRelief_FoldsWhereTheBoardWraps(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	grid.(interface{ SetWrap(x, y bool) }).SetWrap(true, false)
	r := New(board.NewBoard(grid))
	west := grid.CellIndex(0, 1)
	east := grid.CellIndex(3, 1)
	r.SetCorners(west, Corners{5, 0, 5, 0})
	if got := r.Corners(east); got != (Corners{0, 5, 0, 5}) {
		t.Errorf("across the seam the east cell's corners are %v, want its right ones the west cell's left, 5", got)
	}
	if got := r.GroundAt(geom.NewVec(127.9, 48)); got < 4.9 {
		t.Errorf("the ground at the seam stands at %v, want about 5", got)
	}
}

// A run of heights goes through a save as it is, its size fixed.
func TestHeights_ARunGoesThroughASave(t *testing.T) {
	in := Heights{First: 1024, Count: 3}
	in.Values[0], in.Values[2] = -4.5, 900
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(in); err != nil {
		t.Fatal(err)
	}
	var out Heights
	if err := gob.NewDecoder(&buf).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("a run came back as from %d, %d of them, %v...; want it as it went", out.First, out.Count, out.Values[:3])
	}
}

// relief.Heights over more corners than a run holds go over several, from one another's end, all of
// them; the ground takes them back only when they are all there.
func TestRelief_CutsItsHeightsIntoRunsAndTakesThemBack(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(40, 40, 32)
	r := New(board.NewBoard(grid))
	c := grid.CellIndex(39, 39)
	r.SetCorners(c, Corners{1, 2, 3, 4})
	runs := make([]Heights, r.Runs())
	for i := range runs {
		runs[i].First = uint32(i * HeightsRun)
		r.fill(&runs[i])
	}
	if len(runs) != 2 || runs[0].Count != HeightsRun || runs[1].Count != 41*41-HeightsRun {
		t.Fatalf("%d runs of %d and %d, want two: 1024 and the other %d of 1681 corners", len(runs), runs[0].Count, runs[1].Count, 41*41-HeightsRun)
	}
	back := New(board.NewBoard(grid))
	if back.adopt(runs[:1]) {
		t.Error("the ground took back one run of two")
	}
	if !back.adopt(runs) {
		t.Fatal("the ground did not take back both runs")
	}
	if got := back.Corners(c); got != (Corners{1, 2, 3, 4}) {
		t.Errorf("the last cell's corners came back as %v, want 1 to 4", got)
	}
}

// The ground under a point is the ground drawn: the cell's top split into two flat triangles along
// the diagonal whose corners stand nearer in height, as render.Frame.Fold draws it.
func TestRelief_TheGroundIsTheGroundDrawn(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(2, 2, 32)
	r := New(board.NewBoard(grid))
	c := grid.CellIndex(0, 0)
	// corners 0, 20, 24 and 2: the 0 and the 2 stand nearer, so the top is split along 0–3
	r.SetCorners(c, Corners{0, 20, 24, 2})
	for _, p := range []struct{ x, y, want float64 }{
		{16, 16, 1},    // the middle, on the diagonal of the 0 and the 2
		{24, 8, 10.5},  // on the triangle with the 20
		{8, 24, 12.5},  // on the triangle with the 24
		{28, 4, 15.25}, // near the 20
	} {
		if got := r.GroundAt(geom.NewVec(p.x, p.y)); math.Abs(got-p.want) > 1e-9 {
			t.Errorf("the ground at (%v, %v) is %v, want %v on the triangles drawn", p.x, p.y, got, p.want)
		}
	}
	// corners 10, 0, 0, 20: split along the two 0s, 1–2; the middle is 0, not the mean 7.5
	r.SetCorners(c, Corners{10, 0, 0, 20})
	if got := r.GroundAt(geom.NewVec(16, 16)); got != 0 {
		t.Errorf("the ground in the middle is %v, want 0 on the diagonal of the 0s", got)
	}
	if got := r.GroundAt(geom.NewVec(4, 4)); math.Abs(got-7.5) > 1e-9 {
		t.Errorf("the ground near the 10 is %v, want 7.5", got)
	}
}
