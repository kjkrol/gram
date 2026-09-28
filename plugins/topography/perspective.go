package topography

import (
	"math"

	"github.com/kjkrol/gram/camera"
)

var _ camera.Projection = perspective{}

// perspective is the world seen from an eye at a point of it: a point lands on the screen where
// the line from it to the eye crosses the picture plane focal screen units before the eye, so what
// is further off is smaller. right, up and forward are the eye's frame — right along the screen,
// up up it, forward the way it looks — and the projected origin is where forward meets the screen.
// A point nearer than near is drawn as if at near; a screen point looking past the ground is put
// far along its line. cell is the world size of a cell, for Depth.
type perspective struct {
	eye, right, up, forward [3]float32
	focal, near, far, cell  float32
}

// newPerspective is the perspective of an eye at eye looking at target, the screen turned by
// heading as the isometric view turns it: 0 has the world's x run down-right and y down-left.
func newPerspective(eye, target [3]float32, heading, focal, near, far, cell float32) perspective {
	p := perspective{eye: eye, focal: focal, near: near, far: far, cell: cell}
	f := sub(target, eye)
	if n := norm(f); n > 0 {
		f = scale(f, 1/n)
	} else {
		f = [3]float32{0, 0, -1}
	}
	s, c := math.Sincos(float64(heading) + math.Pi/4)
	r := [3]float32{float32(c), -float32(s), 0}
	r = sub(r, scale(f, dot(r, f))) // square to the way the eye looks, whatever heading says
	if n := norm(r); n > 0 {
		r = scale(r, 1/n)
	} else {
		r = [3]float32{1, 0, 0}
	}
	p.forward, p.right, p.up = f, r, cross(f, r)
	return p
}

// view is the point (x, y, z) in the eye's frame: across, up and ahead of it.
func (p perspective) view(x, y, z float32) (across, up, ahead float32) {
	d := [3]float32{x - p.eye[0], y - p.eye[1], z - p.eye[2]}
	return dot(d, p.right), dot(d, p.up), dot(d, p.forward)
}

func (p perspective) Project(x, y, z float32) (float32, float32) {
	across, up, ahead := p.view(x, y, z)
	if ahead < p.near { // behind the eye, or all but: far off the screen the way it lies
		n := float32(math.Hypot(float64(across), float64(up)))
		if n < 1e-3 {
			across, up, n = 0, 1, 1
		}
		k := p.far * p.focal / n
		return across * k, -up * k
	}
	return p.focal * across / ahead, -p.focal * up / ahead
}

func (p perspective) Unproject(sx, sy, z float32) (float32, float32) {
	x, y, _ := p.cast(sx, sy, z)
	return x, y
}

// cast is the world point at height z seen at the screen point (sx, sy), and whether the line from
// the eye through it meets that height ahead of the eye at all; where it does not — the sky, the
// ground behind the eye — the point far along the line.
func (p perspective) cast(sx, sy, z float32) (x, y float32, hit bool) {
	d := p.ray(sx, sy)
	t := (z - p.eye[2]) / d[2]
	if d[2] == 0 || t <= 0 || t != t {
		t = p.far
	} else {
		hit = true
	}
	return p.eye[0] + d[0]*t, p.eye[1] + d[1]*t, hit
}

// ray is the way from the eye through the screen point (sx, sy): forward, a focal length off, and
// across and up by the point.
func (p perspective) ray(sx, sy float32) [3]float32 {
	return add(p.forward, add(scale(p.right, sx/p.focal), scale(p.up, -sy/p.focal)))
}

// Vanish is where the direction (dx, dy, dz) vanishes on the screen — where everything far along
// it is drawn — and false for a direction not ahead of the eye.
func (p perspective) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	d := [3]float32{dx, dy, dz}
	ahead := dot(d, p.forward)
	if ahead <= 1e-6 {
		return 0, 0, false
	}
	return p.focal * dot(d, p.right) / ahead, -p.focal * dot(d, p.up) / ahead, true
}

// Depth is how near the eye the middle of the cell under the point lies, on the ground: nearer is
// larger, and everything in one cell ties with its tile, so what a Composer is handed on a higher
// tier — the entities standing on it — is drawn over it and under what lies in front.
func (p perspective) Depth(x, y, _ float32) float32 {
	cx := (float32(math.Floor(float64(x/p.cell))) + 0.5) * p.cell
	cy := (float32(math.Floor(float64(y/p.cell))) + 0.5) * p.cell
	_, _, ahead := p.view(cx, cy, 0)
	return -ahead
}

func (perspective) Wraps() bool { return false }
func (perspective) Sorts() bool { return true }

// Toward is the way from what the eye looks at towards the eye: one way for the whole screen, as
// the shader takes it, though each point has its own.
func (p perspective) Toward() [3]float32 { return scale(p.forward, -1) }

// aheadOf is how far ahead of the eye the point lies, never nearer than near.
func (p perspective) aheadOf(x, y, z float32) float32 {
	_, _, ahead := p.view(x, y, z)
	return max(ahead, p.near)
}

func add(a, b [3]float32) [3]float32           { return [3]float32{a[0] + b[0], a[1] + b[1], a[2] + b[2]} }
func sub(a, b [3]float32) [3]float32           { return [3]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func scale(a [3]float32, s float32) [3]float32 { return [3]float32{a[0] * s, a[1] * s, a[2] * s} }
func dot(a, b [3]float32) float32              { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func norm(a [3]float32) float32                { return float32(math.Sqrt(float64(dot(a, a)))) }
func cross(a, b [3]float32) [3]float32 {
	return [3]float32{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
