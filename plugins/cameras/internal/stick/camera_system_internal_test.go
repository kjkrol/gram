package stick

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/vec"
	"github.com/kjkrol/gram/plugins/motion/steering"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

type followRig struct {
	t       *testing.T
	sys     *Keeper
	turns   command.Queue[TurnOrder]
	tilts   command.Queue[TiltOrder]
	rides   command.Queue[Ride]
	views   command.Queue[View]
	looks   command.Queue[Look]
	cam     *viewCamera
	cams    []*viewCamera
	walkers [2]uid.UID64
	ents    walkers
	arms    *armSettings
}

// the arm as the plugin's settings keep it, tilted within the plugin's bounds
type armSettings struct{ arm Arm }

func (a *armSettings) Arm() Arm { return a.arm }

func (a *armSettings) Tilt(pitch float32) {
	a.arm.Pitch = min(max(pitch, math.Pi/36), math.Pi/2)
}

// a walker as the keeper sees it
type walker struct {
	base   world.Base
	z      world.Z
	eye    world.Eye
	driven steering.Driven
}

type walkers map[uid.UID64]*walker

var _ Entities = walkers{}

func (ws walkers) Of(id uid.UID64) (Target, bool) {
	w, ok := ws[id]
	if !ok {
		return Target{}, false
	}
	box := w.base.Pos.AABB
	t := Target{
		X:       (box.TopLeft.X + box.BottomRight.X) / 2,
		Y:       (box.TopLeft.Y + box.BottomRight.Y) / 2,
		Alt:     w.z.Altitude,
		Level:   w.eye.Level(w.z),
		Across:  float32(w.eye.Angle),
		Tilt:    w.eye.Tilt,
		Eyed:    true,
		Turning: w.driven.Turn != 0,
		Moving:  w.base.Vel.Value > 0,
	}
	if n := f32.Hypot(w.base.Vel.Dir.X, w.base.Vel.Dir.Y); n > 0 {
		t.Dir = geom.NewVec(w.base.Vel.Dir.X/n, w.base.Vel.Dir.Y/n)
	}
	return t, true
}

func (ws walkers) Drive(id uid.UID64, look geom.Vec, flown bool, climb float32) {
	if w, ok := ws[id]; ok {
		w.driven.Look, w.driven.Flown, w.driven.Climb = look, flown, climb
	}
}

func (ws walkers) Tilt(id uid.UID64, tilt float32) {
	if w, ok := ws[id]; ok {
		w.eye.Tilt = tilt
	}
}

func newFollowRig(t *testing.T) *followRig {
	t.Helper()
	r := &followRig{t: t, ents: walkers{}}
	r.cam = testViewsOver(1280, true)
	r.cams = []*viewCamera{r.cam}
	orders := queued{
		turns: &r.turns,
		tilts: &r.tilts,
		rides: &r.rides,
		views: &r.views,
		looks: &r.looks,
	}
	r.arms = &armSettings{arm: Arm{Pitch: math.Pi / 6, Distance: 2}}
	r.sys = &Keeper{orders: orders, perspective: true, cams: &r.cams, arms: r.arms}
	for n := range r.walkers {
		id := uid.UID64(n + 1)
		r.walkers[n] = id
		r.ents[id] = &walker{
			base: world.Base{Pos: world.Position{
				AABB: plane.NewAABB(geom.NewVec(300+float32(n)*100, 300), 10, 10),
			}},
			z:   world.Z{Altitude: 5, Height: 2},
			eye: world.Eye{Height: 2}, // the top, as without one
		}
	}
	return r
}

func (r *followRig) tick(d time.Duration) { r.sys.Update(d, r.ents) }

func (r *followRig) walk(x, y, dx, dy float32) {
	b := &r.ents[r.walkers[0]].base
	b.Pos.AABB = plane.NewAABB(geom.NewVec(x, y), 10, 10)
	b.Vel.Dir, b.Vel.Value = geom.NewVec(dx, dy), 20
}

func (r *followRig) fasten(id uid.UID64, how contract.How) {
	r.cam.Fasten(contract.Fastening{Entity: id, How: how})
	r.tick(time.Second / 60)
}

func (r *followRig) behind() { r.fasten(r.walkers[0], contract.Behind) }
func (r *followRig) inside() { r.fasten(r.walkers[0], contract.Inside) }

