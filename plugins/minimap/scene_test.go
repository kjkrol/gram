package minimap_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
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
func (c *installCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *installCtx) ECS() *goke.ECS         { return c.ecs }
func (c *installCtx) Hosts(...host.Host)     {}
func (c *installCtx) Draws(...render.Source) {}
func (c *installCtx) Looks(func())           {}
func (*installCtx) Pool() *parallel.Pool     { return nil }

// a scene of a player's picture of a world 4000 a side and its minimap 200 a side top-right, on a
// screen 1200 by 800, the plugins installed and stepped as a game steps them
func minimapScene(t *testing.T) (scene *ui.Scene, cam camera.Camera, tick func()) {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 4000, Height: 4000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 10, MaxSize: 10},
	})
	cams := cameras.NewPlugin(w, cameras.Config{})
	mini := minimap.NewPlugin(w, nil, minimap.Config{Reach: 300})
	ps := players.NewPlugin(w, cams, mini)
	pl := ps.Local("player")
	if err := pl.Bind(cams.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	cam = cams.New(cameras.FromAbove, camera.Config{})
	scene = ui.NewScene("main", ui.Layers(
		ui.Image(render.NewFeed(cam, nil)).Input(ps.Through(pl)),
		ui.TopRight(mini.For(cam, pl.ID)).Size(200, 200),
	)).Input(ps.Handle)
	scene.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(1200, 800)))
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{w, cams, mini, ps} {
		if err := p.Install(ctx); err != nil {
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
		cams.RunPlan(rc, d)
		mini.RunPlan(rc, d)
		ps.RunPlan(rc, d)
	})
	tick = func() { ctx.ecs.Tick(time.Second / 60) }
	cam.CenterOn(2000, 2000, 0)
	tick()
	return scene, cam, tick
}

func middle(cam camera.Camera) (float32, float32) {
	x, y, _ := cam.(camera.Focused).Focus()
	return x, y
}

func TestMinimapInAScene_TheCursorAtTheEdgeOverItScrollsNothing(t *testing.T) {
	scene, cam, tick := minimapScene(t)
	at := func(x, y float64) {
		scene.HandleEvents(&control.InputEvents{MousePos: geom.NewVec(x, y),
			WindowFillsScreen: true}, nil, nil)
		tick()
	}
	x0, y0 := middle(cam)
	at(1195, 100)
	if x, y := middle(cam); x != x0 || y != y0 {
		t.Errorf("the cursor at the edge over the minimap moved the camera from (%v, %v) to "+
			"(%v, %v)", x0, y0, x, y)
	}
	at(1195, 500)
	if x, _ := middle(cam); x <= x0 {
		t.Errorf("the cursor at the edge under the minimap left the camera at x %v, want it "+
			"scrolled east of %v", x, x0)
	}
}

func TestMinimapInAScene_APressOnItTakesTheCameraThere(t *testing.T) {
	scene, cam, tick := minimapScene(t)
	x0, y0 := middle(cam)
	// 50 pixels right of the minimap's middle, a pixel three world units
	press := geom.NewVec(1150, 100)
	scene.HandleEvents(&control.InputEvents{MousePos: press, ClickQueue: []control.ClickEvent{
		{Button: control.MouseButtonLeft, Action: control.ActionPress, Pos: press}}}, nil, nil)
	tick()
	if x, y := middle(cam); x != x0+150 || y != y0 {
		t.Errorf("the camera went from (%v, %v) to (%v, %v), want 150 east", x0, y0, x, y)
	}
}
