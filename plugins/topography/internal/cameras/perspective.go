package cameras

import (
	"math"

	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/topography/internal/vec"
)

var _ camera.Projection = perspective{}

// perspective is the world seen from an eye at a point of it: a point lands on the screen where
// the line from it to the eye crosses the picture plane focal screen units before the eye, so what
// is further off is smaller. right, up and forward are the eye's frame — right along the screen,
// up up it, forward the way it looks — and the projected origin is where forward meets the screen.
// A point nearer than near is drawn as if at near; a screen point looking past the ground is put
// far along its line. cell is the world size of a cell, for Depth. bend is how far below the eye's
// level a point d off along the ground is drawn, per d² — the Earth's curve with the air's
// refraction; 0 flat.
type perspective struct {
	eye, right, up, forward      [3]float32
	focal, near, far, cell, bend float32
}

// newPerspective is the perspective of an eye at eye looking at target, the screen turned by
// heading as the isometric view turns it: 0 has the world's x run down-right and y down-left.
func newPerspective(eye, target [3]float32, heading, focal, near, far, cell float32) perspective {
	p := perspective{eye: eye, focal: focal, near: near, far: far, cell: cell}
	f := vec.Sub(target, eye)
	if n := vec.Norm(f); n > 0 {
		f = vec.Scale(f, 1/n)
	} else {
		f = [3]float32{0, 0, -1}
	}
	s, c := math.Sincos(float64(heading) + math.Pi/4)
	r := [3]float32{float32(c), -float32(s), 0}
	r = vec.Sub(r, vec.Scale(f, vec.Dot(r, f))) // square to the way the eye looks, whatever heading says
	if n := vec.Norm(r); n > 0 {
		r = vec.Scale(r, 1/n)
	} else {
		r = [3]float32{1, 0, 0}
	}
	p.forward, p.right, p.up = f, r, vec.Cross(f, r)
	return p
}

// view is the point (x, y, z) in the eye's frame: across, up and ahead of it, sunk by the curve
// of the ground as far off as it lies.
func (p perspective) view(x, y, z float32) (across, up, ahead float32) {
	dx, dy := x-p.eye[0], y-p.eye[1]
	d := [3]float32{dx, dy, z - p.eye[2] - p.bend*(dx*dx+dy*dy)}
	return vec.Dot(d, p.right), vec.Dot(d, p.up), vec.Dot(d, p.forward)
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
// With a curve the line meets the height where eye + t·d[2] = z − bend·(t·h)², h the line's run
// along the ground a unit of t: the nearer root ahead.
func (p perspective) cast(sx, sy, z float32) (x, y float32, hit bool) {
	d := p.ray(sx, sy)
	t := p.meet(d, z)
	if t <= 0 || t != t {
		t = p.far
	} else {
		hit = true
	}
	return p.eye[0] + d[0]*t, p.eye[1] + d[1]*t, hit
}

// meet is how far along d the line of sight meets the height z, the ground's curve and all: 0 or
// less, or NaN, where it never does ahead of the eye.
func (p perspective) meet(d [3]float32, z float32) float32 {
	a, b, c := p.bend*(d[0]*d[0]+d[1]*d[1]), d[2], p.eye[2]-z
	if a == 0 {
		if b == 0 {
			return 0
		}
		return -c / b
	}
	disc := float64(b)*float64(b) - 4*float64(a)*float64(c)
	if disc < 0 {
		return 0 // over the horizon
	}
	// the roots as q/a and c/q, q away from zero: neither loses its digits to a near cancellation
	q := -0.5 * (float64(b) + math.Copysign(math.Sqrt(disc), float64(b)))
	if q == 0 {
		return 0
	}
	t0, t1 := q/float64(a), float64(c)/q
	if t0 > t1 {
		t0, t1 = t1, t0
	}
	if t0 > 0 {
		return float32(t0)
	}
	return float32(t1)
}

// ray is the way from the eye through the screen point (sx, sy): forward, a focal length off, and
// across and up by the point.
func (p perspective) ray(sx, sy float32) [3]float32 {
	return vec.Add(p.forward, vec.Add(vec.Scale(p.right, sx/p.focal), vec.Scale(p.up, -sy/p.focal)))
}

// Vanish is where the direction (dx, dy, dz) vanishes on the screen — where everything far along
// it is drawn — and false for a direction not ahead of the eye.
func (p perspective) Vanish(dx, dy, dz float32) (float32, float32, bool) {
	d := [3]float32{dx, dy, dz}
	ahead := vec.Dot(d, p.forward)
	if ahead <= 1e-6 {
		return 0, 0, false
	}
	return p.focal * vec.Dot(d, p.right) / ahead, -p.focal * vec.Dot(d, p.up) / ahead, true
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
func (p perspective) Toward() [3]float32 { return vec.Scale(p.forward, -1) }

// aheadOf is how far ahead of the eye the point lies, never nearer than near.
func (p perspective) aheadOf(x, y, z float32) float32 {
	_, _, ahead := p.view(x, y, z)
	return max(ahead, p.near)
}
