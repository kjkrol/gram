package stick

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/vec"
)

// a ridge across the 20 x 20 cells of 32 from x0 to x1, 400 high, sloping down a cell either side
// as the relief interpolates its corners; flattened, it is level ground
type ridge struct {
	x0, x1 float32
	flat   bool
}

func (g *ridge) at(x float32) float32 {
	corner := func(c float32) float32 {
		if !g.flat && c >= g.x0 && c <= g.x1 {
			return 400
		}
		return 0
	}
	c := float32(math.Floor(float64(x / 32)))
	f := x/32 - c
	return corner(c*32)*(1-f) + corner((c+1)*32)*f
}

func (r *followRig) ridged(x0, x1 float32) *ridge {
	g := &ridge{x0: x0, x1: x1}
	r.sys.free = func(x, _, z float32) bool { return z >= g.at(x) }
	r.cam.persp.ground = func(x, _ float32) float32 { return g.at(x) }
	r.cam.persp.extent = func() (float32, float32) {
		if g.flat {
			return 0, 0
		}
		return 0, 400
	}
	return g
}

func TestFollow_InPerspectiveBehindTheArmShortensBeforeARidgeAndComesBack(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next() // isometric to perspective
	if !r.cam.inPersp {
		t.Fatal("the rig's camera is not in perspective")
	}
	r.walk(300, 300, 1, 0)      // east: the eye stands west of the walker
	ridge := r.ridged(256, 256) // west of the walker, sloping down to 288
	pitch := r.cam.Pitch()
	r.behind()
	unit := [3]float32{305, 305, 5}
	reach := func() float32 { return vec.Norm(vec.Sub(r.cam.persp.eye(), unit)) }
	full := float32(behindArm * 32)
	short := reach()
	if short >= full-1 || !r.centredOn(305, 305) || !near(r.cam.Pitch(), pitch) {
		t.Errorf("before the ridge the arm reaches %v at pitch %v, want shorter than its %v at "+
			"the player's %v, the walker in the middle", short, r.cam.Pitch(), full, pitch)
	}
	e := r.cam.persp.eye()
	for k := float32(0); k <= 1; k += 0.05 {
		if at := vec.Add(unit, vec.Scale(vec.Sub(e, unit), k)); !r.sys.free(at[0], at[1], at[2]) {
			t.Errorf("the ridge stands between the walker and the eye at %v", at)
			break
		}
	}
	ridge.flat = true
	r.tick(time.Second / 60)
	if got := reach(); got <= short || got >= full {
		t.Errorf("the ridge gone, a tick on the arm reaches %v, want easing out from %v to %v",
			got, short, full)
	}
	r.tick(3 * time.Second)
	if got := reach(); !near(got, full) {
		t.Errorf("the ridge gone, three seconds on the arm reaches %v, want all of it, %v", got,
			full)
	}
}

func TestFollow_InPerspectiveBehindTheWheelLeavesTheArm(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001) // a relief with no ridge in the way
	r.walk(300, 300, 1, 0)
	r.behind()
	unit := [3]float32{305, 305, 5}
	reach := func() float32 { return vec.Norm(vec.Sub(r.cam.persp.eye(), unit)) }
	full := float32(behindArm * 32)
	for _, zoom := range []func(){
		func() { r.cam.ZoomIn(2, 305, 305) },
		func() { r.cam.ZoomOut(4, 305, 305) },
	} {
		zoom()
		r.tick(time.Second / 60)
		if got := reach(); !near(got, full) || !r.centredOn(305, 305) {
			t.Errorf("zoomed behind the walker the arm reaches %v, want %v as ever, the walker "+
				"in the middle", got, full)
		}
	}
}