// V with id to go behind, for a camera fastened to nothing
func (r *followRig) rideOn(id uid.UID64) {
	r.rides.Add(command.Nobody, Ride{Camera: r.cam, Entity: id})
	r.tick(time.Second / 60)
}

func (r *followRig) letGo() {
	r.cam.Fasten(contract.Fastening{})
	r.tick(time.Second / 60)
}

func (r *followRig) pressV() {
	r.rides.Add(command.Nobody, Ride{Camera: r.cam})
	r.tick(time.Second / 60)
}

func (r *followRig) hand(id uid.UID64, ahead, turn int8) {
	r.ents[id].driven.Ahead, r.ents[id].driven.Turn = ahead, turn
}

func (r *followRig) centredOn(x, y float32) bool {
	sx, sy := r.cam.Project(x, y, 5)
	return near(sx, 200) && near(sy, 150)
}

func (r *followRig) upTheScreen(x, y, dx, dy float32) bool {
	ax, ay := r.cam.Project(x, y, 0)
	bx, by := r.cam.Project(x+dx*10, y+dy*10, 0)
	return f32.Abs(float32(bx-ax)) < 0.05*f32.Abs(float32(by-ay)) && by < ay
}

func TestBehind_FastensTheCameraBehindTheWalkerWhereverItHeads(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.behind()
	if len(r.sys.following) != 1 || !r.centredOn(305, 305) || !r.upTheScreen(305, 305, 1, 0) {
		t.Fatalf("fastened Behind: kept %d, centred %v, heading east up the screen %v; want all",
			len(r.sys.following), r.centredOn(305, 305), r.upTheScreen(305, 305, 1, 0))
	}
	r.walk(320, 380, 0, 1) // turned south
	r.tick(time.Second / 60)
	if !r.centredOn(325, 385) || r.upTheScreen(325, 385, 0, 1) {
		t.Errorf(
			"a tick after turning: centred %v, south already up the screen %v; want centred, turning eased",
			r.centredOn(325, 385),
			r.upTheScreen(325, 385, 0, 1),
		)
	}
	r.tick(3 * time.Second)
	if !r.upTheScreen(325, 385, 0, 1) {
		t.Error("given time the camera did not come round behind the walker heading south")
	}
}

func TestBehind_IsThePerspectiveAsNearAsTheArmAndLettingGoGivesTheViewBack(t *testing.T) {
	r := newFollowRig(t)
	tactical, turn, pitch := r.cam.Zoom(), r.cam.Heading(), r.cam.Pitch()
	r.walk(300, 300, 1, 0)
	r.behind()
	arm := r.arms.arm.Distance * 32
	if !r.cam.inPersp || !near(r.cam.persp.length, arm) || !r.centredOn(305, 305) {
		t.Fatalf("behind the walker: in perspective %v, the stick %v long, centred %v; want the "+
			"perspective, the arm's %v, the walker in the middle", r.cam.inPersp,
			r.cam.persp.length, r.centredOn(305, 305), arm)
	}
	r.cam.ZoomIn(2, 305, 305)
	r.cam.ZoomOut(3, 305, 305)
	r.tick(time.Second / 60)
	if !near(r.cam.persp.length, arm) {
		t.Errorf("zoomed behind the walker, the stick is %v long, want %v as it was",
			r.cam.persp.length, arm)
	}
	r.letGo()
	if r.cam.inPersp || r.cam.Zoom() != tactical || r.cam.Heading() != turn ||
		r.cam.Pitch() != pitch {
		t.Errorf("let go: in perspective %v, zoom %v, heading %v, pitch %v; want the "+
			"orthographic view back as it was: %v, %v, %v", r.cam.inPersp, r.cam.Zoom(),
			r.cam.Heading(), r.cam.Pitch(), tactical, turn, pitch)
	}
	if r.cam.ZoomIn(2, 305, 305); f32.Abs(float32(r.cam.Zoom()-2*tactical)) > 1e-3 {
		t.Errorf("let go, zoomed in twice the zoom is %v, want %v", r.cam.Zoom(), 2*tactical)
	}
}

