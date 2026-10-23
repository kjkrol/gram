package cameras_test

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/internal/parallel"
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
func (*installCtx) Pool() *parallel.Pool                              { return nil }

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
	cam := c.cams.Write().New(cameras.FromAbove, camera.Config{})
	cam.SetViewport(200, 200)
	cam.MoveTo(0, 0)
	c.one, c.two = c.players.Local("one"), c.players.Add("two")
	c.wire = c.players.Through(c.one)
	c.look(cam)
	if err := c.one.Bind(c.sel.FollowKey(control.KeyC)); err != nil {
		t.Fatal(err)
	}
	c.w.Write().Seed(entries(c, unit)...)
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
		c.w.Write().Clock().Replay(rc, d)
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
	c.w.Write().Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{mine}})
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
	c.w.Write().Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100]}})
	c.w.Write().Carrier().Put(c.two.ID, selection.Select{IDs: []uid.UID64{ids[200]}})
	c.tick()
	c.pressC()
	if f := fastening(c); f.Entity != ids[100] {
		t.Fatalf("one's camera is fastened %+v, want over its own selected unit, not two's", f)
	}
	c.w.Write().Carrier().Put(c.one.ID, cameras.Pan{Camera: c.cam, Dx: 40})
	c.tick()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("after a Pan the camera is fastened %+v, want let go", f)
	}
	c.w.Write().Carrier().Put(c.one.ID, selection.Select{IDs: []uid.UID64{ids[100], ids[300]}})
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
	c.w.Write().Carrier().PutFrom(mine, world.Despawn{})
	c.tick()
	if f := fastening(c); f != (camera.Fastening{}) {
		t.Errorf("the unit gone, the camera is fastened %+v, want let go", f)
	}
}

func TestLookAt_CentresTheCameraOnceAndLetsGo(t *testing.T) {
	c := newCamp(t, twoUnits)
	far := c.ids()[200]
	c.moveTo(far, 700, 600)
	c.w.Write().Carrier().Put(c.one.ID, cameras.CenterOn{Camera: c.cam, Entity: far})
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
	cam := c.cams.Write().New(cameras.FromAbove, camera.Config{Follow: "scout"})
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
			w.Write().Clock().Replay(rc, d)
			rc.Sync()
			p.RunPlan(rc, d)
		})
	}
	return w, p, start, func() { ctx.ecs.Tick(time.Second / 10) }
}

func TestSight_HasACameraSeeFartherAndNearer(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{Detail: camera.Detail{Sight: 1000, Fog: 100}})
	seeing := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	sight := func(factor float64) float64 {
		w.Write().Carrier().Put(command.Nobody, cameras.Sight{Camera: seeing, Factor: factor})
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
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	cam.SetViewport(200, 200)
	cam.(camera.Fastenable).Fasten(camera.Fastening{Entity: 7, How: camera.Loose})
	w.Write().Carrier().Put(command.Nobody, cameras.CenterAt{Camera: cam, X: 700, Y: 300})
	tick()
	if x, y, _ := cam.(camera.Focused).Focus(); x != 700 || y != 300 {
		t.Errorf("the camera's middle is at (%v, %v), want (700, 300)", x, y)
	}
	if f := cam.(camera.Fastenable).Fastening(); f.How != 0 {
		t.Errorf("the camera is fastened %+v, want to nothing", f)
	}
}

