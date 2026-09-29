package topography

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	contract "github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/steering"
	"github.com/kjkrol/uid"
)

// selected is the Selected tag of the rig's selection family.
const selected plugin.Tag[selection.Family] = 1

// followRig is the camera system over two 10x10 walkers 5 up, the first at (300, 300), and an
// isometric camera.
type followRig struct {
	t        *testing.T
	ecs      *goke.ECS
	sys      *cameraSystem
	turns    control.Queue[Turn]
	tilts    control.Queue[Tilt]
	drives   control.Queue[Drive]
	follow   control.Queue[Follow]
	lookOuts control.Queue[LookOut]
	views    control.Queue[View]
	looks    control.Queue[Look]
	cam      *viewCamera
	walkers  [2]uid.UID64
	// the query's own handles: a handle serves the query or factory it was built into
	base   goke.Comp[world.Base]
	marks  goke.Comp[plugin.Tags[selection.Family]]
	q      *goke.Query
	driven goke.OptComp[steering.Driven]
	dq     *goke.Query
}

func newFollowRig(t *testing.T) *followRig {
	t.Helper()
	r := &followRig{t: t, ecs: goke.New()}
	r.cam = newCamera(testProjection, 1280, 1280, 0, contract.Config{ViewportWidth: 400, ViewportHeight: 300}, 0, true, nil, nil, 0)
	r.sys = &cameraSystem{turns: &r.turns, tilts: &r.tilts, follows: &r.follow, drives: &r.drives, lookOuts: &r.lookOuts, views: &r.views, looks: &r.looks, selected: selected, selecting: true}
	var base goke.Comp[world.Base]
	var z goke.Comp[world.Z]
	var marks goke.Comp[plugin.Tags[selection.Family]]
	r.ecs.Setup(goke.SystemFn{OnInit: func(si *goke.SysInit) {
		r.q = si.NewQueryBuilder(&r.base, &r.marks).Build()
		var b goke.Comp[world.Base]
		r.dq = si.NewQueryBuilder(&b).Optional(&r.driven).Build()
		f := si.NewFactory(&base, &z, &marks)
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

// each calls fn with every walker's base and marks.
func (r *followRig) each(fn func(id uid.UID64, b *world.Base, m *plugin.Tags[selection.Family])) {
	r.q.All()
	for r.q.Next() {
		cur := r.q.Cursor()
		for i, id := range cur.IDs {
			fn(id, &r.base.Slice(cur)[i], &r.marks.Slice(cur)[i])
		}
	}
}

// walk puts the first walker at (x, y) heading along (dx, dy).
func (r *followRig) walk(x, y, dx, dy float64) {
	r.each(func(id uid.UID64, b *world.Base, _ *plugin.Tags[selection.Family]) {
		if id == r.walkers[0] {
			b.Pos.AABB = plane.NewAABB(geom.NewVec(x, y), 10, 10)
			b.Vel.Dir, b.Vel.Value = geom.NewVec(dx, dy), 20
		}
	})
}

// selectOnly has exactly the walkers given Selected.
func (r *followRig) selectOnly(ids ...uid.UID64) {
	r.each(func(id uid.UID64, _ *world.Base, m *plugin.Tags[selection.Family]) {
		*m = m.Without(selected)
		for _, want := range ids {
			if id == want {
				*m = m.With(selected)
			}
		}
	})
}

func (r *followRig) pressV() {
	r.follow.Add(control.Nobody, Follow{Camera: r.cam})
	r.ecs.Tick(time.Second / 60)
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

func TestFollow_FastensTheCameraBehindTheSelectedWalkerWhereverItHeads(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressV()
	if len(r.sys.following) != 1 || !r.centredOn(305, 305) || !r.upTheScreen(305, 305, 1, 0) {
		t.Fatalf("after V: fastened %d, centred %v, heading east up the screen %v; want all",
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

func TestFollow_HoldsWhateverElseIsDoneUntilVAgain(t *testing.T) {
	r := newFollowRig(t)
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressV()
	r.selectOnly(r.walkers[1]) // another unit selected, given orders
	r.cam.Pan(30, 0)
	r.cam.ZoomIn(1.5, 305, 305)
	r.turns.Add(control.Nobody, Turn{Camera: r.cam, Angle: 0.2})
	r.ecs.Tick(time.Second / 60)
	if len(r.sys.following) != 1 || !r.centredOn(305, 305) {
		t.Fatalf("after selecting another, panning, zooming and turning: fastened %d, centred %v; want it held on the first",
			len(r.sys.following), r.centredOn(305, 305))
	}
	r.pressV()
	if len(r.sys.following) != 0 {
		t.Error("V again did not let the camera go")
	}
	r.cam.Pan(30, 0)
	r.ecs.Tick(time.Second / 60)
	if r.centredOn(305, 305) {
		t.Error("the camera let go still comes back to the walker")
	}
}

func TestFollow_NeedsExactlyOneSelected(t *testing.T) {
	r := newFollowRig(t)
	r.pressV()
	if len(r.sys.following) != 0 {
		t.Error("V with nothing selected fastened the camera")
	}
	r.selectOnly(r.walkers[0], r.walkers[1])
	r.pressV()
	if len(r.sys.following) != 0 {
		t.Error("V with two selected fastened the camera")
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

func TestDrive_SteersTheFastenedUnitOnlyAndStopsItWhenLetGo(t *testing.T) {
	r := newFollowRig(t)
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Ahead: 1})
	r.ecs.Tick(time.Second / 60)
	if _, ok := r.drivenOf(r.walkers[0]); ok {
		t.Fatal("a camera fastened to nothing drove a walker")
	}
	r.walk(300, 300, 1, 0)
	r.selectOnly(r.walkers[0])
	r.pressV()
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Ahead: 1})
	r.drives.Add(control.Nobody, Drive{Camera: r.cam, Turn: -1})
	r.ecs.Tick(time.Second / 60)
	if d, ok := r.drivenOf(r.walkers[0]); !ok || d != (steering.Driven{Ahead: 1, Turn: -1}) {
		t.Errorf("with Up and Left held the fastened walker is driven %+v (%v), want walking on and turning anticlockwise", d, ok)
	}
	if _, ok := r.drivenOf(r.walkers[1]); ok {
		t.Error("the other walker is driven too")
	}
	r.ecs.Tick(time.Second / 60)
	if d, _ := r.drivenOf(r.walkers[0]); d != (steering.Driven{}) {
		t.Errorf("with no key held the walker is driven %+v, want nothing asked", d)
	}
	r.pressV()
	if d, ok := r.drivenOf(r.walkers[0]); !ok || d != (steering.Driven{}) {
		t.Errorf("let go the walker is driven %+v (%v), want no hand on it once: braking, not backing away", d, ok)
	}
	r.ecs.Tick(time.Second / 60)
	if _, ok := r.drivenOf(r.walkers[0]); ok {
		t.Error("a tick after letting go the walker is still driven")
	}
}