func TestBehind_HoldsWhateverElseIsDoneUntilLetGo(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.behind()
	r.cam.Pan(30, 0)
	r.cam.ZoomIn(1.5, 305, 305)
	r.turns.Add(command.Nobody, TurnOrder{Camera: r.cam, Angle: 0.2})
	r.tick(time.Second / 60)
	if len(r.sys.following) != 1 || !r.centredOn(305, 305) {
		t.Fatalf(
			"after panning, zooming and turning: kept %d, centred %v; want it held on the walker",
			len(r.sys.following),
			r.centredOn(305, 305),
		)
	}
	r.letGo()
	if len(r.sys.following) != 0 {
		t.Error("the camera let go is still kept")
	}
	r.cam.Pan(30, 0)
	r.tick(time.Second / 60)
	if r.centredOn(305, 305) {
		t.Error("the camera let go still comes back to the walker")
	}
}

func TestRide_GoesRoundBehindInsideAndOverAgain(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.pressV()
	if f := r.cam.Fastening(); f != (contract.Fastening{}) || len(r.sys.following) != 0 {
		t.Fatalf("V with the camera fastened to nothing fastened it %+v", f)
	}
	r.fasten(r.walkers[0], contract.Centred)
	if len(r.sys.following) != 0 {
		t.Fatal("a camera fastened Centred is kept by the view: it is the players'")
	}
	r.pressV()
	if f := r.cam.Fastening(); f.How != contract.Behind || r.sys.fastened(r.cam) == nil ||
		r.sys.fastened(r.cam).how != contract.Behind || !r.cam.inPersp {
		t.Fatalf("V over the walker: fastened %+v, in perspective %v; want Behind, kept so, in "+
			"the perspective", f, r.cam.inPersp)
	}
	r.pressV()
	if f := r.cam.Fastening(); f.How != contract.Inside || r.sys.fastened(r.cam) == nil ||
		!r.sys.fastened(r.cam).inside() || !r.cam.insideUnit() {
		t.Fatalf("V behind the walker: fastened %+v, want Inside and riding", f)
	}
	r.pressV()
	centred := contract.Fastening{Entity: r.walkers[0], How: contract.Centred}
	if f := r.cam.Fastening(); f != centred ||
		r.sys.fastened(r.cam) != nil ||
		r.cam.insideUnit() ||
		r.cam.inPersp {
		t.Fatalf(
			"V inside the walker: fastened %+v, kept %v, in perspective %v; want Centred over "+
				"it, back in the view it rode from",
			f,
			r.sys.fastened(r.cam) != nil,
			r.cam.inPersp,
		)
	}
	r.sys.perspective = false
	r.pressV()
	if f := r.cam.Fastening(); f != centred || r.sys.fastened(r.cam) != nil || r.cam.inPersp {
		t.Errorf("without the perspective V over the walker: fastened %+v, kept %v; want it left "+
			"over the walker", f, r.sys.fastened(r.cam) != nil)
	}
}

func TestRide_WithAnEntityFastensALooseCameraBehindIt(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.rideOn(r.walkers[0])
	behind := contract.Fastening{Entity: r.walkers[0], How: contract.Behind}
	if f := r.cam.Fastening(); f != behind ||
		r.sys.fastened(r.cam) == nil ||
		!r.centredOn(305, 305) {
		t.Fatalf(
			"V with the camera loose and the walker given: fastened %+v, kept %v; want Behind "+
				"it",
			f,
			r.sys.fastened(r.cam) != nil,
		)
	}
	r.letGo()
	r.rideOn(999)
	if f := r.cam.Fastening(); f != (contract.Fastening{}) {
		t.Errorf("V given a walker that is gone fastened the camera %+v, want nothing", f)
	}
	r.fasten(r.walkers[0], contract.Centred)
	r.rideOn(r.walkers[1])
	behind = contract.Fastening{Entity: r.walkers[0], How: contract.Behind}
	if f := r.cam.Fastening(); f != behind {
		t.Errorf("V over one walker, given another, fastened %+v, want Behind its own", f)
	}
}

