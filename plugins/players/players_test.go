package players_test

import (
	"errors"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
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

// order and note are two command types; general is the made-up plugin defining order.
type order struct{ Cell int }
type note struct{ Text string }

type general struct{ orders control.Queue[order] }

func (g *general) Queues() []control.CommandQueue     { return []control.CommandQueue{&g.orders} }
func (g *general) DefaultBindings() []control.Binding { return nil }

// rig is a players plugin over a 1000×1000 world with one local player and a general's order queue.
type rig struct {
	t      *testing.T
	w      *world.Plugin
	p      *players.Plugin
	local  *players.Player
	orders *control.Queue[order]
}

func newRig(t *testing.T, cfg ...camera.Config) *rig {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	if len(cfg) > 0 {
		w.Res.Camera = icamera.NewFromSpaceWithConfig(1000, 1000, 0, cfg[0])
	}
	g := &general{}
	p := players.NewPlugin(w, g)
	return &rig{t: t, w: w, p: p, local: p.Local("tester"), orders: &g.orders}
}

// start installs the plugin into an ECS whose plan is players' RunPlan, so camera commands land.
func (r *rig) start() *goke.ECS {
	r.t.Helper()
	ctx := &installCtx{ecs: goke.New()}
	if err := r.p.Install(ctx); err != nil {
		r.t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { r.p.RunPlan(rc, d) })
	return ctx.ecs
}

func (r *rig) bind(b ...control.Binding) {
	r.t.Helper()
	if err := r.local.Bind(b...); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) handle(ev *control.InputEvents) { r.p.EventHandler().HandleEvents(ev) }

func (r *rig) drained() []control.Issued[order] {
	var got []control.Issued[order]
	r.orders.Drain(func(i control.Issued[order]) { got = append(got, i) })
	return got
}

func orderOf(n int) func(control.Context) (order, bool) {
	return func(control.Context) (order, bool) { return order{n}, true }
}

func TestBind_RefusesTwoBindingsOnOneTrigger(t *testing.T) {
	r := newRig(t)
	r.bind(control.Command(control.KeyPress{Key: ebiten.KeyA}, "one", orderOf(1)))
	err := r.local.Bind(control.Command(control.KeyPress{Key: ebiten.KeyA}, "two", orderOf(2)))
	if err == nil {
		t.Fatal("two bindings on KeyPress A were accepted")
	}
	if err := r.local.Bind(control.Command(control.KeyPress{Key: ebiten.KeyA, Mods: control.Mods{Shift: true}}, "shifted", orderOf(3))); err != nil {
		t.Errorf("Shift+A beside A: %v, want accepted as a different trigger", err)
	}
	if err := r.local.Bind(control.Binding{Label: "bare"}); err == nil {
		t.Error("a Binding not built with Command was accepted")
	}
}

func TestIssue_RefusesACommandNobodyListensFor(t *testing.T) {
	r := newRig(t)
	if err := r.p.Issue(r.local, note{"hi"}); !errors.Is(err, players.ErrUnknownCommand) {
		t.Errorf("Issue(note) = %v, want ErrUnknownCommand", err)
	}
	if err := r.p.Issue(nil, order{7}); err != nil {
		t.Fatalf("Issue(order) = %v", err)
	}
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 7 || got[0].Player != control.Nobody {
		t.Errorf("drained %v, want order 7 from Nobody", got)
	}
	if !r.orders.Empty() {
		t.Error("the queue is not empty after Drain")
	}
}

func TestNewPlugin_RefusesTwoHandlersOfOneType(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	defer func() {
		if recover() == nil {
			t.Error("two handlers defining order did not panic")
		}
	}()
	players.NewPlugin(w, &general{}, &general{})
}

func TestAdd_MakesAPlayerWithoutAKeyboard(t *testing.T) {
	r := newRig(t)
	bot := r.p.Add("bot")
	if bot.ID == control.Nobody || bot.ID == r.local.ID || r.p.ByID(bot.ID) != bot || r.p.ByID(control.Nobody) != nil {
		t.Errorf("bot has id %d beside local %d; ByID must find it and nobody else", bot.ID, r.local.ID)
	}
	if locals := r.p.Locals(); len(locals) != 1 || locals[0] != r.local {
		t.Errorf("Locals = %v, want the keyboard player alone", locals)
	}
	if err := r.p.Issue(bot, order{3}); err != nil {
		t.Fatal(err)
	}
	if got := r.drained(); len(got) != 1 || got[0].Player != bot.ID {
		t.Errorf("drained %v, want the bot's order", got)
	}
}

func TestDefaults_CollectEveryHandlersBindings(t *testing.T) {
	r := newRig(t)
	if got := len(r.p.Defaults()); got != len(players.CameraBindings()) {
		t.Errorf("Defaults has %d bindings, want the camera's %d (the general suggests none)", got, len(players.CameraBindings()))
	}
	if err := r.local.Bind(r.p.Defaults()...); err != nil {
		t.Error(err)
	}
}

func TestSetup_PanicsOnABindingNobodyListensFor(t *testing.T) {
	r := newRig(t)
	r.bind(control.Command(control.KeyPress{Key: ebiten.KeyN}, "unheard", func(control.Context) (note, bool) { return note{}, true }))
	ctx := &installCtx{ecs: goke.New()}
	if err := r.p.Install(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if recover() == nil {
			t.Error("Setup accepted a binding whose command nobody listens for")
		}
	}()
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
}

func TestKeysAndButtons_FireWithExactlyTheirModifiers(t *testing.T) {
	r := newRig(t)
	r.bind(
		control.Command(control.KeyPress{Key: ebiten.KeyA}, "a", orderOf(1)),
		control.Command(control.KeyPress{Key: ebiten.KeyA, Mods: control.Mods{Shift: true}}, "shift a", orderOf(2)),
		control.Command(control.ButtonPress{Button: ebiten.MouseButtonRight}, "right", func(c control.Context) (order, bool) {
			return order{int(c.Cursor.X)}, true
		}),
	)
	ev := &control.InputEvents{}
	ev.AddKeyEvent(ebiten.KeyA, control.ActionPress)
	ev.AddKeyEvent(ebiten.KeyA, control.ActionRelease)
	ev.AddClickEvent(40, 5, ebiten.MouseButtonRight, control.ActionPress)
	r.handle(ev)
	got := r.drained()
	if len(got) != 2 || got[0].Command.Cell != 1 || got[1].Command.Cell != 40 || got[0].Player != r.local.ID {
		t.Errorf("plain A and a right click at 40 issued %v, want orders 1 and 40 from the local player", got)
	}

	ev = &control.InputEvents{}
	ev.Modifiers.Shift = true
	ev.AddKeyEvent(ebiten.KeyA, control.ActionPress)
	r.handle(ev)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 2 {
		t.Errorf("Shift+A issued %v, want order 2 alone", got)
	}
}

func TestDrag_FiresOnReleaseAndButtonHeldKnowsWhereItBegan(t *testing.T) {
	r := newRig(t)
	r.bind(control.Command(control.Drag{Button: ebiten.MouseButtonLeft}, "box", func(c control.Context) (order, bool) {
		return order{int(c.Start.X)*1000 + int(c.Cursor.X)}, true
	}), control.Command(control.ButtonHeld{Button: ebiten.MouseButtonLeft}, "dragging", func(c control.Context) (order, bool) {
		return order{-(int(c.Start.X)*1000 + int(c.Cursor.X))}, true
	}))

	press := &control.InputEvents{MousePos: geom.NewVec(10, 10)}
	press.AddClickEvent(10, 10, ebiten.MouseButtonLeft, control.ActionPress)
	r.handle(press)
	if got := r.drained(); len(got) != 0 {
		t.Fatalf("a press alone issued %v", got)
	}

	r.handle(&control.InputEvents{MousePos: geom.NewVec(40, 60), CursorDelta: geom.NewVec(30, 50)})
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != -(10*1000+40) {
		t.Errorf("mid-drag issued %v, want one ButtonHeld order from 10 to 40", got)
	}

	release := &control.InputEvents{MousePos: geom.NewVec(60, 60)}
	release.AddClickEvent(60, 60, ebiten.MouseButtonLeft, control.ActionRelease)
	r.handle(release)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 10*1000+60 {
		t.Errorf("the release issued %v, want one order from 10 to 60", got)
	}
}

func TestWorldBox_StaysNarrowAcrossATorusSeam(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	w.Res.Camera = icamera.NewFromSpaceWithConfig(1000, 1000, aabbworld.Torus, camera.Config{ViewportWidth: 200, ViewportHeight: 200})
	w.Res.Camera.MoveTo(950, 500)
	ctx := control.Context{Camera: w.Res.Camera}

	box := ctx.WorldBox(geom.NewVec(0, 0), geom.NewVec(200, 10))
	if width := box.BottomRight.X - box.TopLeft.X; width > 200 {
		t.Errorf("WorldBox is %v wide across the seam, want the 200 that was dragged", width)
	}
}

// cameraRig is a rig with the default camera bindings, started, so Pan and Zoom reach the camera.
func cameraRig(t *testing.T, cfg ...camera.Config) (*rig, *goke.ECS) {
	t.Helper()
	r := newRig(t, cfg...)
	r.bind(players.CameraBindings()...)
	return r, r.start()
}

func (r *rig) move(ev *control.InputEvents, ecs *goke.ECS) {
	r.handle(ev)
	ecs.Tick(time.Second / 60)
}

func TestCamera_WheelZooms(t *testing.T) {
	r, ecs := cameraRig(t)
	cam := r.local.Camera
	r.move(&control.InputEvents{ScrollDelta: 1}, ecs)
	if cam.Zoom() <= 1 {
		t.Fatalf("Zoom() after a notch up = %v, want > 1", cam.Zoom())
	}
	zoomedIn := cam.Zoom()
	r.move(&control.InputEvents{ScrollDelta: -1}, ecs)
	if cam.Zoom() >= zoomedIn {
		t.Fatalf("Zoom() after a notch down = %v, want < %v", cam.Zoom(), zoomedIn)
	}
}

func TestCamera_MiddleDragPansOneToOneWithTheCursor(t *testing.T) {
	for _, zoom := range []float32{1, 2} {
		r, ecs := cameraRig(t, camera.Config{ViewportWidth: 200, ViewportHeight: 200})
		cam := r.local.Camera
		cam.MoveTo(400, 400)
		cam.ZoomIn(zoom, 500, 500)
		before := cam.Bounds()

		r.move(&control.InputEvents{MousePos: geom.NewVec(100, 100), MiddleDown: true, CursorDelta: geom.NewVec(10, 0)}, ecs)

		after := cam.Bounds()
		if want := before.TopLeft.X - 10/float64(zoom); after.TopLeft.X != want {
			t.Errorf("zoom %v: TopLeft.X after a 10-pixel drag = %v, want %v", zoom, after.TopLeft.X, want)
		}
	}
}

func TestCamera_EdgeScrollIsTheSameOnScreenAtAnyZoom(t *testing.T) {
	for _, zoom := range []float32{1, 2, 4} {
		r, ecs := cameraRig(t, camera.Config{ViewportWidth: 200, ViewportHeight: 200})
		cam := r.local.Camera
		cam.MoveTo(400, 400)
		cam.ZoomIn(zoom, 500, 500)
		before := cam.Bounds()

		r.move(&control.InputEvents{MousePos: geom.NewVec(190, 100)}, ecs) // near the right edge of the 200-pixel window

		moved := (cam.Bounds().TopLeft.X - before.TopLeft.X) * float64(zoom)
		if moved != float64(players.DefaultScrollSpeed) {
			t.Errorf("zoom %v: an edge scroll moved %v pixels of world, want %v", zoom, moved, players.DefaultScrollSpeed)
		}
	}
}

func TestCamera_EdgeDeadZoneAndWindowEdges(t *testing.T) {
	cases := map[string]struct {
		ev    control.InputEvents
		moves bool
	}{
		"near the right edge":               {control.InputEvents{MousePos: geom.NewVec(990, 500)}, true},
		"at the true edge":                  {control.InputEvents{MousePos: geom.NewVec(999, 500)}, false},
		"at the true edge, window fills it": {control.InputEvents{MousePos: geom.NewVec(999, 500), WindowFillsScreen: true}, true},
		"outside the window":                {control.InputEvents{MousePos: geom.NewVec(-5, 500)}, false},
		"middle drag outside the window":    {control.InputEvents{MousePos: geom.NewVec(-5, 500), MiddleDown: true, CursorDelta: geom.NewVec(10, 0)}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r, ecs := cameraRig(t)
			cam := r.local.Camera
			cam.ZoomIn(2, 500, 500)
			before := cam.Bounds()
			ev := tc.ev
			r.move(&ev, ecs)
			if moved := cam.Bounds() != before; moved != tc.moves {
				t.Errorf("camera moved = %v, want %v", moved, tc.moves)
			}
		})
	}
}

func TestPlugin_Contract(t *testing.T) {
	r := newRig(t)
	if r.p.Name() != "gram.players" {
		t.Errorf("Name = %q", r.p.Name())
	}
	if r.p.EventHandler() == nil {
		t.Error("EventHandler is nil — players translate input")
	}
	if r.p.Renderer() != nil || r.p.Serializable() != nil {
		t.Error("players draw nothing and save nothing of their own")
	}
	if err := r.p.RegisterBehavior(struct{}{}); !errors.Is(err, plugin.ErrUnhostedBehavior) {
		t.Errorf("RegisterBehavior = %v, want ErrUnhostedBehavior", err)
	}
}
