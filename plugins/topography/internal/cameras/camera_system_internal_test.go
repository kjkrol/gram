package cameras

import (
	"bytes"
	"encoding/gob"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// followRig is the camera system over two 10x10 walkers 5 up, the first at (300, 300), each
// carrying a Driven as navigation gives a unit a hand is on, and an isometric camera reaching
// the perspective.
type followRig struct {
	t       *testing.T
	ecs     *goke.ECS
	sys     *cameraSystem
	turns   control.Queue[Turn]
	tilts   control.Queue[Tilt]
	rides   control.Queue[Ride]
	views   control.Queue[View]
	looks   control.Queue[Look]
	cam     *viewCamera
	cams    []*viewCamera
	walkers [2]uid.UID64
	// the query's own handles: a handle serves the query or factory it was built into
	base   goke.Comp[world.Base]
	q      *goke.Query
	driven goke.OptComp[steering.Driven]
	dq     *goke.Query
}

func newFollowRig(t *testing.T) *followRig {
	t.Helper()
	r := &followRig{t: t, ecs: goke.New()}
	r.cam = sized(newCamera(testProjection, 1280, 1280, 0, contract.Config{}, 0, true, nil, nil, 0), 400, 300)
	r.cams = []*viewCamera{r.cam}
	r.sys = &cameraSystem{orders: queued{turns: &r.turns, tilts: &r.tilts, rides: &r.rides, views: &r.views, looks: &r.looks}, perspective: true, cams: &r.cams}
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var driven goke.Comp[steering.Driven]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.base).Build()
		var b goke.Comp[world.Base]
		r.dq = si.NewQueryBuilder(&b).Optional(&r.driven).Build()
		f := si.NewFactory(&base, &z, &driven)
		f.Create(2)
		n := 0
		for f.Next() {
			for i, id := range f.Cursor.IDs {
				r.walkers[n] = id
				base.Slice(&f.Cursor)[i].Pos = world.Position{AABB: plane.NewAABB(geom.NewVec(300+float64(n)*100, 300), 10, 10)}
				z.Slice(&f.Cursor)[i] = world.Z{Altitude: 5, Height: 2}
				n++
			}
		}
	}})
	run := r.ecs.RegSys(r.sys)
	r.ecs.SetPlan(func(ctx goke.RunCtx, d time.Duration) { ctx.Run(run, d); ctx.Sync() })
	return r
}

// each calls fn with every walker's base.
func (r *followRig) each(fn func(id uid.UID64, b *world.Base)) {
	r.q.All()
	for r.q.Next() {
		cur := r.q.Cursor()
		for i, id := range cur.IDs {
			fn(id, &r.base.Slice(cur)[i])
		}
	}
}

// walk puts the first walker at (x, y) heading along (dx, dy).
func (r *followRig) walk(x, y, dx, dy float64) {
	r.each(func(id uid.UID64, b *world.Base) {
		if id == r.walkers[0] {
			b.Pos.AABB = plane.NewAABB(geom.NewVec(x, y), 10, 10)
			b.Vel.Dir, b.Vel.Value = geom.NewVec(dx, dy), 20
		}
	})
}

// fasten fastens the rig's camera to id how, as the players' Follow and Ride do, and ticks.
func (r *followRig) fasten(id uid.UID64, how contract.How) {
	r.cam.Fasten(contract.Fastening{Entity: id, How: how})
	r.ecs.Tick(time.Second / 60)
}

// behind fastens the rig's camera Behind the first walker; inside fastens it Inside.
func (r *followRig) behind() { r.fasten(r.walkers[0], contract.Behind) }
func (r *followRig) inside() { r.fasten(r.walkers[0], contract.Inside) }

// letGo lets the rig's camera go of everything, as the players' Follow again does, and ticks.
func (r *followRig) letGo() {
	r.cam.Fasten(contract.Fastening{})
	r.ecs.Tick(time.Second / 60)
}

// pressV gives a Ride and ticks.
func (r *followRig) pressV() {
	r.rides.Add(control.Nobody, Ride{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
}

// hand writes the hand's part of id's Driven, as navigation does: on, turning, sprinting.
func (r *followRig) hand(id uid.UID64, ahead, turn int8) {
	r.dq.All()
	for r.dq.Next() {
		cur := r.dq.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				if d := r.driven.Slice(cur); d != nil {
					d[i].Ahead, d[i].Turn = ahead, turn
				}
			}
		}
	}
}