func TestRide_WithoutThePerspectiveNothingGoesBehind(t *testing.T) {
	r := newFollowRig(t)
	r.cam = testViewsOver(1280, false)
	r.cams[0] = r.cam
	r.sys.perspective = false
	r.walk(300, 300, 1, 0)
	r.fasten(r.walkers[0], contract.Centred)
	centred := contract.Fastening{Entity: r.walkers[0], How: contract.Centred}
	r.pressV()
	if f := r.cam.Fastening(); f != centred || r.sys.fastened(r.cam) != nil {
		t.Errorf("V over the walker: fastened %+v, kept %v; want it left over the walker", f,
			r.sys.fastened(r.cam) != nil)
	}
	r.behind()
	if f := r.cam.Fastening(); f != centred || r.sys.fastened(r.cam) != nil || r.cam.inPersp {
		t.Errorf("fastened Behind by hand: fastened %+v, kept %v, in perspective %v; want it "+
			"put over the walker", f, r.sys.fastened(r.cam) != nil, r.cam.inPersp)
	}
	r.letGo()
	r.rideOn(r.walkers[0])
	if f := r.cam.Fastening(); f != (contract.Fastening{}) || r.sys.fastened(r.cam) != nil {
		t.Errorf("V with the camera loose and the walker given: fastened %+v, want nothing", f)
	}
}

func TestView_OnTheArmTabKeepsThePerspective(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.behind()
	r.views.Add(command.Nobody, View{Camera: r.cam})
	r.tick(time.Second / 60)
	if f := r.cam.Fastening(); !r.cam.inPersp || f.How != contract.Behind ||
		r.sys.fastened(r.cam) == nil {
		t.Errorf("Tab behind the walker: in perspective %v, fastened %+v, kept %v; want the "+
			"perspective kept behind it", r.cam.inPersp, f, r.sys.fastened(r.cam) != nil)
	}
	r.letGo()
	if r.cam.inPersp {
		t.Error("let go after Tab, the camera is not back in the orthographic view")
	}
}

