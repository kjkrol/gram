package cameras

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/topography/internal/relief"
)

// ridged gives the rig's camera system and camera a relief over a 20 x 20 board of 32-unit cells
// with a ridge 400 high along x0 to x1, or level ground with none.
func (r *followRig) ridged(x0, x1 float64) *relief.Relief {
	relief := relief.New(grid.DefaultGrids{}.Square(20, 20, 32))
	relief.SetHeights(func(p geom.Vec) float64 {
		if p.X >= x0 && p.X <= x1 {
			return 400
		}
		return 0
	})
	r.sys.relief = relief
	r.cam.persp.ground = func(x, y float32) float32 { return float32(relief.GroundAt(geom.NewVec(float64(x), float64(y)))) }
	r.cam.persp.extent = func() (float32, float32) {
		low, high := relief.Extent()
		return float32(low), float32(high)
	}
	return relief
}

func (r *followRig) pressShiftV() {
	r.lookOuts.Add(control.Nobody, LookOut{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
}

func TestFollow_InPerspectiveTheEyeLooksDownSteeplyEnoughToSeeOverTheGround(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next() // isometric to perspective
	if !r.cam.inPersp {
		t.Fatal("the rig's camera is not in perspective")
	}
	r.walk(300, 300, 1, 0) // east: the eye stands west of the walker
	r.selectOnly(r.walkers[0])
	relief := r.ridged(160, 192) // west of the walker, sloping down to 224
	want := r.cam.Pitch()
	r.pressV()
	if len(r.sys.following) != 1 {
		t.Fatalf("after V: fastened %d, want 1", len(r.sys.following))
	}
	got := r.cam.Pitch()
	if got <= want+0.5 || got >= maxPitch {
		t.Errorf("behind the ridge the eye looks down at %v, want well steeper than %v, short of straight down", got, want)
	}
	unit := [3]float32{305, 305, 5}
	if e := r.cam.persp.eye(); !near(e[2], r.cam.persp.ceiling()) || !r.sys.lineClear(unit, e, 32) {
		t.Errorf("the eye flies at %v, want at the ceiling %v with the walker in sight over the ridge", e, r.cam.persp.ceiling())
	}
	r.ecs.Tick(time.Second / 60)
	if p := r.cam.Pitch(); !near(p, got) {
		t.Errorf("a tick on, the ridge still there, the pitch moved from %v to %v", got, p)
	}
	relief.SetHeights(func(geom.Vec) float64 { return 0 })
	r.ecs.Tick(time.Second / 60)
	if p := r.cam.Pitch(); p >= got || p <= want {
		t.Errorf("the ridge gone, a tick on the pitch is %v, want easing back from %v towards %v", p, got, want)
	}
	r.ecs.Tick(3 * time.Second)
	if p := r.cam.Pitch(); math.Abs(float64(p-want)) > 0.01 {
		t.Errorf("the ridge gone, three seconds on the pitch is %v, want back at %v", p, want)
	}
}

func TestFollow_InPerspectiveOverLevelGroundThePitchIsThePlayers(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001) // a relief with no ridge in the way
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	want := r.cam.Pitch()
	r.pressV()
	r.ecs.Tick(time.Second)
	if p := r.cam.Pitch(); !near(p, want) {
		t.Errorf("over level ground a second on the eye looks down at %v, want %v still", p, want)
	}
	r.cam.Tilt(0.3)
	r.ecs.Tick(time.Second / 60)
	r.ecs.Tick(time.Second)
	if p := r.cam.Pitch(); !near(p, want+0.3) {
		t.Errorf("bowed by the player while fastened, a second on the pitch is %v, want %v: the player's", p, want+0.3)
	}
	if e := r.cam.persp.eye(); e[2] < r.cam.persp.ceiling() {
		t.Errorf("fastened, the eye flies at %v, want no lower than the ceiling %v", e, r.cam.persp.ceiling())
	}
}

func TestLookOut_RidesInTheWalkerLookingTheWayItFaces(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	iso := r.cam.iso.Zoom()
	r.pressShiftV()
	if f := r.sys.fastened(r.cam); f == nil || !f.inside || !r.cam.FirstPerson() || contract.ModeOf(r.cam) != contract.FirstPerson {
		t.Fatalf("after LookOut: fastened %v, first person %v; want riding in the walker", f != nil, r.cam.FirstPerson())
	}
	if e := r.cam.persp.eye(); !near(e[0], 305) || !near(e[1], 305) || !near(e[2], 7) {
		t.Errorf("the eye is at %v, want on the walker's top at (305, 305, 7): its centre, 5 up and 2 high (world.Z.Top)", e)
	}
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(1, 0))) || !near(r.cam.Pitch(), 0) {
		t.Errorf("the heading is %v and the pitch %v, want %v — east running up the screen — along the ground", h, r.cam.Pitch(), wrapAngle(behind(1, 0)))
	}
	// the walker goes on and turns: the eye goes with it, looking the way it faces at once
	r.walk(400, 300, 0, 1)
	r.ecs.Tick(time.Second / 60)
	if e := r.cam.persp.eye(); !near(e[0], 405) || !near(e[1], 305) || !near(e[2], 7) || !near(r.cam.Heading(), wrapAngle(behind(0, 1))) {
		t.Errorf("the walker gone east and turned south, the eye is at %v looking from %v, want (405, 305, 7) from %v", r.cam.persp.eye(), r.cam.Heading(), wrapAngle(behind(0, 1)))
	}
	// Q and E do nothing: the eye stays on the way the walker faces
	r.turns.Add(control.Nobody, Turn{Camera: r.cam, Angle: 0.5})
	r.ecs.Tick(time.Second / 60)
	if !near(r.cam.Heading(), wrapAngle(behind(0, 1))) {
		t.Errorf("Turn inside took the heading to %v, want %v still", r.cam.Heading(), wrapAngle(behind(0, 1)))
	}
	// Drive steers the walker
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Ahead: 1, Turn: -1})
	r.ecs.Tick(time.Second / 60)
	if d, ok := r.drivenOf(r.walkers[0]); !ok || d.Ahead != 1 || d.Turn != -1 {
		t.Errorf("driven by hand the walker is driven %+v, want on and turning left", d)
	}
	r.cam.Tilt(-1)
	if r.cam.Pitch() >= 0 {
		t.Errorf("raised inside the pitch is %v, want negative: the sky", r.cam.Pitch())
	}
	// LookOut again: back to the isometric view it was in, over the walker, as it was
	r.pressShiftV()
	if r.sys.fastened(r.cam) != nil || r.cam.FirstPerson() || r.cam.inPersp {
		t.Fatalf("after LookOut again: fastened %v, first person %v, in perspective %v; want back in the isometric view", r.sys.fastened(r.cam) != nil, r.cam.FirstPerson(), r.cam.inPersp)
	}
	if sx, sy := r.cam.Project(405, 305, 0); !near(sx, 200) || !near(sy, 150) || !near(r.cam.Zoom(), iso) {
		t.Errorf("back, the walker's ground is drawn at (%v, %v) at zoom %v, want the middle at %v", sx, sy, r.cam.Zoom(), iso)
	}
	// from the perspective, Tab comes out back to the perspective as it stood
	r.cam.next()
	pose := r.cam.persp.pose()
	r.pressShiftV()
	r.views.Add(control.Nobody, View{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
	if r.cam.FirstPerson() || !r.cam.inPersp || r.sys.fastened(r.cam) != nil {
		t.Fatalf("Tab while riding: first person %v, in perspective %v; want back in the free perspective", r.cam.FirstPerson(), r.cam.inPersp)
	}
	if p := r.cam.persp.pose(); p.alt != pose.alt || p.heading != pose.heading || p.pitch != pose.pitch || p.narrow != pose.narrow {
		t.Errorf("back, the free eye stands %+v, want as it stood %+v, moved over the walker only", p, pose)
	}
	if sx, sy := r.cam.Project(405, 305, 0); !near(sx, 200) || !near(sy, 150) {
		t.Errorf("back, the walker's ground is drawn at (%v, %v), want the middle", sx, sy)
	}
	// nothing selected: nothing happens
	r.selectOnly()
	eye := r.cam.persp.eye()
	r.pressShiftV()
	if r.sys.fastened(r.cam) != nil || r.cam.persp.eye() != eye {
		t.Error("with nothing selected LookOut fastened the camera or moved the eye")
	}
}

func TestLookOut_TheRiddenUnitIsNotDrawn(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressShiftV()
	if !ridden(r.cam, 300, 300, 310, 310) || ridden(r.cam, 400, 300, 410, 310) {
		t.Error("riding in the walker at (300, 300), its box is not the one ridden, or another is")
	}
	r.pressShiftV()
	if ridden(r.cam, 300, 300, 310, 310) {
		t.Error("let out, the walker's box is still ridden")
	}
}

// The mouse looks round riding: across turns the view at once and has the walker turn to face
// it, till it does; up and down raise and lower the head; the keys turning the walker take over.
func TestLook_TurnsTheViewAtOnceAndTheWalkerToFaceIt(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressShiftV()
	east := r.cam.Heading()
	r.looks.Add(control.Nobody, Look{Camera: r.cam, Dx: 200, Dy: -100})
	r.ecs.Tick(time.Second / 60)
	a := float64(200 * LookStep)
	want := geom.NewVec(math.Cos(a), math.Sin(a)) // east turned clockwise, to the right
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(float32(want.X), float32(want.Y)))) || near(h, east) {
		t.Errorf("after the mouse went right the view looks from %v, want %v at once", h, wrapAngle(behind(float32(want.X), float32(want.Y))))
	}
	if p := r.cam.Pitch(); !near(p, -100*LookStep) {
		t.Errorf("after the mouse went up the pitch is %v, want %v: the head raised", p, -100*LookStep)
	}
	d, ok := r.drivenOf(r.walkers[0])
	if !ok || math.Abs(d.Face.X-want.X) > 1e-4 || math.Abs(d.Face.Y-want.Y) > 1e-4 || d.Ahead != 0 || d.Turn != 0 {
		t.Errorf("the walker is driven %+v, want to face %v", d, want)
	}
	if !d.Flown {
		t.Error("ridden, the walker is not flown from inside")
	}
	if rise := math.Sin(100 * LookStep); math.Abs(d.Climb-rise) > 1e-4 {
		t.Errorf("the head raised, the walker is steered at a rise of %v, want the look's %v", d.Climb, rise)
	}
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Ahead: 1, Sprint: true})
	r.ecs.Tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d.Ahead != 1 || !d.Sprint {
		t.Errorf("W with Shift drives the walker %+v, want on, sprinting", d)
	}
	// the walker still faces east: the view stays where the eye looks, the walker turning to it
	r.ecs.Tick(time.Second / 60)
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(float32(want.X), float32(want.Y)))) {
		t.Errorf("a tick on, the walker not turned yet, the view looks from %v, want where the mouse put it", h)
	}
	// the walker faces it now: pinned to it again, no more Face
	r.walk(300, 300, want.X, want.Y)
	r.ecs.Tick(time.Second / 60)
	r.ecs.Tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d.Face != (geom.Vec{}) {
		t.Errorf("the walker facing where the eye looks is still driven to face %v", d.Face)
	}
	r.walk(300, 300, 0, 1)
	r.ecs.Tick(time.Second / 60)
	if h := r.cam.Heading(); !near(h, wrapAngle(behind(0, 1))) {
		t.Errorf("the walker turned south by itself, the view looks from %v, want %v: pinned to it", h, wrapAngle(behind(0, 1)))
	}
	// A or D take over from the mouse
	r.looks.Add(control.Nobody, Look{Camera: r.cam, Dx: 300})
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Turn: -1})
	r.ecs.Tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d.Face != (geom.Vec{}) || d.Turn != -1 {
		t.Errorf("the mouse and A together drive the walker %+v, want A's turn alone", d)
	}
	// free, the mouse does nothing
	r.pressShiftV()
	h, p := r.cam.Heading(), r.cam.Pitch()
	r.looks.Add(control.Nobody, Look{Camera: r.cam, Dx: 300, Dy: 50})
	r.ecs.Tick(time.Second / 60)
	if r.cam.Heading() != h || r.cam.Pitch() != p {
		t.Error("a Look for a camera riding in nothing moved it")
	}
}

