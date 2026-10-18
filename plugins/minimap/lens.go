package minimap

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
)

// lens is a minimap's camera: straight down on a point of the world, turned so that ahead runs up
// the picture, scale pixels a world unit; on a torus the way to a point is the shorter one round.
type lens struct {
	world        geom.Vec
	wrapX, wrapY bool
	reach        float64
	w, h         float32
	scale        float32
	// the middle of the picture on the world, and the way up the picture, a unit vector
	cx, cy float32
	ax, ay float32
}

var _ camera.Camera = (*lens)(nil)
var _ camera.Focused = (*lens)(nil)

func newLens(world geom.Vec, wrapX, wrapY bool, reach float64) *lens {
	return &lens{world: world, wrapX: wrapX, wrapY: wrapY, reach: reach, ay: -1}
}

// aim puts the picture's middle at (x, y), ahead (ax, ay) running up it
func (l *lens) aim(x, y, ax, ay float32) {
	l.cx, l.cy = x, y
	n := float32(math.Hypot(float64(ax), float64(ay)))
	if n == 0 {
		ax, ay, n = 0, -1, 1
	}
	l.ax, l.ay = ax/n, ay/n
}

func (l *lens) fit() {
	l.scale = 0
	if l.reach > 0 {
		l.scale = min(l.w, l.h) / float32(2*l.reach)
	}
}

// the shorter way from a to b along an axis of length size, wrapping as wraps says
func along(a, b float32, size float64, wraps bool) float32 {
	d := b - a
	if !wraps {
		return d
	}
	s := float32(size)
	d -= s * float32(math.Round(float64(d/s)))
	return d
}

func (l *lens) Project(x, y, _ float32) (float32, float32) {
	dx := along(l.cx, x, l.world.X, l.wrapX)
	dy := along(l.cy, y, l.world.Y, l.wrapY)
	// right is ahead turned a quarter clockwise on the screen: (-ay, ax)
	right := dx*-l.ay + dy*l.ax
	up := dx*l.ax + dy*l.ay
	return l.w/2 + right*l.scale, l.h/2 - up*l.scale
}

func (l *lens) Unproject(sx, sy, _ float32) (float32, float32) {
	if l.scale == 0 {
		return l.cx, l.cy
	}
	right := (sx - l.w/2) / l.scale
	up := (l.h/2 - sy) / l.scale
	return l.cx + right*-l.ay + up*l.ax, l.cy + right*l.ax + up*l.ay
}

func (l *lens) Projection() camera.Projection            { return camera.TopDown{} }
func (l *lens) Depth(_, y, _ float32) float32            { return y }
func (l *lens) Viewport() (float32, float32)             { return l.w, l.h }
func (l *lens) ToScreen(x, y float32) (float32, float32) { return l.Project(x, y, 0) }

func (l *lens) FromScreen(sx, sy float32) (float32, float32) { return l.Unproject(sx, sy, 0) }

func (l *lens) SetViewport(w, h float32) {
	l.w, l.h = w, h
	l.fit()
}

// ToScreenQuads appends the screen rectangle round the world rectangle's corners.
func (l *lens) ToScreenQuads(x0, y0, x1, y1 float32, dst []camera.Quad) []camera.Quad {
	inf := float32(math.Inf(1))
	q := camera.Quad{X0: inf, Y0: inf, X1: -inf, Y1: -inf, T1X: 1, T1Y: 1}
	for _, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		sx, sy := l.Project(p[0], p[1], 0)
		q.X0 = min(q.X0, sx)
		q.Y0 = min(q.Y0, sy)
		q.X1 = max(q.X1, sx)
		q.Y1 = max(q.Y1, sy)
	}
	return append(dst, q)
}

// Bounds is the square round the picture, turned as it may be: on a torus it may run past the
// world's own size.
func (l *lens) Bounds() camera.AABB {
	r := l.radius()
	return geom.NewAABB(geom.NewVec(float64(l.cx)-r, float64(l.cy)-r),
		geom.NewVec(float64(l.cx)+r, float64(l.cy)+r))
}

// covered is the world rectangle round the picture's corners: as wide as the picture north up,
// wider turned
func (l *lens) covered() camera.AABB {
	inf := math.Inf(1)
	lo, hi := geom.NewVec(inf, inf), geom.NewVec(-inf, -inf)
	for _, c := range [4][2]float32{{0, 0}, {l.w, 0}, {0, l.h}, {l.w, l.h}} {
		x, y := l.Unproject(c[0], c[1], 0)
		lo = geom.NewVec(min(lo.X, float64(x)), min(lo.Y, float64(y)))
		hi = geom.NewVec(max(hi.X, float64(x)), max(hi.Y, float64(y)))
	}
	return geom.NewAABB(lo, hi)
}

// half the picture's diagonal, in world units
func (l *lens) radius() float64 {
	if l.scale == 0 {
		return 0
	}
	return math.Hypot(float64(l.w), float64(l.h)) / 2 / float64(l.scale)
}

func (l *lens) Visible(box camera.AABB) bool {
	r := l.radius()
	hw := (box.BottomRight.X - box.TopLeft.X) / 2
	hh := (box.BottomRight.Y - box.TopLeft.Y) / 2
	dx := along(l.cx, float32(box.TopLeft.X+hw), l.world.X, l.wrapX)
	dy := along(l.cy, float32(box.TopLeft.Y+hh), l.world.Y, l.wrapY)
	return math.Abs(float64(dx)) < r+hw && math.Abs(float64(dy)) < r+hh
}

func (l *lens) Focus() (float32, float32, float32) { return l.cx, l.cy, 0 }

// The rest is a camera's control: a minimap's follows another's alone.

func (l *lens) MoveTo(x, y float64)               {}
func (l *lens) CenterOn(x, y, _ float64)          { l.cx, l.cy = float32(x), float32(y) }
func (l *lens) Translate(dx, dy float64)          {}
func (l *lens) Pan(dx, dy float32)                {}
func (l *lens) Zoom() float32                     { return l.scale }
func (l *lens) ZoomIn(float32, float32, float32)  {}
func (l *lens) ZoomOut(float32, float32, float32) {}
func (l *lens) State() camera.State {
	return camera.State{Viewport: l.Bounds(), Zoom: l.scale}
}
func (l *lens) Persisted() []any   { return nil }
func (l *lens) Restore()           {}
func (l *lens) SetMinZoom(float32) {}
func (l *lens) SetMaxZoom(float32) {}
