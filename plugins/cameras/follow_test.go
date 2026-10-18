package cameras_test

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/ui"
	"math"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

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
func (c *installCtx) Hosts(...host.Host)                              {}
func (c *installCtx) Draws(...render.Source)                          {}
func (c *installCtx) Looks(func())                                    {}

type camp struct {
	t        *testing.T
	w        *world.Plugin
	sel      *selection.Plugin
	cams     *cameras.Plugin
	players  *players.Plugin
	one, two *players.Player
	cam      camera.Camera
	wire     ui.Input
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
	c.cams = cameras.NewPlugin(c.w, cameras.Config{})
	c.players = players.NewPlugin(c.w, c.cams, c.sel)
	cam := c.cams.New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(200, 200)
	cam.MoveTo(0, 0)
	c.one, c.two = c.players.Local("one"), c.players.Add("two")
	c.wire = c.players.Through(c.one)
	c.look(cam)
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

func (c *camp) look(cam camera.Camera) {
	c.cam = cam
	c.wire.Over(geom.AABB{}, render.NewFeed(cam, nil))
}

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

func (c *camp) moveTo(id uid.UID64, x, y float64) {
	if c.q.Seek(id) {
		pos := &c.base.At(c.q.Cursor()).Pos
		pos.AABB = plane.NewAABB(geom.NewVec(x, y), pos.Size.X, pos.Size.Y)
	}
}

func centred(c *camp, x, y float64) bool {
	sx, sy := c.cam.Project(float32(x+5), float32(y+5), 0)
	return math.Abs(float64(sx-100)) < 0.5 && math.Abs(float64(sy-100)) < 0.5
}

func fastening(c *camp) camera.Fastening { return c.cam.(camera.Fastenable).Fastening() }

func twoUnits(c *camp, unit kind.Of[float64]) []kind.Entry {
	return []kind.Entry{
		unit.Entry(100).Told(players.Give{To: c.one.ID}, selection.Allow{}),
		unit.Entry(200).Told(players.Give{To: c.two.ID}, selection.Allow{}),
	}
}

func TestFollow_FastensTheCameraOverTheOneSelectedUnitAndKeepsItCentred(t *testing.T) {
	c := newCamp(t, twoUnits)
	mine := c.ids()[100]
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{mine}})
	c.tick()
	c.pressC()
	if f := fastening(c); f != (camera.Fastening{Entity: mine, How: camera.Centred}) || !centred(c, 100, 100) {
		t.Fatalf("after C: fastened %+v, centred %v; want Centred over the unit", f, centred(c, 100, 100))
	}
	c.moveTo(mine, 500, 420)
	c.tick()
	if !centred(c, 500, 420) {
		t.Error("the camera did not follow the unit to its new place")
	}
	c.cam.ZoomIn(2, 505, 425)
	c.tick()
	if fastening(c).How != camera.Centred || !centred(c, 500, 420) {
		t.Error("zooming ended the following")
	}
	c.pressC()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("after C again the camera is fastened %+v, want let go", f)
	}
}

func TestFollow_PanLetsGoAndSeveralOrAnothersSelectedFastenNone(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return append(twoUnits(c, unit), unit.Entry(300).Told(players.Give{To: c.one.ID}, selection.Allow{}))
	})
	ids := c.ids()
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100]}})
	c.w.Carrier().Put(c.two.ID, selection.Select{IDs: []uid.UID64{ids[200]}})
	c.tick()
	c.pressC()
	if f := fastening(c); f.Entity != ids[100] {
		t.Fatalf("one's camera is fastened %+v, want over its own selected unit, not two's", f)
	}
	c.w.Carrier().Put(c.one.ID, cameras.Pan{Camera: c.cam, Dx: 40})
	c.tick()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("after a Pan the camera is fastened %+v, want let go", f)
	}
	c.w.Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100], ids[300]}})
	c.tick()
	c.pressC()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("with two selected C fastened %+v, want none", f)
	}
}