func TestResolution_StepsTheCamerasResolutionAndAdaptStopsOrStartsFollowingTheBudget(t *testing.T) {
	w, p, start, tick := emptyWorld(t,
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	minimap := p.Write().New(cameras.FromAbove, camera.Config{Whole: true})
	start()
	if got := report(p); got != "Picture: tactical\n1.00 of the pixels\nfollowing a budget of "+
		"60 FPS" {
		t.Fatalf("the telemetry says %q, want the one picture at all its pixels, following", got)
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: cam,
		Factor: cameras.ResolutionStep})
	tick()
	if got := camera.ResolutionOf(cam); got != cameras.ResolutionStep {
		t.Errorf("after a coarser step the camera is drawn at %v, want %v", got,
			cameras.ResolutionStep)
	}
	if got := report(p); got != "Picture: tactical\n0.85 of the pixels\nthe figures sharp\nstill" {
		t.Errorf("the telemetry says %q, want the step and the following stopped", got)
	}
	if p.Read(0).Strategy(cameras.Tactical) != cameras.Still ||
		p.Read(0).Resolution(cameras.Tactical) != cameras.ResolutionStep {
		t.Errorf("after the key the strategy is %v at %v, want Still, the Settings noting 0.85",
			p.Read(0).Strategy(cameras.Tactical), p.Read(0).Resolution(cameras.Tactical))
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: cam, Factor: 1 / cameras.ResolutionStep})
	tick()
	if got := camera.ResolutionOf(cam); math.Abs(float64(got-1)) > 1e-6 {
		t.Errorf("after a finer step back the camera is drawn at %v, want 1", got)
	}
	for range 10 {
		w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: cam,
			Factor: cameras.ResolutionStep})
		tick()
	}
	if got := camera.ResolutionOf(cam); got != cameras.LeastResolution {
		t.Errorf("stepped coarser ten times the camera is drawn at %v, want the least, %v", got,
			cameras.LeastResolution)
	}
	w.Write().Carrier().Put(command.Nobody,
		cameras.Adapt{Profile: cameras.Tactical, Strategy: cameras.Uniform})
	tick()
	if got := report(p); got != "Picture: tactical\n0.40 of the pixels\nthe figures sharp\n"+
		"following a budget of 60 FPS" {
		t.Errorf("after Adapt the telemetry says %q, want the resolution following the budget", got)
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: minimap, Factor: cameras.ResolutionStep})
	tick()
	if got := camera.ResolutionOf(minimap); got != 1 {
		t.Errorf("a minimap's camera stepped coarser is drawn at %v, want all its pixels", got)
	}
}

// the cameras' line of the telemetry panel
func report(p *cameras.Plugin) string {
	var got string
	p.Read(0).Reporter().Report(func(label, value string) { got = label + ": " + value })
	return got
}

func TestAdapt_SwitchesStillTheWholePictureAndInLayers(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	for steps, want := range []struct {
		layers   int
		strategy cameras.Strategy
		report   string
	}{
		{0, cameras.Still, "Picture: tactical\n1.00 of the pixels\nstill"},
		{0, cameras.Uniform, "Picture: tactical\n1.00 of the pixels\nfollowing a budget of 60 FPS"},
		{3, cameras.InLayers, "Picture: tactical\n0.60/0.80/1.00 of the pixels\nin layers at 60% " +
			"and 35%\nfollowing a budget of 60 FPS"},
		{3, cameras.Still, "Picture: tactical\n0.60/0.80/1.00 of the pixels\nin layers at 60% " +
			"and 35%\nstill"},
		{0, cameras.Uniform, "Picture: tactical\n1.00 of the pixels\nfollowing a budget of 60 FPS"},
	} {
		if steps > 0 {
			w.Write().Carrier().Put(command.Nobody,
				cameras.Adapt{Profile: cameras.Tactical, Strategy: want.strategy})
			tick()
		}
		layers := len(camera.LayersOf(cam))
		on, strategy := p.Read(0).Layers(cameras.Tactical), p.Read(0).Strategy(cameras.Tactical)
		if layers != want.layers || on != (want.layers > 0) || strategy != want.strategy {
			t.Errorf("after %d Adapt the camera has %d layers, the plugin layers %v, strategy %v; "+
				"want %d layers and %v", steps, layers, on, strategy, want.layers, want.strategy)
		}
		if got := report(p); got != want.report {
			t.Errorf("after %d Adapt the telemetry says %q, want %q", steps, got, want.report)
		}
	}
}