func TestFollow_InPerspectiveOverLevelGroundThePitchIsThePlayers(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001) // a relief with no ridge in the way
	r.walk(300, 300, 1, 0)
	want := r.cam.Pitch()
	r.behind()
	r.tick(time.Second)
	if p := r.cam.Pitch(); !near(p, want) {
		t.Errorf("over level ground a second on the eye looks down at %v, want %v still", p, want)
	}
	r.cam.Tilt(0.3)
	r.tick(time.Second / 60)
	r.tick(time.Second)
	if p := r.cam.Pitch(); !near(p, want+0.3) {
		t.Errorf("bowed by the player while fastened, a second on the pitch is %v, want %v: the player's", p, want+0.3)
	}
	unit := [3]float32{305, 305, 5}
	if d := vec.Norm(vec.Sub(r.cam.persp.eye(), unit)); !near(d, behindArm*32) {
		t.Errorf("fastened, the eye stands %v from the walker, want on the arm, %v", d,
			behindArm*32)
	}
}

func TestInside_RidesInTheWalkerLookingTheWayItFaces(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	iso := r.cam.ortho.Zoom()
	r.inside()
	if f := r.sys.fastened(r.cam); f == nil || !f.inside || !r.cam.insideUnit() || contract.HowOf(r.cam) != contract.Inside {
		t.Fatalf("fastened Inside: kept %v, first person %v; want riding in the walker", f != nil, r.cam.insideUnit())
	}
	if e := r.cam.persp.eye(); !near(e[0], 305) || !near(e[1], 305) || !near(e[2], 7) {
		t.Errorf("the eye is at %v, want on the walker's top at (305, 305, 7): its centre, 5 up and 2 high (world.Z.Top)", e)
	}
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(1, 0))) || !near(r.cam.Pitch(), 0) {
		t.Errorf("the heading is %v and the pitch %v, want %v — east running up the screen — along the ground", h, r.cam.Pitch(), wrapAngle(behind(1, 0)))
	}
	// the walker goes on and turns: the eye goes with it, looking the way it faces at once
	r.walk(400, 300, 0, 1)
	r.tick(time.Second / 60)
	if e := r.cam.persp.eye(); !near(e[0], 405) || !near(e[1], 305) || !near(e[2], 7) || !near(r.cam.Heading(), wrapAngle(behind(0, 1))) {
		t.Errorf("the walker gone east and turned south, the eye is at %v looking from %v, want (405, 305, 7) from %v", r.cam.persp.eye(), r.cam.Heading(), wrapAngle(behind(0, 1)))
	}
	// Q and E do nothing: the eye stays on the way the walker faces
	r.turns.Add(command.Nobody, TurnOrder{Camera: r.cam, Angle: 0.5})
	r.tick(time.Second / 60)
	if !near(r.cam.Heading(), wrapAngle(behind(0, 1))) {
		t.Errorf("Turn inside took the heading to %v, want %v still", r.cam.Heading(), wrapAngle(behind(0, 1)))
	}
	// the hand steers the walker: the view leaves the hand's part of its Driven alone
	r.hand(r.walkers[0], 1, -1)
	r.tick(time.Second / 60)
	if d, ok := r.drivenOf(r.walkers[0]); !ok || d.Ahead != 1 || d.Turn != -1 || !d.Flown {
		t.Errorf("driven by hand the walker is driven %+v, want on and turning left, flown from inside", d)
	}
	r.hand(r.walkers[0], 0, 0)
	r.cam.Tilt(-1)
	if r.cam.Pitch() >= 0 {
		t.Errorf("raised inside the pitch is %v, want negative: the sky", r.cam.Pitch())
	}
	// V again: back to the isometric view it was in, over the walker, as it was
	r.pressV()
	if r.sys.fastened(r.cam) != nil || r.cam.insideUnit() || r.cam.inPersp || contract.HowOf(r.cam) != contract.Centred {
		t.Fatalf("after V again: kept %v, first person %v, in perspective %v, fastened %+v; want back in the isometric view, over the walker", r.sys.fastened(r.cam) != nil, r.cam.insideUnit(), r.cam.inPersp, r.cam.Fastening())
	}
	if d, _ := r.drivenOf(r.walkers[0]); d.Flown || d.Look != (geom.Vec{}) {
		t.Errorf("out of the walker, it is driven %+v, want the eye's part of its Driven cleared", d)
	}
	if sx, sy := r.cam.Project(405, 305, 0); !near(sx, 200) || !near(sy, 150) || !near(r.cam.Zoom(), iso) {
		t.Errorf("back, the walker's ground is drawn at (%v, %v) at zoom %v, want the middle at %v", sx, sy, r.cam.Zoom(), iso)
	}
	// from the perspective, Tab comes out back to the perspective as it stood
	r.cam.next()
	pose := r.cam.persp.pose()
	r.inside()
	r.views.Add(command.Nobody, View{Camera: r.cam})
	r.tick(time.Second / 60)
	if r.cam.insideUnit() || !r.cam.inPersp || r.sys.fastened(r.cam) != nil || contract.HowOf(r.cam) != contract.Centred {
		t.Fatalf("Tab while riding: first person %v, in perspective %v, fastened %+v; want back in the free perspective, over the walker", r.cam.insideUnit(), r.cam.inPersp, r.cam.Fastening())
	}
	if p := r.cam.persp.pose(); p.length != pose.length || p.heading != pose.heading ||
		p.pitch != pose.pitch {
		t.Errorf("back, the free eye stands %+v, want as it stood %+v, moved over the walker only", p, pose)
	}
	if sx, sy := r.cam.Project(405, 305, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("back, the walker's ground is drawn at (%v, %v), want the middle", sx, sy)
	}
	// a walker that is gone: the eye stays out, the camera over nothing
	eye := r.cam.persp.eye()
	r.fasten(999, contract.Inside)
	if r.sys.fastened(r.cam) != nil || r.cam.persp.eye() != eye || r.cam.Fastening() != (contract.Fastening{}) {
		t.Error("fastened Inside a walker that is gone, the view kept the camera or moved the eye")
	}
}