func TestFollow_ToldAtSpawnFastensTheCameraUntilTheUnitIsGone(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{
			unit.Entry(100).Told(players.Give{To: c.one.ID}, cameras.Follow{Camera: c.cam, On: true}),
			unit.Entry(300).Told(cameras.Follow{}),
		}
	})
	mine := c.ids()[100]
	if f := fastening(c); f != (camera.Fastening{Entity: mine, How: camera.Centred}) || !centred(c, 100, 100) {
		t.Fatalf("one's camera is fastened %+v, centred %v; want Centred over the unit told Follow", f, centred(c, 100, 100))
	}
	c.w.Carrier().PutFrom(mine, world.Despawn{})
	c.tick()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("the unit gone, the camera is fastened %+v, want let go", f)
	}
}

func TestLookAt_CentresTheCameraOnceAndLetsGo(t *testing.T) {
	c := newCamp(t, twoUnits)
	far := c.ids()[200]
	c.moveTo(far, 700, 600)
	c.w.Carrier().Put(c.one.ID, cameras.CenterOn{Camera: c.cam, Entity: far})
	c.tick()
	if !centred(c, 700, 600) || fastening(c) != (camera.Fastening{}) {
		t.Fatalf("after LookAt: centred %v, fastened %+v; want centred and loose", centred(c, 700, 600), fastening(c))
	}
	c.moveTo(far, 300, 300)
	c.tick()
	if centred(c, 300, 300) {
		t.Error("the camera followed the entity after a LookAt")
	}
}

func TestNew_ACameraStartsFastenedToTheEntityItsConfigNames(t *testing.T) {
	c := newCamp(t, func(c *camp, unit kind.Of[float64]) []kind.Entry {
		return []kind.Entry{unit.Entry(100), unit.Entry(300).Named("scout")}
	})
	scout := c.ids()[300]
	cam := c.cams.New(cameras.FromAbove, camera.Config{Follow: "scout"})
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

// a world 1000 a side with no entities and the cameras over it, cfg theirs: start installs them
// once the cameras are made, tick runs a tick
func emptyWorld(t *testing.T, cfg cameras.Config) (w *world.Plugin, p *cameras.Plugin,
	start func(), tick func()) {
	t.Helper()
	w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
	})
	p = cameras.NewPlugin(w, cfg)
	if err := w.Carry(p); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	start = func() {
		for _, pl := range []plugin.Plugin{w, p} {
			if err := pl.Install(ctx); err != nil {
				t.Fatal(err)
			}
		}
		var systems []goke.System
		for _, produce := range ctx.pending {
			systems = append(systems, produce()...)
		}
		ctx.ecs.Setup(systems...)
		ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
			w.RunPlan(rc, d)
			w.Clock().Replay(rc, d)
			rc.Sync()
			p.RunPlan(rc, d)
		})
	}
	return w, p, start, func() { ctx.ecs.Tick(time.Second / 10) }
}

func TestSight_HasACameraSeeFartherAndNearer(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{Detail: camera.Detail{Sight: 1000, Fog: 100}})
	seeing := p.New(cameras.FromAbove, camera.Config{})
	start()
	sight := func(factor float64) float64 {
		w.Carrier().Put(command.Nobody, cameras.Sight{Camera: seeing, Factor: factor})
		tick()
		return camera.DetailOf(seeing).Sight
	}
	if got := sight(cameras.SightStep); got != 1250 {
		t.Errorf("seeing farther, the sight is %v, want 1250", got)
	}
	if got := sight(1 / cameras.SightStep); got != 1000 {
		t.Errorf("seeing nearer again, the sight is %v, want 1000", got)
	}
	if d := camera.DetailOf(seeing); d.Fog != 100 {
		t.Errorf("the fog is %v, want the Config's 100 kept", d.Fog)
	}
}

func TestCenterAt_PutsTheCameraOverThePointUnfastened(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{})
	cam := p.New(cameras.FromAbove, camera.Config{})
	start()
	cam.SetViewport(200, 200)
	cam.(camera.Fastenable).Fasten(camera.Fastening{Entity: 7, How: camera.Loose})
	w.Carrier().Put(command.Nobody, cameras.CenterAt{Camera: cam, X: 700, Y: 300})
	tick()
	if x, y, _ := cam.(camera.Focused).Focus(); x != 700 || y != 300 {
		t.Errorf("the camera's middle is at (%v, %v), want (700, 300)", x, y)
	}
	if f := cam.(camera.Fastenable).Fastening(); f.How != 0 {
		t.Errorf("the camera is fastened %+v, want to nothing", f)
	}
}
