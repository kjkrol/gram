package stick

import (
	"bytes"
	"encoding/gob"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
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
	r.sys = &Keeper{orders: orders, perspective: true, cams: &r.cams}
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
		t.Errorf("a tick after turning: centred %v, south already up the screen %v; want centred, turning eased",
			r.centredOn(325, 385), r.upTheScreen(325, 385, 0, 1))
	}
	r.tick(3 * time.Second)
	if !r.upTheScreen(325, 385, 0, 1) {
		t.Error("given time the camera did not come round behind the walker heading south")
	}
}

func TestBehind_IsNearNothingZoomsItAndLettingGoGivesTheTacticalViewBack(t *testing.T) {
	r := newFollowRig(t)
	tactical := r.cam.Zoom()
	r.walk(300, 300, 1, 0)
	r.behind()
	arm := r.cam.persp.focal() / (behindArm * 32)
	if z := r.cam.Zoom(); f32.Abs(float32(z-arm)) > 1e-3 || !r.centredOn(305, 305) {
		t.Errorf("behind the walker the zoom is %v, want the arm's %v, the walker in the middle",
			z, arm)
	}
	r.cam.ZoomIn(2, 305, 305)
	r.cam.ZoomOut(3, 305, 305)
	r.tick(time.Second / 60)
	if z := r.cam.Zoom(); f32.Abs(float32(z-arm)) > 1e-3 {
		t.Errorf("zoomed behind the walker, the zoom is %v, want %v as it was", z, arm)
	}
	r.letGo()
	if z := r.cam.Zoom(); z != tactical {
		t.Errorf("let go, the zoom is %v, want the tactical %v back", z, tactical)
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
		t.Fatalf("after panning, zooming and turning: kept %d, centred %v; want it held on the walker",
			len(r.sys.following), r.centredOn(305, 305))
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
	if f := r.cam.Fastening(); f.How != contract.Behind || r.sys.fastened(r.cam) == nil || r.sys.fastened(r.cam).inside {
		t.Fatalf("V over the walker: fastened %+v, want Behind and kept so", f)
	}
	r.pressV()
	if f := r.cam.Fastening(); f.How != contract.Inside || r.sys.fastened(r.cam) == nil || !r.sys.fastened(r.cam).inside || !r.cam.insideUnit() {
		t.Fatalf("V behind the walker: fastened %+v, want Inside and riding", f)
	}
	r.pressV()
	if f := r.cam.Fastening(); f != (contract.Fastening{Entity: r.walkers[0], How: contract.Centred}) || r.sys.fastened(r.cam) != nil || r.cam.insideUnit() {
		t.Fatalf("V inside the walker: fastened %+v, kept %v; want Centred over it, out of it", f, r.sys.fastened(r.cam) != nil)
	}
	r.sys.perspective = false
	r.pressV()
	r.pressV()
	if f := r.cam.Fastening(); f.How != contract.Centred || r.sys.fastened(r.cam) != nil {
		t.Errorf("without the perspective V behind the walker: fastened %+v, want over it again", f)
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
	if f := loaded.Fastening(); f != (contract.Fastening{Entity: r.walkers[0], How: contract.Inside}) || !loaded.insideUnit() {
		t.Errorf("after a load the camera is fastened %+v, riding %v; want Inside the walker again", f, loaded.insideUnit())
	}
}

func TestBehind_LetsGoOfAWalkerGone(t *testing.T) {
	r := newFollowRig(t)
	r.fasten(999, contract.Behind)
	if len(r.sys.following) != 0 || r.cam.Fastening() != (contract.Fastening{}) {
		t.Errorf("fastened to a walker that is gone: kept %d, fastened %+v; want let go", len(r.sys.following), r.cam.Fastening())
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
