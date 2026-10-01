package navigation

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// placeRig is a 7 x 5 field of 32-unit cells, land everywhere unless a test says otherwise, kept
// apart by boxes over a ground as heights says.
type placeRig struct {
	grid    grid.Grid
	terrain *cell.TerrainMap
	keep    *bodyKeeping
	heights func(p geom.Vec) float64
}

func newPlaceRig() *placeRig {
	r := &placeRig{grid: grid.DefaultGrids{}.Square(7, 5, 32), terrain: cell.NewTerrainMap()}
	r.terrain.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	r.keep = newBodyKeeping(newPathFinder(r.grid, r.terrain, nil, openOccupancy{}), nil, func() ground.Heights {
		if r.heights == nil {
			return nil
		}
		return groundFunc(r.heights)
	})
	r.keep.begin(func(b []body) []body { return b })
	return r
}

// groundFunc is a ground.Heights read off a function.
type groundFunc func(p geom.Vec) float64

func (g groundFunc) At(p geom.Vec) float64 { return g(p) }
func (g groundFunc) Step() float64         { return 1 }

func (r *placeRig) at(x, y uint32) cell.ID { c, _ := r.grid.CellIndex(x, y); return c }

// units is n land units side a side, the i-th standing in the cell (0, i % 5), 20 each nearer the
// point than the one before.
func (r *placeRig) units(n int, side float64) []member {
	var out []member
	for i := range n {
		c := r.grid.CellCenter(r.at(0, uint32(i%5)))
		pos := world.Position{AABB: plane.NewAABB(geom.NewVec(c.X-side/2+float64(i%3), c.Y-side/2), side, side)}
		out = append(out, member{id: uid.UID64(i + 1), cell: r.at(0, uint32(i%5)), from: r.at(0, uint32(i%5)), domain: cell.Land, pos: pos, z: world.Z{Height: 6}})
	}
	return out
}

// checkApart fails when two of the spots' boxes of side side come nearer than touching.
func checkApart(t *testing.T, got []placed, side float64) {
	t.Helper()
	for i := range got {
		for j := i + 1; j < len(got); j++ {
			d := geom.NewVec(got[j].spot.X-got[i].spot.X, got[j].spot.Y-got[i].spot.Y)
			if !apart(d, geom.NewVec(side/2, side/2), geom.NewVec(side/2, side/2), 0) {
				t.Errorf("spots %v and %v overlap", got[i].spot, got[j].spot)
			}
		}
	}
}

func TestPlace_TheGroupFillsTheTargetRoundThePointThenTheCheapestNeighbour(t *testing.T) {
	r := newPlaceRig()
	target := r.at(3, 2)
	for _, c := range r.grid.Neighbors(target) {
		r.terrain.Set(c, cell.Kind{Cost: 5, Allows: cell.Land})
	}
	r.terrain.Set(r.at(2, 2), cell.Kind{Cost: 1, Allows: cell.Land}) // the cheap way west
	at := r.grid.CellCenter(target)
	units := r.units(12, 6) // 12 apart with the gap: 3 x 3 in a 32 cell round its middle
	got, _ := r.keep.place(units, at, target)
	inTarget, west := 0, 0
	for i, p := range got {
		if !p.ok {
			t.Fatalf("unit %d found no spot", i)
		}
		switch p.cell {
		case target:
			inTarget++
		case r.at(2, 2):
			west++
		}
	}
	if inTarget != 9 || west != 3 {
		t.Errorf("%d stand on the target and %d on the cheap cell west, want 9 and the 3 left over", inTarget, west)
	}
	onPoint := 0
	for _, p := range got {
		if p.spot == at {
			onPoint++
		}
	}
	if onPoint != 1 {
		t.Errorf("%d units stand on the point, want one", onPoint)
	}
	checkApart(t, got, 6)
}

