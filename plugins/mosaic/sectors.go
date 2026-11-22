package mosaic

import (
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/mosaic/plane"
)

// ground is the plane and how each of the world's sectors shows it: at what tolerance, and
// whether its triangles wait to be laid anew
type ground struct {
	plane     *plane.Plane
	cfg       Config
	width     float32
	height    float32
	low, high float32
	look      Look
	sectors   []sector
	cols      int
	side      float32
	waiting   []int
	triangles int
	settled   bool
	// the sectors waiting to be laid anew, and whether some laid wait for the rest
	stale  int
	staged bool
}

type sector struct {
	box       geom.AABB
	tolerance float32
	wanted    float32
	shown     bool
	stale     bool
	triangles int
}

// floats a triangle: its three corners, a colour's channel riding with each, and the way up
// off the ground at each
const triangleFloats = 24

func newGround(m Map, width, height float32, cfg Config) *ground {
	g := &ground{plane: build(m, width, height), cfg: cfg, width: width, height: height,
		look: DefaultLook}
	g.low, g.high = float32(math.Inf(1)), float32(math.Inf(-1))
	if m.Ground == nil {
		g.low, g.high = 0, 0
	} else {
		for x := float32(0); x <= width; x += 8 {
			for y := float32(0); y <= height; y += 8 {
				h := m.Ground(geom.NewVec(x, y))
				g.low, g.high = min(g.low, h), max(g.high, h)
			}
		}
	}
	return g
}

func (g *ground) paint(look Look) {
	g.look = look
	for i := range g.sectors {
		g.sectors[i].stale = true
	}
	g.stale = len(g.sectors)
}

// lay cuts the ground into the world's sectors, once they are known
func (g *ground) lay(r *world.Read) bool {
	if g.sectors != nil {
		return true
	}
	n, side := r.SectorsLen(), r.SectorSide()
	if n == 0 || side <= 0 {
		return false
	}
	g.side, g.cols = side, int(math.Ceil(float64(g.width/side)))
	g.sectors = make([]sector, n)
	for i := range g.sectors {
		x, y := float32(i%g.cols)*side, float32(i/g.cols)*side
		g.sectors[i] = sector{
			box: geom.NewAABB(geom.NewVec(x, y),
				geom.NewVec(min(x+side, g.width), min(y+side, g.height))),
		}
	}
	return true
}

// step follows the sectors' levels a batch at a time, so the picture is ever of one state of
// the plane: shows a few sectors at their levels' tolerances, works out the cells that changes
// within the frame's share, then lays anew the triangles of every sector whose ground changed,
// a few a frame, calling laid with each one's and swap once the batch is whole.
func (g *ground) step(r *world.Read, laid func(i int, triangles []float32), swap func()) {
	if !g.lay(r) {
		return
	}
	// the first time everything, before a frame is shown
	shown, cells, triangles := g.cfg.Shown, g.cfg.Cells, g.cfg.Triangles
	first := !g.settled
	if first {
		g.settled = true
		shown, cells, triangles = len(g.sectors), math.MaxInt, math.MaxInt
		for i := range g.sectors {
			g.sectors[i].stale = true
		}
		g.stale = len(g.sectors)
	}
	if first || !g.plane.Pending() && g.stale == 0 && !g.staged {
		g.waiting = g.waiting[:0]
		for i := range g.sectors {
			s := &g.sectors[i]
			mid := geom.NewVec((s.box.TopLeft.X+s.box.BottomRight.X)/2,
				(s.box.TopLeft.Y+s.box.BottomRight.Y)/2)
			s.wanted = g.cfg.Tolerances[min(r.LevelAt(mid.X, mid.Y), lod.Omitted)]
			if !s.shown || s.wanted != s.tolerance {
				g.waiting = append(g.waiting, i)
			}
		}
		// those wanted finest first
		slices.SortFunc(g.waiting, func(a, b int) int {
			switch wa, wb := g.sectors[a].wanted, g.sectors[b].wanted; {
			case wa < wb:
				return -1
			case wa > wb:
				return 1
			}
			return a - b
		})
		for _, i := range g.waiting[:min(len(g.waiting), shown)] {
			s := &g.sectors[i]
			s.tolerance, s.shown = s.wanted, true
			g.plane.Show(s.box, s.tolerance)
		}
	}
	ch := g.plane.Update(cells)
	for _, box := range ch.Redrawn {
		x0, x1 := g.colOf(box.TopLeft.X), g.colOf(box.BottomRight.X)
		y0, y1 := g.colOf(box.TopLeft.Y), g.colOf(box.BottomRight.Y)
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if i := y*g.cols + x; i < len(g.sectors) && !g.sectors[i].stale {
					g.sectors[i].stale = true
					g.stale++
				}
			}
		}
	}
	// triangles of a plane still changing would be of a state never whole
	if g.plane.Pending() {
		return
	}
	for i := range g.sectors {
		if triangles <= 0 || g.stale == 0 {
			break
		}
		s := &g.sectors[i]
		if !s.stale {
			continue
		}
		s.stale = false
		g.stale--
		tris := g.surface(s.box)
		triangles -= len(tris) / triangleFloats
		g.triangles += len(tris)/triangleFloats - s.triangles
		s.triangles = len(tris) / triangleFloats
		laid(i, tris)
		g.staged = true
	}
	if g.stale == 0 && g.staged {
		g.staged = false
		swap()
	}
}

func (g *ground) colOf(x float32) int {
	return min(max(int(x/g.side), 0), g.cols-1)
}

// surface is the ground over a box as the renderer takes it
func (g *ground) surface(box geom.AABB) []float32 {
	m := g.plane.Surface(box, nil)
	out := make([]float32, 0, len(m.Tags)*triangleFloats)
	for t, tag := range m.Tags {
		c := g.look.Colors[kindOf(tag)]
		rgb := [3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
		for v := 0; v < 3; v++ {
			p := m.Positions[9*t+3*v:]
			out = append(out, p[0], p[1], p[2], rgb[v])
		}
		for v := 0; v < 3; v++ {
			n := m.Normals[9*t+3*v:]
			out = append(out, n[0], n[1], n[2], 0)
		}
	}
	return out
}