// An eye riding in a walker never goes under the top of the cell it stands in as it is drawn: on
// a cell whose kind stands 20 tall it looks from over that top.
func TestLookOut_TheEyeRidesOverWhatStandsOnTheCell(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	raised := func(p geom.Vec) float64 { return 20 } // a kind 20 tall on every cell
	r.sys.topAt = raised
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressShiftV()
	if e := r.cam.persp.eye(); e[2] < 20 || e[2] > 21 {
		t.Errorf("riding in a walker on cells 20 tall the eye stands %v up, want just over their top", e[2])
	}
	r.sys.topAt = func(geom.Vec) float64 { return 0 }
	r.ecs.Tick(time.Second / 60)
	if e := r.cam.persp.eye(); !near(e[2], 7) {
		t.Errorf("on level cells the eye stands %v up, want on the walker's top, 7", e[2])
	}
}

// ridden reports whether cam rides in the unit whose box is x0..x1, y0..y1, as the topography's
// look asks it through the camera's contracts: first person, the eye over the box.
func ridden(cam contract.Camera, x0, y0, x1, y1 float32) bool {
	if r, ok := cam.(contract.Rider); !ok || !r.FirstPerson() {
		return false
	}
	x, y, _, ok := cam.(contract.Eyed).Eye()
	return ok && x >= x0 && x <= x1 && y >= y0 && y <= y1
}