func TestResolution_InLayersStepsTheOuterRingFirstAndStopsTheFollowing(t *testing.T) {
	w, p, start, tick := emptyWorld(t,
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.InLayers}})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	if got := report(p); got != "Picture: tactical\n0.60/0.80/1.00 of the pixels\nin layers at "+
		"60% and 35%\nfollowing a budget of 60 FPS" {
		t.Fatalf("the telemetry says %q, want the default layers following the budget", got)
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: cam,
		Factor: cameras.ResolutionStep})
	tick()
	if got := report(p); got != "Picture: tactical\n0.51/0.80/1.00 of the pixels\nin layers at "+
		"60% and 35%\nstill" {
		t.Errorf("the telemetry says %q, want the outer layer stepped and the following stopped",
			got)
	}
	layers := camera.LayersOf(cam)
	if len(layers) != 3 || camera.ResolutionOf(cam) != layers[2].Resolution {
		t.Errorf("the camera is drawn at %v in the layers %v, want the centre layer's resolution",
			camera.ResolutionOf(cam), layers)
	}
	if outer, _ := p.Read(0).LayerResolutions(cameras.Tactical); outer != layers[0].Resolution {
		t.Errorf("the Settings say the outer layer is at %v, want the camera's %v", outer,
			layers[0].Resolution)
	}
}

func TestAdapt_FromLayersStillGoesOnFollowingInTheLayersAsTheyAre(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{})
	cam := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	w.Write().Carrier().Put(command.Nobody, cameras.Layers{Profile: cameras.Tactical, On: true})
	tick()
	w.Write().Carrier().Put(command.Nobody, cameras.Resolution{Camera: cam,
		Factor: cameras.ResolutionStep})
	tick()
	if got := report(p); got != "Picture: tactical\n0.51/0.80/1.00 of the pixels\nin layers at "+
		"60% and 35%\nstill" {
		t.Fatalf("the telemetry says %q, want the picture still, in layers, the outer one stepped",
			got)
	}
	w.Write().Carrier().Put(command.Nobody,
		cameras.Adapt{Profile: cameras.Tactical, Strategy: cameras.InLayers})
	tick()
	if got := report(p); got != "Picture: tactical\n0.51/0.80/1.00 of the pixels\nin layers at "+
		"60% and 35%\nfollowing a budget of 60 FPS" {
		t.Errorf("after Adapt the telemetry says %q, want the following resumed in the layers as "+
			"they are, not the default again", got)
	}
	w.Write().Carrier().Put(command.Nobody, cameras.Layers{Profile: cameras.Tactical, On: false})
	tick()
	if p.Read(0).Strategy(cameras.Tactical) != cameras.Still || len(camera.LayersOf(cam)) != 0 {
		t.Errorf("the layers switched off under InLayers leave the strategy %v and %d layers, want "+
			"Still and none", p.Read(0).Strategy(cameras.Tactical), len(camera.LayersOf(cam)))
	}
}

func TestPicture_ReachesEveryCameraButAMinimapsAndStopsTheFollowing(t *testing.T) {
	w, p, start, tick := emptyWorld(t,
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.Uniform}})
	one := p.Write().New(cameras.FromAbove, camera.Config{})
	two := p.Write().New(cameras.FromAbove, camera.Config{})
	minimap := p.Write().New(cameras.FromAbove, camera.Config{Whole: true})
	start()
	w.Write().Carrier().Put(command.Nobody, cameras.Picture{Profile: cameras.Tactical,
		Resolution: 0.7})
	tick()
	for i, cam := range []camera.Camera{one, two} {
		if got := camera.ResolutionOf(cam); got != 0.7 {
			t.Errorf("camera %d is drawn at %v, want 0.7", i, got)
		}
	}
	if got := camera.ResolutionOf(minimap); got != 1 {
		t.Errorf("the minimap is drawn at %v, want all its pixels", got)
	}
	rd := p.Read(0)
	if s, r := rd.Strategy(cameras.Tactical), rd.Resolution(cameras.Tactical); s != cameras.Still ||
		r != 0.7 {
		t.Errorf("after Picture the strategy is %v at %v, want Still at 0.7", s, r)
	}
	if got := report(p); got != "Picture: tactical\n0.70, 0.70 of the pixels\nthe figures sharp\n"+
		"still" {
		t.Errorf("the telemetry says %q, want both pictures at 0.70, still", got)
	}
}

