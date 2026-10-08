package cameras_test

import (
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// installCtx is the plugin.Installer a Stage would hand over, minus the engine.
type installCtx struct {
	ecs     *goke.ECS
	pending []func() []goke.System
}

func (c *installCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *installCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *installCtx) ECS() *goke.ECS                                  { return c.ecs }
func (c *installCtx) Hosts(...plugin.Host)                            {}

// camp is a world of 10x10 units seen through a 200x200 screen, with a selection, the cameras and
// two players: one at the keyboard, through the main camera, C following, and two without.
type camp struct {
	t        *testing.T
	w        *world.Plugin
	sel      *selection.Plugin
	cams     *cameras.Plugin
	players  *players.Plugin
	one, two *players.Player
	ecs      *goke.ECS
	base     goke.Comp[world.Base]
	q        *goke.Query
}

func newCamp(t *testing.T, entries func(c *camp, unit kind.Of[float64]) []kind.Entry) *camp {
	t.Helper()
	c := &camp{t: t}
	c.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 10},
	})
	kind.Define[float64](c.w.Kinds(), "unit", kind.Spec{
		comp.Load(func(x float64) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
	unit := kind.Named[float64](c.w.Kinds(), "unit")
	c.sel = selection.NewPlugin(c.w)
	c.cams = cameras.NewPlugin(c.w)
	c.players = players.NewPlugin(c.w, c.cams, c.sel)
	cam := c.cams.New(cameras.TopDown(), camera.Config{})
	cam.SetViewport(200, 200)
	cam.MoveTo(0, 0)
	c.one, c.two = c.players.Local("one", cam), c.players.Add("two")
	if err := c.one.Bind(c.sel.FollowKey(control.KeyC)); err != nil {
		t.Fatal(err)
	}
	c.w.Seed(entries(c, unit)...)
	if err := c.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{c.w, c.sel, c.cams, c.players} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) { c.q = si.NewQueryBuilder(&c.base).Build() }})
	c.ecs = ctx.ecs
	c.ecs.Setup(systems...)
	c.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.sel.RunPlan(rc, d)
		c.w.RunPlan(rc, d)
		c.w.Clock().Replay(rc, d)
		rc.Sync()
		c.cams.RunPlan(rc, d)
		c.players.RunPlan(rc, d)
	})
	c.tick()
	return c
}

// pressC presses C at the keyboard: one follows its chosen unit, or lets go.
func (c *camp) pressC() {
	ev := &control.InputEvents{}
	ev.AddKeyEvent(control.KeyC, control.ActionPress)
	ev.AddKeyEvent(control.KeyC, control.ActionRelease)
	c.players.EventHandler().HandleEvents(ev)
	c.tick()
}

func (c *camp) tick() {
	for range 2 {
		c.ecs.Tick(time.Second / 10)
	}
}

// ids are the units by where they stand.
func (c *camp) ids() map[float64]uid.UID64 {
	out := map[float64]uid.UID64{}
	for c.q.All(); c.q.Next(); {
		cur := c.q.Cursor()
		for i, id := range cur.IDs {
			out[c.base.Slice(cur)[i].Pos.TopLeft.X] = id
		}
	}
	return out
}

// moveTo puts id's box with its top-left at (x, y).
func (c *camp) moveTo(id uid.UID64, x, y float64) {
	if c.q.Seek(id) {
		pos := &c.base.At(c.q.Cursor()).Pos
		pos.AABB = plane.NewAABB(geom.NewVec(x, y), pos.Size.X, pos.Size.Y)
	}
}

// centred reports whether pl's camera draws the middle of the 10x10 box at (x, y) in the middle
// of its screen.
func centred(pl *players.Player, x, y float64) bool {
	sx, sy := pl.Camera.Project(float32(x+5), float32(y+5), 0)
	return math.Abs(float64(sx-100)) < 0.5 && math.Abs(float64(sy-100)) < 0.5
}

func fastening(pl *players.Player) camera.Fastening { return pl.Camera.(camera.Fastenable).Fastening() }

// twoUnits is a unit of player one at 100 and one of player two at 200, both selectable.
func twoUnits(c *camp, unit kind.Of[float64]) []kind.Entry {
	return []kind.Entry{
		unit.Entry(100).Told(players.Give{To: c.one.ID}, selection.Allow{}),
		unit.Entry(200).Told(players.Give{To: c.two.ID}, selection.Allow{}),
	}
}

