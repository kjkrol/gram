package topography

import (
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/topography/internal/terrain"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// the care's bands over the ground: world units over it and a line's width in pixels
const (
	careLift  = 0.5
	careWidth = 2
)

// a colour a care, the least none: orange, yellow, yellow-green and green for the full
var careColors = [terrain.FullCare + 1]color.RGBA{
	{},
	{235, 140, 60, 255},
	{235, 200, 60, 255},
	{160, 210, 70, 255},
	{70, 200, 90, 255},
}

// careView lays the care's bands over the ground: the boundaries between sectors of different
// care, each in the colour of the greater.
type careView struct{ p *Plugin }

var _ render.Source = (*careView)(nil)

func (v *careView) Compose(f *render.Frame, cam camera.Camera) {
	p := v.p
	if s := p.drawnSettings(); !s.ShowCare || p.ground == nil || s.Care <= 0 {
		return
	}
	cols, rows, cell, _, ok := p.drawnRelief().Lattice()
	if !ok || cell <= 0 {
		return
	}
	seen := p.worldPlugin.Read(0).ViewFor(cam)
	g := seen.Grid()
	if g == nil {
		return
	}
	care := p.ground.CareOf(cam)
	n, across := g.Len(), g.Cols()
	lines := careLines{f: f, cam: cam, p: p, cell: cell}
	// the lattice's span: a sector past the board has no ground to lie on
	wide, high := float32(cols-1)*cell, float32(rows-1)*cell
	seen.Sectors(func(i int, box geom.AABB, _ view.Level) {
		c := care(i)
		if c == terrain.LeastCare {
			return
		}
		colour := careColors[c]
		x0, y0 := min(box.TopLeft.X, wide), min(box.TopLeft.Y, high)
		x1, y1 := min(box.BottomRight.X, wide), min(box.BottomRight.Y, high)
		if x1 <= x0 || y1 <= y0 {
			return
		}
		col := i % across
		if col > 0 && care(i-1) < c {
			lines.draw(x0, y0, x0, y1, colour)
		}
		if col < across-1 && care(i+1) < c {
			lines.draw(x1, y0, x1, y1, colour)
		}
		if i >= across && care(i-across) < c {
			lines.draw(x0, y0, x1, y0, colour)
		}
		if i+across < n && care(i+across) < c {
			lines.draw(x0, y1, x1, y1, colour)
		}
	})
}

// a frame's care lines through a camera, over the relief's lattice of corners cell apart
type careLines struct {
	f    *render.Frame
	cam  camera.Camera
	p    *Plugin
	cell float32
}

// draw lays the boundary from (ax, ay) to (bx, by), along one axis, as segments between the
// lattice's lines it crosses, each lifted over the ground
func (l careLines) draw(ax, ay, bx, by float32, c color.RGBA) {
	vertical := ax == bx
	from, to := ax, bx
	if vertical {
		from, to = ay, by
	}
	point := func(t float32) (x, y float32) {
		if vertical {
			return ax, t
		}
		return t, ay
	}
	x, y := point(from)
	sx, sy, z := l.at(x, y)
	for k := int(f32.Floor(from / l.cell)); from < to; k++ {
		next := min(float32(k+1)*l.cell, to)
		nx, ny := point(next)
		sx1, sy1, z1 := l.at(nx, ny)
		at := l.f.At((x+nx)/2, (y+ny)/2, (z+z1)/2)
		l.f.Edge(at, sx, sy, z, sx1, sy1, z1, careWidth, c)
		from, x, y, sx, sy, z = next, nx, ny, sx1, sy1, z1
	}
}

// at is the screen point and the height of the care line over the ground at (x, y)
func (l careLines) at(x, y float32) (sx, sy, z float32) {
	z = l.p.drawnRelief().GroundAt(geom.NewVec(x, y)) + careLift
	sx, sy = l.cam.Project(x, y, z)
	return sx, sy, z
}