func TestRide_ACameraSavedInsideComesBackInside(t *testing.T) {
	r := newFollowRig(t)
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.inside()
	if !r.cam.insideUnit() {
		t.Fatal("fastened Inside the camera does not ride in the walker")
	}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range r.cam.Persisted() {
		if err := enc.Encode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded := testViewsOver(1280, true)
	dec := gob.NewDecoder(&buf)
	for _, v := range loaded.Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded.Restore()
	r.cam, r.cams = loaded, []*viewCamera{loaded} // the camera a scene makes after the load
	r.tick(time.Second / 60)
	inside := contract.Fastening{Entity: r.walkers[0], How: contract.Inside}
	if f := loaded.Fastening(); f != inside ||
		!loaded.insideUnit() {
		t.Errorf(
			"after a load the camera is fastened %+v, riding %v; want Inside the walker again",
			f,
			loaded.insideUnit(),
		)
	}
}

func TestBehind_LetsGoOfAWalkerGone(t *testing.T) {
	r := newFollowRig(t)
	r.fasten(999, contract.Behind)
	if len(r.sys.following) != 0 || r.cam.Fastening() != (contract.Fastening{}) {
		t.Errorf(
			"fastened to a walker that is gone: kept %d, fastened %+v; want let go",
			len(r.sys.following),
			r.cam.Fastening(),
		)
	}
}

func TestTurn_TurnsACameraOfThisViewAndNoOther(t *testing.T) {
	r := newFollowRig(t)
	r.turns.Add(command.Nobody, TurnOrder{Camera: r.cam, Angle: 0.5})
	r.turns.Add(command.Nobody, TurnOrder{Camera: nil, Angle: 0.5})
	r.tick(time.Second / 60)
	if !near(r.cam.Heading(), 0.5) {
		t.Errorf("heading %v after Turn by 0.5, want 0.5", r.cam.Heading())
	}
}

func (r *followRig) eyeOf(id uid.UID64) world.Eye { return r.ents[id].eye }

func (r *followRig) drivenOf(id uid.UID64) (steering.Driven, bool) {
	return r.ents[id].driven, true
}

// the mouse moved by dx and dy pixels
func (r *followRig) look(dx, dy float32) {
	r.looks.Add(command.Nobody, Look{Camera: r.cam, Dx: dx, Dy: dy})
	r.tick(time.Second / 60)
}

// the walker stops, facing as it does
func (r *followRig) stand() { r.ents[r.walkers[0]].base.Vel.Value = 0 }

func TestOrbit_BehindTheMouseTurnsTheCameraRoundTheWalkerNotTheWalker(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.walk(300, 300, 1, 0)
	r.behind()
	r.arms.arm.Tilts = true
	h, p := r.cam.Heading(), r.cam.Pitch()
	r.look(200, 0)
	if got := r.cam.Heading(); !near(got, wrapAngle(h-200*LookStep)) || !r.centredOn(305, 305) {
		t.Errorf("the mouse 200 px across: heading %v, want %v, the walker in the middle", got,
			wrapAngle(h-200*LookStep))
	}
	w := r.ents[r.walkers[0]]
	if w.driven.Look != (geom.Vec{}) || w.base.Vel.Dir != geom.NewVec(1, 0) {
		t.Errorf("the walker looks %v and heads %v, want it left to its way east", w.driven.Look,
			w.base.Vel.Dir)
	}
	r.look(0, 100)
	if got := r.cam.Pitch(); !near(got, p+100*LookStep) {
		t.Errorf("the mouse 100 px down: pitch %v, want %v", got, p+100*LookStep)
	}
}

func TestOrbit_HoldsWhileTheWalkerStandsAndComesBackBehindItAsItWalks(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.behind()
	r.stand()
	r.look(200, 0)
	h := r.cam.Heading()
	r.tick(3 * time.Second)
	if got := r.cam.Heading(); !near(got, h) {
		t.Errorf("the walker standing, three seconds on the heading is %v, want the mouse's %v",
			got, h)
	}
	r.look(10, 0)
	h = r.cam.Heading()
	r.walk(300, 300, 1, 0)
	r.tick(time.Second / 60)
	if got := r.cam.Heading(); !near(got, h) {
		t.Errorf("a tick into the walk, right after the mouse, the heading is %v, want its %v held",
			got, h)
	}
	r.tick(3 * time.Second)
	if !r.upTheScreen(305, 305, 1, 0) {
		t.Error("walking east three seconds, the camera did not come back behind the walker")
	}
	r.look(100, 0)
	h = r.cam.Heading()
	r.tick(time.Second / 2)
	if got := r.cam.Heading(); !near(got, h) {
		t.Errorf("half a second after the mouse the heading is %v, want its %v held a second",
			got, h)
	}
}

func TestOrbit_BehindTheMouseTiltsTheArmWhereTheSettingsLetItUnderTheFloorTakenBack(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.behind()
	floor, p := r.cam.persp.minPitch, r.cam.Pitch()
	r.look(0, -1000)
	if got := r.cam.Pitch(); !near(got, p) {
		t.Errorf("the mouse up with the lean off: pitch %v, want the arm's %v untouched", got, p)
	}
	r.arms.arm.Tilts = true
	r.look(0, -1000)
	if got := r.cam.Pitch(); got >= floor || !near(got, math.Pi/36) {
		t.Errorf("the mouse 1000 px up behind the walker: pitch %v, want the settings' least %v, "+
			"under the floor %v", got, math.Pi/36, floor)
	}
	if !r.centredOn(305, 305) {
		t.Error("tilted under the floor, the walker is no longer in the middle")
	}
	r.letGo()
	if got := r.cam.Pitch(); got < floor {
		t.Errorf("let go, the pitch is %v, want the floor %v back", got, floor)
	}
}

func TestArm_HoldsTheEyeAsTheSettingsSayAimedUpTheWalker(t *testing.T) {
	r := newFollowRig(t)
	r.cam.next()
	r.ridged(1000, 1001)
	r.walk(300, 300, 1, 0)
	r.arms.arm = Arm{Pitch: math.Pi / 9, Distance: 3, Aim: 1}
	r.behind()
	aim := [3]float32{305, 305, 7} // the walker's top: its feet at 5, 2 high
	if d := vec.Norm(vec.Sub(r.cam.persp.eye(), aim)); !near(d, 3*32) ||
		!near(r.cam.Pitch(), math.Pi/9) {
		t.Errorf("the eye stands %v from the walker's top at pitch %v, want 3 cells at a ninth",
			d, r.cam.Pitch())
	}
	if sx, sy := r.cam.Project(305, 305, 7); f32.Abs(sx-200) > 0.5 || f32.Abs(sy-150) > 0.5 {
		t.Errorf("the walker's top is drawn at (%v, %v), want the middle: the arm aims up it",
			sx, sy)
	}
	if x, y, z := r.cam.Focus(); x != 305 || y != 305 || z != 7 {
		t.Errorf("the focus is (%v, %v, %v), want the point aimed at", x, y, z)
	}
	r.arms.arm.Distance = 1
	r.tick(time.Second / 60)
	if d := vec.Norm(vec.Sub(r.cam.persp.eye(), aim)); !near(d, 32) {
		t.Errorf("the arm drawn in to a cell, the eye stands %v off, want 32", d)
	}
}