func TestPlace_UnitsNearlyACellLargeSpreadOverTheNeighboursAndTwoShareACellAtItsEdges(t *testing.T) {
	r := newPlaceRig()
	target := r.at(3, 2)
	centre := r.grid.CellCenter(target)
	cells := map[cell.ID]bool{}
	big, _ := r.keep.place(r.units(3, 28), centre, target)
	for _, p := range big {
		if !p.ok {
			t.Fatal("a large unit found no spot")
		}
		cells[p.cell] = true
	}
	if len(cells) != 3 {
		t.Errorf("three units nearly a cell large stand on %d cells, want one each", len(cells))
	}
	checkApart(t, big, 28)

	two, _ := r.keep.place(r.units(2, 12), geom.NewVec(centre.X-11, centre.Y), target)
	for _, p := range two {
		if p.cell != target {
			t.Fatalf("spots %v, want both on the target: a 12 box each side of the cell, over its edges", two)
		}
	}
	checkApart(t, two, 12)
}

func TestPlace_NoSpotOverGroundTheUnitCannotTakeOrOnAStep(t *testing.T) {
	r := newPlaceRig()
	target := r.at(3, 2)
	r.terrain.Set(r.at(4, 2), cell.Kind{Cost: 1, Allows: cell.Water})            // water east
	r.terrain.Set(r.at(3, 1), cell.Kind{Cost: 1, Allows: cell.Land, Height: 10}) // a rock north
	centre := r.grid.CellCenter(target)
	r.heights = func(geom.Vec) float64 { return 0 }
	corner := geom.NewVec(centre.X+14, centre.Y-14) // the north-east corner, by the water and the rock
	got, _ := r.keep.place(r.units(6, 8), corner, target)
	for _, p := range got {
		if !p.ok {
			t.Fatal("a unit found no spot")
		}
		box := boxAt(p.spot, geom.NewVec(4*0.999, 4*0.999))
		rock, off := false, false
		r.grid.CellsUnder(box, func(c cell.ID) {
			if c == r.at(4, 2) {
				t.Errorf("spot %v hangs over the water", p.spot)
			}
			rock = rock || c == r.at(3, 1)
			off = off || c != r.at(3, 1)
		})
		if rock && off {
			t.Errorf("spot %v straddles the rock's step", p.spot)
		}
	}
	checkApart(t, got, 8)
}

// Spots are planned over the ground and the group alone: one standing on the point, unknown, does
// not move the spot; once struck, it does, while a flyer struck is no bar to a walker.
func TestPlace_KnowsOnlyWhoeverItStruck(t *testing.T) {
	r := newPlaceRig()
	target := r.at(3, 2)
	centre := r.grid.CellCenter(target)
	h := geom.NewVec(5, 5)
	standing := body{id: 100, at: centre, half: h, domain: cell.Land}
	r.keep.begin(func(b []body) []body { return append(b, standing) })
	spots, _ := r.keep.place(r.units(1, 10), centre, target)
	if spots[0].spot != centre {
		t.Errorf("the unit stands at %v, want the point %v: nobody told it of the one standing there", spots[0].spot, centre)
	}
	flyer := body{id: 101, at: geom.NewVec(centre.X-20, centre.Y), half: h, domain: cell.Air}
	spots, _ = r.keep.placeClearOf(r.units(1, 10), centre, target, []body{standing, flyer})
	got := spots[0]
	if !got.ok || got.spot == centre {
		t.Fatalf("placed again clear of the one struck the unit stands at %v %v, want off the point", got.spot, got.ok)
	}
	if got.spot != geom.NewVec(centre.X-20, centre.Y) && got.spot != geom.NewVec(centre.X+20, centre.Y) && got.spot != geom.NewVec(centre.X, centre.Y-20) && got.spot != geom.NewVec(centre.X, centre.Y+20) {
		t.Errorf("the unit stands at %v, want the next lattice point round the point, the flyer's too", got.spot)
	}
}

func TestSpacing_AutoKeepsBodiesApartForUnitsAThirdOfACellOrLess(t *testing.T) {
	for _, c := range []struct {
		largest, cell float64
		want          Spacing
	}{{3, 32, BodySpacing}, {10, 32, BodySpacing}, {22, 32, CellSpacing}, {24, 48, CellSpacing}, {0, 32, CellSpacing}} {
		if got := AutoSpacing.resolve(c.largest, c.cell); got != c.want {
			t.Errorf("units %v on cells %v keep apart by %v, want %v", c.largest, c.cell, got, c.want)
		}
	}
	if got := CellSpacing.resolve(3, 32); got != CellSpacing {
		t.Errorf("asked for cells, small units keep apart by %v", got)
	}
}
