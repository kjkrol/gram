package mosaic

import (
	"math"
	"os"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
)

const testSide = 384

// testMap is the level in testdata: a crossing in the middle of hills, a road leaving it askew
func testMap(t *testing.T) Map {
	t.Helper()
	m, err := Load(os.DirFS("../board/mapfile/testdata"), "level/level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// uncovered is how many of the ground's places, one every half a metre, lie under no triangle
// of the sectors' surfaces, and the first of them
func uncovered(g *ground, sector, side float32) (int, geom.Vec) {
	const every = 0.5
	n := int(side / every)
	covered := make([]bool, n*n)
	for sx := float32(0); sx < side; sx += sector {
		for sy := float32(0); sy < side; sy += sector {
			box := geom.NewAABB(geom.NewVec(sx, sy), geom.NewVec(sx+sector, sy+sector))
			tris := g.surface(box)
			for t := 0; t+triangleFloats <= len(tris); t += triangleFloats {
				ax, ay := float64(tris[t]), float64(tris[t+1])
				bx, by := float64(tris[t+4]), float64(tris[t+5])
				cx, cy := float64(tris[t+8]), float64(tris[t+9])
				x0 := int(math.Floor(min(ax, bx, cx) / every))
				x1 := int(math.Ceil(max(ax, bx, cx) / every))
				y0 := int(math.Floor(min(ay, by, cy) / every))
				y1 := int(math.Ceil(max(ay, by, cy) / every))
				for y := max(y0, 0); y <= min(y1, n-1); y++ {
					for x := max(x0, 0); x <= min(x1, n-1); x++ {
						px, py := (float64(x)+0.5)*every, (float64(y)+0.5)*every
						// within a centimetre of the triangle: the cells' corners are kept to a
						// millimetre
						off := func(ux, uy, vx, vy float64) float64 {
							return ((px-vx)*(uy-vy) - (ux-vx)*(py-vy)) / math.Hypot(ux-vx, uy-vy)
						}
						d1, d2, d3 := off(ax, ay, bx, by), off(bx, by, cx, cy), off(cx, cy, ax, ay)
						const eps = 0.01
						neg := d1 < -eps || d2 < -eps || d3 < -eps
						pos := d1 > eps || d2 > eps || d3 > eps
						if !(neg && pos) {
							covered[y*n+x] = true
						}
					}
				}
			}
		}
	}
	holes, first := 0, geom.Vec{}
	for i, c := range covered {
		if !c {
			if holes == 0 {
				first = geom.NewVec((float32(i%n)+0.5)*every, (float32(i/n)+0.5)*every)
			}
			holes++
		}
	}
	return holes, first
}

// testMapCut is the same level cut into tiles 128 m a side and read back
func testMapCut(t *testing.T) Map {
	t.Helper()
	dir := t.TempDir()
	if err := Cut(os.DirFS("../board/mapfile/testdata"), "level/level.yaml", dir, Cutting{Tile: 128}); err != nil {
		t.Fatal(err)
	}
	m, err := Load(os.DirFS(dir), "level.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestTheGroundIsWholeAtEveryTolerance(t *testing.T) {
	for name, m := range map[string]Map{"in one piece": testMap(t), "in tiles": testMapCut(t)} {
		g := newGround(m, testSide, testSide, Config{}.filled())
		edge := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(testSide, testSide))
		for _, tol := range []float32{4, 2, 0.25, 0} {
			g.plane.Show(edge, tol)
			g.plane.Update(math.MaxInt)
			if holes, at := uncovered(g, 64, testSide); holes > 0 {
				t.Errorf("%s, at tolerance %v the ground has %d holes, the first at %v",
					name, tol, holes, at)
			}
		}
		// and where sectors as fine as can be lie beside sectors as coarse
		g.plane.Show(edge, 4)
		g.plane.Show(geom.NewAABB(geom.NewVec(128, 128), geom.NewVec(256, 256)), 0)
		g.plane.Update(math.MaxInt)
		if holes, at := uncovered(g, 64, testSide); holes > 0 {
			t.Errorf("%s, with a fine middle the ground has %d holes, the first at %v",
				name, holes, at)
		}
		if len(m.Roads) != 4 || len(m.Junctions) != 1 {
			t.Errorf("%s: %d roads and %d junctions, want 4 and 1", name, len(m.Roads),
				len(m.Junctions))
		}
	}
}