func TestLayerShares_ShapeEveryCamerasLayersAndTheMiddleStaysOverTheInner(t *testing.T) {
	w, p, start, tick := emptyWorld(t, cameras.Config{Tactical: cameras.Knobs{Layers: true}})
	one := p.Write().New(cameras.FromAbove, camera.Config{})
	two := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	w.Write().Carrier().Put(command.Nobody,
		cameras.LayerShares{Profile: cameras.Tactical, Inner: 0.3, Middle: 0.5})
	tick()
	for i, cam := range []camera.Camera{one, two} {
		layers := camera.LayersOf(cam)
		if len(layers) != 3 || layers[1].Share != 0.5 || layers[2].Share != 0.3 {
			t.Errorf("camera %d is drawn in %v, want the middle layer at 50%% and the inner at 30%%",
				i, layers)
		}
	}
	if got := report(p); got != "Picture: tactical\n0.60/0.80/1.00, 0.60/0.80/1.00 of the pixels"+
		"\nin layers at 50% and 30%\nstill" {
		t.Errorf("the telemetry says %q, want the shares set", got)
	}
	w.Write().Carrier().Put(command.Nobody,
		cameras.LayerShares{Profile: cameras.Tactical, Inner: 0.5, Middle: 0.5})
	tick()
	if inner, middle := p.Read(0).LayerShares(cameras.Tactical); inner != 0.5 ||
		math.Abs(float64(middle-0.6)) > 1e-6 {
		t.Errorf("the shares are %v and %v, want the middle a tenth over the inner, 0.5 and 0.6",
			inner, middle)
	}
	w.Write().Carrier().Put(command.Nobody,
		cameras.LayerResolutions{Profile: cameras.Tactical, Outer: 0.5, Middle: 0.7})
	tick()
	if layers := camera.LayersOf(one); layers[0].Resolution != 0.5 || layers[1].Resolution != 0.7 {
		t.Errorf("the camera is drawn in %v, want the outer layer at 0.5 and the middle at 0.7",
			layers)
	}
}

func TestRedraw_NotesTheLeadCamerasResolutionsIntoTheSettings(t *testing.T) {
	_, p, start, _ := emptyWorld(t,
		cameras.Config{Tactical: cameras.Knobs{Strategy: cameras.InLayers}})
	minimap := p.Write().New(cameras.FromAbove, camera.Config{Whole: true})
	lead := p.Write().New(cameras.FromAbove, camera.Config{})
	start()
	p.Redraw(cameras.ResolutionStep)
	outer, middle := p.Read(0).LayerResolutions(cameras.Tactical)
	centre := p.Read(0).Resolution(cameras.Tactical)
	if math.Abs(float64(outer-0.51)) > 1e-6 || middle != 0.8 || centre != 1 {
		t.Errorf("after a coarser step the Settings say %v/%v/%v, want 0.51/0.8/1: the lead "+
			"camera's layers", outer, middle, centre)
	}
	if got := camera.LayersOf(minimap); len(got) != 0 || camera.ResolutionOf(minimap) != 1 {
		t.Errorf("the minimap is drawn in %v at %v, want whole at all its pixels", got,
			camera.ResolutionOf(minimap))
	}
	if got := camera.LayersOf(lead); math.Abs(float64(got[0].Resolution-0.51)) > 1e-6 {
		t.Errorf("the lead camera is drawn in %v, want its outer layer at 0.51", got)
	}
	p.Write().SetStrategy(cameras.Tactical, cameras.Uniform)
	p.Redraw(cameras.ResolutionStep)
	if got := p.Read(0).Resolution(cameras.Tactical); got != cameras.ResolutionStep {
		t.Errorf("under Uniform a coarser step leaves the Settings at %v, want 0.85", got)
	}
	if got := camera.ResolutionOf(lead); got != cameras.ResolutionStep {
		t.Errorf("under Uniform a coarser step draws the lead camera at %v, want 0.85", got)
	}
}