// C fastens the player's camera Centred over the one unit it has selected and keeps it in the
// middle of the screen as it goes; zooming keeps it there; C again lets go.
func TestFollow_FastensTheCameraOverTheOneSelectedUnitAndKeepsItCentred(t *testing.T) {
	c := newCamp(t, twoUnits)
	mine := c.ids()[100]
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{mine}})
	c.tick()
	c.pressC()
	if f := fastening(c.one); f != (camera.Fastening{Entity: mine, How: camera.Centred}) || !centred(c.one, 100, 100) {
		t.Fatalf("after C: fastened %+v, centred %v; want Centred over the unit", f, centred(c.one, 100, 100))
	}
	c.moveTo(mine, 500, 420)
	c.tick()
	if !centred(c.one, 500, 420) {
		t.Error("the camera did not follow the unit to its new place")
	}
	c.one.Camera.ZoomIn(2, 505, 425)
	c.tick()
	if fastening(c.one).How != camera.Centred || !centred(c.one, 500, 420) {
		t.Error("zooming ended the following")
	}
	c.pressC()
	if f := fastening(c.one); f != (camera.Fastening{}) {
		t.Errorf("after C again the camera is fastened %+v, want let go", f)
	}
}

// A Pan lets the camera go; with several units selected, or another player's, C fastens to none.
func TestFollow_PanLetsGoAndSeveralOrAnothersSelectedFastenNone(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return append(twoUnits(c, unit), unit.Entry(300).Told(players.Give{To: c.one.ID}, selection.Allow{}))
	})
	ids := c.ids()
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100]}})
	c.w.Carrier().Put(c.two.ID, selection.Select{IDs: []uid.UID64{ids[200]}})
	c.tick()
	c.pressC()
	if f := fastening(c.one); f.Entity != ids[100] {
		t.Fatalf("one's camera is fastened %+v, want over its own selected unit, not two's", f)
	}
	c.w.Carrier().Put(c.one.ID, cameras.Pan{Camera: c.one.Camera, Dx: 40})
	c.tick()
	if f := fastening(c.one); f != (camera.Fastening{}) {
		t.Errorf("after a Pan the camera is fastened %+v, want let go", f)
	}
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100], ids[300]}})
	c.tick()
	c.pressC()
	if f := fastening(c.one); f != (camera.Fastening{}) {
		t.Errorf("with two selected C fastened %+v, want none", f)
	}
}

// A unit told Follow as it is made has the camera it names fastened over it from its first step;
// a Follow naming no camera does nothing; the camera lets go once the unit is gone.
func TestFollow_ToldAtSpawnFastensTheCameraUntilTheUnitIsGone(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{
			unit.Entry(100).Told(players.Give{To: c.one.ID}, cameras.Follow{Camera: c.one.Camera, On: true}),
			unit.Entry(300).Told(cameras.Follow{}),
		}
	})
	mine := c.ids()[100]
	if f := fastening(c.one); f != (camera.Fastening{Entity: mine, How: camera.Centred}) || !centred(c.one, 100, 100) {
		t.Fatalf("one's camera is fastened %+v, centred %v; want Centred over the unit told Follow", f, centred(c.one, 100, 100))
	}
	c.w.Carrier().PutFrom(mine, world.Despawn{})
	c.tick()
	if f := fastening(c.one); f != (camera.Fastening{}) {
		t.Errorf("the unit gone, the camera is fastened %+v, want let go", f)
	}
}

// LookAt centres the camera on the entity once, fastened to nothing: the entity moving on leaves it.
func TestLookAt_CentresTheCameraOnceAndLetsGo(t *testing.T) {
	c := newCamp(t, twoUnits)
	far := c.ids()[200]
	c.moveTo(far, 700, 600)
	c.w.Carrier().Put(c.one.ID, cameras.LookAt{Camera: c.one.Camera, Entity: far})
	c.tick()
	if !centred(c.one, 700, 600) || fastening(c.one) != (camera.Fastening{}) {
		t.Fatalf("after LookAt: centred %v, fastened %+v; want centred and loose", centred(c.one, 700, 600), fastening(c.one))
	}
	c.moveTo(far, 300, 300)
	c.tick()
	if centred(c.one, 300, 300) {
		t.Error("the camera followed the entity after a LookAt")
	}
}

// A camera whose config follows an entity by name starts fastened Centred over it once it is in
// the world, and keeps it in the middle as it goes.
func TestNew_ACameraStartsFastenedToTheEntityItsConfigNames(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{unit.Entry(100), unit.Entry(300).Named("scout")}
	})
	scout := c.ids()[300]
	cam := c.cams.New(cameras.TopDown(), camera.Config{Follow: "scout"})
	cam.SetViewport(200, 200)
	c.tick()
	if f := cam.(camera.Fastenable).Fastening(); f != (camera.Fastening{Entity: scout, How: camera.Centred}) {
		t.Fatalf("the camera is fastened %+v, want Centred over the scout", f)
	}
	c.moveTo(scout, 600, 500)
	c.tick()
	if sx, sy := cam.Project(605, 505, 0); math.Abs(float64(sx-100)) > 0.5 || math.Abs(float64(sy-100)) > 0.5 {
		t.Errorf("the scout is drawn at (%v, %v), want the middle of the screen", sx, sy)
	}
}
