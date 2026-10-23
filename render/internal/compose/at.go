package compose

import "math"

// At is where a piece of the picture lies: at a height in the world, its depth worked out on the
// GPU, drawn back to front by its key, or on top of the world as a mark.
type At struct {
	z, key float32
	top    bool
	over   bool // tested against the ground's depth alone
}

var (
	// Furthest is behind the whole world: the sky.
	Furthest = At{key: float32(math.Inf(-1))}
	// Nearest is before the whole world: what falls through the air.
	Nearest = At{key: float32(math.Inf(1))}
)

// OnTop is a mark at level, not the world: drawn over it on the screen, a higher level over a
// lower, at one level in the order the marks came.
func OnTop(level float32) At { return At{key: level, top: true} }

// Z is the height the piece lies at.
func (a At) Z() float32 { return a.z }

// Top reports whether a is a mark, drawn over the world.
func (a At) Top() bool { return a.top }

// OverFigures is a drawn over whatever stands, hidden by the ground alone: a look into the world,
// such as the solids of the entities' boxes, costing a copy of the ground's depth while drawn.
func (a At) OverFigures() At {
	a.over = true
	return a
}

// Before reports whether a is drawn before b among what is drawn back to front.
func (a At) Before(b At) bool { return a.key < b.key }

// At is where the world point (x, y, z) lies through the frame's camera: from above the higher
// later, in a view that sorts the further back sooner.
func (f *Frame) At(x, y, z float32) At {
	if f.cam != nil && f.cam.Projection().Sorts() {
		return At{z: z, key: f.cam.Depth(x, y, z)}
	}
	return At{z: z, key: z}
}

// Lifted is At of the point lift world units over (x, y, z): what lies on a surface, ordered among
// what lies on it alike.
func (f *Frame) Lifted(x, y, z, lift float32) At { return f.At(x, y, z+lift) }

// OnGround is Lifted of the ground at 0 under the middle of the view: for what a Direct lays over
// the whole ground.
func (f *Frame) OnGround(lift float32) At {
	if f.cam == nil {
		return At{z: lift, key: lift}
	}
	w, h := f.cam.Viewport()
	x, y := f.cam.Unproject(w/2, h/2, 0)
	return f.Lifted(x, y, 0, lift)
}