func TestInside_TheRiddenUnitIsNotDrawn(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.inside()
	if !ridden(r.cam, 300, 300, 310, 310) || ridden(r.cam, 400, 300, 410, 310) {
		t.Error("riding in the walker at (300, 300), its box is not the one ridden, or another is")
	}
	r.pressV()
	if ridden(r.cam, 300, 300, 310, 310) {
		t.Error("let out, the walker's box is still ridden")
	}
}

func TestLook_TurnsTheViewAtOnceAndTheWalkerToFaceIt(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.inside()
	east := r.cam.Heading()
	r.looks.Add(command.Nobody, Look{Camera: r.cam, Dx: 200, Dy: -100})
	r.tick(time.Second / 60)
	a := float64(200 * LookStep)
	want := geom.NewVec(math.Cos(a), math.Sin(a)) // east turned clockwise, to the right
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(float32(want.X), float32(want.Y)))) || near(h, east) {
		t.Errorf("after the mouse went right the view looks from %v, want %v at once", h, wrapAngle(behind(float32(want.X), float32(want.Y))))
	}
	if p := r.cam.Pitch(); !near(p, -100*LookStep) {
		t.Errorf("after the mouse went up the pitch is %v, want %v: the head raised", p, -100*LookStep)
	}
	d, ok := r.drivenOf(r.walkers[0])
	if !ok || math.Abs(d.Look.X-want.X) > 1e-4 || math.Abs(d.Look.Y-want.Y) > 1e-4 || d.Ahead != 0 || d.Turn != 0 {
		t.Errorf("the walker is driven %+v, want to face %v", d, want)
	}
	if !d.Flown {
		t.Error("ridden, the walker is not flown from inside")
	}
	if rise := math.Sin(100 * LookStep); math.Abs(d.Climb-rise) > 1e-4 {
		t.Errorf("the head raised, the walker is steered at a rise of %v, want the look's %v", d.Climb, rise)
	}
	// the walker still faces east: the view stays where the eye looks, the walker turning to it
	r.tick(time.Second / 60)
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(float32(want.X), float32(want.Y)))) {
		t.Errorf("a tick on, the walker not turned yet, the view looks from %v, want where the mouse put it", h)
	}
	// the walker faces it now: pinned to it again, no more Look
	r.walk(300, 300, want.X, want.Y)
	r.tick(time.Second / 60)
	r.tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d.Look != (geom.Vec{}) {
		t.Errorf("the walker facing where the eye looks is still driven to face %v", d.Look)
	}
	r.walk(300, 300, 0, 1)
	r.tick(time.Second / 60)
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(0, 1))) {
		t.Errorf("the walker turned south by itself, the view looks from %v, want %v: pinned to it", h, wrapAngle(behind(0, 1)))
	}
	// A or D take over from the mouse
	r.looks.Add(command.Nobody, Look{Camera: r.cam, Dx: 300})
	r.hand(r.walkers[0], 0, -1)
	r.tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d.Look != (geom.Vec{}) || d.Turn != -1 {
		t.Errorf("the mouse and A together drive the walker %+v, want A's turn alone", d)
	}
	// out, the mouse does nothing
	r.pressV()
	h, p := r.cam.Heading(), r.cam.Pitch()
	r.looks.Add(command.Nobody, Look{Camera: r.cam, Dx: 300, Dy: 50})
	r.tick(time.Second / 60)
	if r.cam.Heading() != h || r.cam.Pitch() != p {
		t.Error("a Look for a camera riding in nothing moved it")
	}
}