// centredOn reports whether the camera holds (x, y) over its shoulder: across the middle, as far
// below it as its pitch puts it.
func (r *followRig) centredOn(x, y float32) bool {
	sx, sy := r.cam.Project(x, y, 5)
	below := 300 * shoulder * float32(math.Cos(float64(r.cam.Pitch())))
	return near(sx, 200) && near(sy, 150+below)
}

// upTheScreen reports whether a step along (dx, dy) from (x, y) goes straight up the screen.
func (r *followRig) upTheScreen(x, y, dx, dy float32) bool {
	ax, ay := r.cam.Project(x, y, 0)
	bx, by := r.cam.Project(x+dx*10, y+dy*10, 0)
	return math.Abs(float64(bx-ax)) < 0.05*math.Abs(float64(by-ay)) && by < ay
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
	r.ecs.Tick(time.Second / 60)
	if !r.centredOn(325, 385) || r.upTheScreen(325, 385, 0, 1) {
		t.Errorf("a tick after turning: centred %v, south already up the screen %v; want centred, turning eased",
			r.centredOn(325, 385), r.upTheScreen(325, 385, 0, 1))
	}
	r.ecs.Tick(3 * time.Second)
	if !r.upTheScreen(325, 385, 0, 1) {
		t.Error("given time the camera did not come round behind the walker heading south")
	}
}

func TestBehind_HoldsWhateverElseIsDoneUntilLetGo(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.behind()
	r.cam.Pan(30, 0)
	r.cam.ZoomIn(1.5, 305, 305)
	r.turns.Add(control.Nobody, Turn{Camera: r.cam, Angle: 0.2})
	r.ecs.Tick(time.Second / 60)
	if len(r.sys.following) != 1 || !r.centredOn(305, 305) {
		t.Fatalf("after panning, zooming and turning: kept %d, centred %v; want it held on the walker",
			len(r.sys.following), r.centredOn(305, 305))
	}
	r.letGo()
	if len(r.sys.following) != 0 {
		t.Error("the camera let go is still kept")
	}
	r.cam.Pan(30, 0)
	r.ecs.Tick(time.Second / 60)
	if r.centredOn(305, 305) {
		t.Error("the camera let go still comes back to the walker")
	}
}

// Ride goes round: Centred to Behind, Behind to Inside where the perspective is reached — else
// over the walker again — Inside to Centred; a camera fastened to nothing stays so.
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

// A camera saved riding inside a walker comes back inside it: a load writes its fastening and the
// system takes it in again.
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
	loaded := sized(newCamera(testProjection, 1280, 1280, 0, contract.Config{}, 0, true, nil, nil, 0), 400, 300)
	dec := gob.NewDecoder(&buf)
	for _, v := range loaded.Persisted() {
		if err := dec.Decode(v); err != nil {
			t.Fatal(err)
		}
	}
	loaded.Restore()
	r.cam, r.cams = loaded, []*viewCamera{loaded} // the camera a scene makes after the load
	r.ecs.Tick(time.Second / 60)
	if f := loaded.Fastening(); f != (contract.Fastening{Entity: r.walkers[0], How: contract.Inside}) || !loaded.insideUnit() {
		t.Errorf("after a load the camera is fastened %+v, riding %v; want Inside the walker again", f, loaded.insideUnit())
	}
}

// A camera fastened to a walker that is gone is let go of everything.
func TestBehind_LetsGoOfAWalkerGone(t *testing.T) {
	r := newFollowRig(t)
	r.fasten(999, contract.Behind)
	if len(r.sys.following) != 0 || r.cam.Fastening() != (contract.Fastening{}) {
		t.Errorf("fastened to a walker that is gone: kept %d, fastened %+v; want let go", len(r.sys.following), r.cam.Fastening())
	}
}

func TestTurn_TurnsACameraOfThisViewAndNoOther(t *testing.T) {
	r := newFollowRig(t)
	r.turns.Add(control.Nobody, Turn{Camera: r.cam, Angle: 0.5})
	r.turns.Add(control.Nobody, Turn{Camera: nil, Angle: 0.5})
	r.ecs.Tick(time.Second / 60)
	if !near(r.cam.Heading(), 0.5) {
		t.Errorf("heading %v after Turn by 0.5, want 0.5", r.cam.Heading())
	}
}

// drivenOf is how id is driven, and whether it carries a Driven at all.
func (r *followRig) drivenOf(id uid.UID64) (steering.Driven, bool) {
	r.dq.All()
	for r.dq.Next() {
		cur := r.dq.Cursor()
		for i, got := range cur.IDs {
			if got == id {
				if d := r.driven.Slice(cur); d != nil {
					return d[i], true
				}
				return steering.Driven{}, false
			}
		}
	}
	return steering.Driven{}, false
}