func TestInside_TheEyeRidesOverWhatStandsOnTheCell(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	// a kind 20 tall on every cell
	r.sys.free = func(_, _, z float32) bool { return z >= 20 }
	r.walk(300, 300, 1, 0)
	r.inside()
	if e := r.cam.persp.eye(); e[2] < 20 || e[2] > 21 {
		t.Errorf("riding in a walker on cells 20 tall the eye stands %v up, want just over their top", e[2])
	}
	r.sys.free = func(_, _, z float32) bool { return z >= 0 }
	r.tick(time.Second / 60)
	if e := r.cam.persp.eye(); !near(e[2], 7) {
		t.Errorf("on level cells the eye stands %v up, want on the walker's top, 7", e[2])
	}
}

func ridden(cam contract.Camera, x0, y0, x1, y1 float32) bool {
	if !cam.(*viewCamera).insideUnit() {
		return false
	}
	x, y, _, ok := cam.(contract.Eyed).Eye()
	return ok && x >= x0 && x <= x1 && y >= y0 && y <= y1
}

func TestInside_TheRidersHeadTiltsTheUnitsEyeAndGivesItBack(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.inside()
	r.cam.Tilt(-0.5) // up: the pitch is negative towards the sky
	r.tick(time.Second / 60)
	if e := r.eyeOf(r.walkers[0]); !near(float32(e.Tilt), 0.5) {
		t.Errorf("looking up 0.5 inside, the unit's eye tilts %v, want 0.5", e.Tilt)
	}
	r.pressV()
	if e := r.eyeOf(r.walkers[0]); e.Tilt != 0 {
		t.Errorf("out of the unit its eye still tilts %v, want its own 0 back", e.Tilt)
	}
}

func TestFollow_InPerspectiveBehindTheTiltIsThePlayersAtOnceTheUnitInTheMiddle(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001) // a relief with no ridge in the way
	r.walk(300, 300, 1, 0)
	r.behind()
	if !r.centredOn(305, 305) {
		sx, sy := r.cam.Project(305, 305, 5)
		t.Errorf("behind the walker it is drawn at (%v, %v), want the middle (200, 150)", sx, sy)
	}
	for _, by := range []float32{0.6, -0.4} {
		want := r.cam.Pitch() + by
		r.tilts.Add(command.Nobody, TiltOrder{Camera: r.cam, Angle: by})
		r.tick(time.Second / 60)
		if p := r.cam.Pitch(); !near(p, want) {
			t.Errorf("tilted %v behind the walker, a tick on the pitch is %v, want %v at once",
				by, p, want)
		}
		if !r.centredOn(305, 305) {
			t.Errorf("tilted %v, the walker is off the middle of the screen", by)
		}
	}
}

func TestFollow_InPerspectiveLettingGoGivesTheStickBack(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001) // a relief with no ridge in the way
	length := r.cam.persp.length
	r.walk(300, 300, 1, 0)
	r.behind()
	if near(r.cam.persp.length, length) {
		t.Fatalf("behind the walker the stick is %v long, want the arm's, not %v",
			r.cam.persp.length, length)
	}
	r.letGo()
	if !near(r.cam.persp.length, length) || r.cam.persp.held {
		t.Errorf("let go, the stick is %v long, held %v; want the tactical %v, free",
			r.cam.persp.length, r.cam.persp.held, length)
	}
}
