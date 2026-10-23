package players_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

type installCtx struct {
	hosts   []host.Host
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

type order struct{ Cell int }

func (order) Command() {}

type note struct{ Text string }

func (note) Command() {}

type general struct{ orders command.Queue[order] }

func (g *general) Queues() []command.CommandQueue     { return []command.CommandQueue{&g.orders} }
func (g *general) DefaultBindings() []control.Binding { return nil }

type rig struct {
	t      *testing.T
	w      *world.Plugin
	cams   *cameras.Plugin
	cam    camera.Camera
	p      *players.Plugin
	local  *players.Player
	wire   ui.Input
	orders *command.Queue[order]
}

func (r *rig) look(cam camera.Camera) {
	r.cam = cam
	r.wire.Over(geom.AABB{}, render.NewFeed(cam, nil))
}

func newRig(t *testing.T) *rig {
	t.Helper()
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	cams := cameras.NewPlugin(w, cameras.Config{})
	cam := cams.Write().New(cameras.FromAbove, camera.Config{})
	g := &general{}
	p := players.NewPlugin(w, cams, g)
	local := p.Local("tester")
	r := &rig{t: t, w: w, cams: cams, p: p, local: local, wire: p.Through(local), orders: &g.orders}
	r.look(cam)
	return r
}

func (r *rig) start() *goke.ECS {
	r.t.Helper()
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{r.cams, r.p} {
		if err := p.Install(ctx); err != nil {
			r.t.Fatal(err)
		}
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		r.cams.RunPlan(rc, d)
		r.p.RunPlan(rc, d)
	})
	return ctx.ecs
}

func (r *rig) bind(b ...control.Binding) {
	r.t.Helper()
	if err := r.local.Bind(b...); err != nil {
		r.t.Fatal(err)
	}
}

func (r *rig) handle(ev *control.InputEvents) { r.p.EventHandler().HandleEvents(ev) }

func (r *rig) drained() []command.Issued[order] {
	var got []command.Issued[order]
	r.orders.Drain(func(i command.Issued[order]) { got = append(got, i) })
	return got
}

func orderOf(n int) func(control.Context) (order, bool) {
	return func(control.Context) (order, bool) { return order{n}, true }
}

func TestBind_RefusesTwoBindingsOnOneTrigger(t *testing.T) {
	r := newRig(t)
	r.bind(control.Build(control.KeyPress{Key: control.KeyA}, "one", orderOf(1)))
	err := r.local.Bind(control.Build(control.KeyPress{Key: control.KeyA}, "two", orderOf(2)))
	if err == nil {
		t.Fatal("two bindings on KeyPress A were accepted")
	}
	if err := r.local.Bind(control.Build(control.KeyPress{Key: control.KeyA, Mods: control.Mods{Shift: true}}, "shifted", orderOf(3))); err != nil {
		t.Errorf("Shift+A beside A: %v, want accepted as a different rule", err)
	}
	if err := r.local.Bind(control.Binding{Label: "bare"}); err == nil {
		t.Error("a Binding not built with Command was accepted")
	}
}

func TestTool_ItsBindingsTakeTheirTriggersWhileHeld(t *testing.T) {
	r := newRig(t)
	click := control.ButtonPress{Button: control.MouseButtonLeft}
	r.bind(
		control.Build(click, "select", orderOf(1)),
		control.Build(click, "a bridge's end", orderOf(2)).With("bridge"),
		control.Build(control.KeyPress{Key: control.KeyA}, "anything", orderOf(3)),
	)
	if err := r.local.Bind(control.Build(click, "again", orderOf(4)).With("bridge")); err == nil {
		t.Error("two bindings of one tool on one trigger were accepted")
	}
	pass := func() []int {
		ev := &control.InputEvents{}
		ev.AddClickEvent(5, 5, control.MouseButtonLeft, control.ActionPress)
		ev.AddClickEvent(5, 5, control.MouseButtonLeft, control.ActionRelease)
		ev.AddKeyEvent(control.KeyA, control.ActionPress)
		ev.AddKeyEvent(control.KeyA, control.ActionRelease)
		r.handle(ev)
		var cells []int
		for _, i := range r.drained() {
			cells = append(cells, i.Command.Cell)
		}
		slices.Sort(cells)
		return cells
	}
	if got := pass(); !slices.Equal(got, []int{1, 3}) {
		t.Errorf("holding nothing a click and A issued orders %v, want 1 and 3", got)
	}
	if err := r.p.Write().Issue(r.local, control.Take{Tool: "bridge"}); err != nil {
		t.Fatal(err)
	}
	if got := pass(); !slices.Equal(got, []int{2, 3}) {
		t.Errorf("holding the bridge a click and A issued orders %v, want 2 and 3", got)
	}
	if err := r.p.Write().Issue(r.local, control.Drop{}); err != nil {
		t.Fatal(err)
	}
	if got := pass(); !slices.Equal(got, []int{1, 3}) {
		t.Errorf("put down, a click and A issued orders %v, want 1 and 3 again", got)
	}
}

func TestIssue_RefusesACommandNobodyListensFor(t *testing.T) {
	r := newRig(t)
	if err := r.p.Write().Issue(r.local, note{"hi"}); !errors.Is(err, players.ErrUnknownCommand) {
		t.Errorf("Issue(note) = %v, want ErrUnknownCommand", err)
	}
	if err := r.p.Write().Issue(nil, order{7}); err != nil {
		t.Fatalf("Issue(order) = %v", err)
	}
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 7 || got[0].Issuer != command.Nobody {
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
	if bot.ID == command.Nobody || bot.ID == r.local.ID || r.p.Read(0).ByID(bot.ID) != bot || r.p.Read(0).ByID(command.Nobody) != nil {
		t.Errorf("bot has id %d beside local %d; ByID must find it and nobody else", bot.ID, r.local.ID)
	}
	if locals := r.p.Read(0).Locals(); len(locals) != 1 || locals[0] != r.local {
		t.Errorf("Locals = %v, want the keyboard player alone", locals)
	}
	if err := r.p.Write().Issue(bot, order{3}); err != nil {
		t.Fatal(err)
	}
	if got := r.drained(); len(got) != 1 || got[0].Issuer != bot.ID {
		t.Errorf("drained %v, want the bot's order", got)
	}
}

func TestDefaults_CollectEveryHandlersBindings(t *testing.T) {
	r := newRig(t)
	if got, want := len(r.p.Defaults()), len(r.p.DefaultBindings())+len(r.w.DefaultBindings())+len(r.cams.DefaultBindings()); got != want {
		t.Errorf("Defaults has %d bindings, want the players' own, the world's clock's and the cameras' %d (the general suggests none)", got, want)
	}
	if err := r.local.Bind(r.p.Defaults()...); err != nil {
		t.Error(err)
	}
}

func TestSetup_PanicsOnABindingNobodyListensFor(t *testing.T) {
	r := newRig(t)
	r.bind(control.Build(control.KeyPress{Key: control.KeyN}, "unheard", func(control.Context) (note, bool) { return note{}, true }))
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
		control.Build(control.KeyPress{Key: control.KeyA}, "a", orderOf(1)),
		control.Build(control.KeyPress{Key: control.KeyA, Mods: control.Mods{Shift: true}}, "shift a", orderOf(2)),
		control.Build(control.ButtonPress{Button: control.MouseButtonRight}, "right", func(c control.Context) (order, bool) {
			return order{int(c.Cursor.X)}, true
		}),
	)
	ev := &control.InputEvents{}
	ev.AddKeyEvent(control.KeyA, control.ActionPress)
	ev.AddKeyEvent(control.KeyA, control.ActionRelease)
	ev.AddClickEvent(40, 5, control.MouseButtonRight, control.ActionPress)
	r.handle(ev)
	got := r.drained()
	if len(got) != 2 || got[0].Command.Cell != 1 || got[1].Command.Cell != 40 || got[0].Issuer != r.local.ID {
		t.Errorf("plain A and a right click at 40 issued %v, want orders 1 and 40 from the local player", got)
	}

	ev = &control.InputEvents{}
	ev.Modifiers.Shift = true
	ev.AddKeyEvent(control.KeyA, control.ActionPress)
	r.handle(ev)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 2 {
		t.Errorf("Shift+A issued %v, want order 2 alone", got)
	}
}

func TestKeyDown_FiresWithAModifierHeldOnlyInChords(t *testing.T) {
	r := newRig(t)
	r.bind(
		control.Build(control.KeyDown{Key: control.KeySpace}, "jump", orderOf(1)).InChords(),
		control.Build(control.KeyPress{Key: control.KeyA}, "a", orderOf(2)),
		control.Build(control.KeyDown{Key: control.KeyJ}, "j", orderOf(3)),
		control.Build(control.KeyDown{Key: control.KeyShift}, "shift", orderOf(4)),
	)
	ev := &control.InputEvents{}
	ev.Modifiers.Shift = true
	ev.AddKeyEvent(control.KeyShift, control.ActionPress)
	ev.AddKeyEvent(control.KeySpace, control.ActionPress)
	ev.AddKeyEvent(control.KeyA, control.ActionPress)
	ev.AddKeyEvent(control.KeyJ, control.ActionPress)
	r.handle(ev)
	if got := cellsOf(r.drained()); !slices.Equal(got, []int{1, 4}) {
		t.Errorf("Shift, Space, A and J with Shift held issued %v, want the jump in chords and "+
			"Shift's own", got)
	}
	ev = &control.InputEvents{}
	ev.AddKeyEvent(control.KeySpace, control.ActionPress)
	ev.AddKeyEvent(control.KeyJ, control.ActionPress)
	r.handle(ev)
	if got := cellsOf(r.drained()); !slices.Equal(got, []int{1, 3}) {
		t.Errorf("Space and J alone issued %v, want both", got)
	}
}

// a key held for scrolling leaves Shift+W to its shortcut; a walk in chords sprints
func TestKeyHeld_LeavesChordsToTheShortcutsUnlessInChords(t *testing.T) {
	r := newRig(t)
	r.bind(
		control.Build(control.KeyHeld{Key: control.KeyW}, "scroll", orderOf(1)),
		control.Build(control.KeyHeld{Key: control.KeyD}, "walk",
			func(c control.Context) (order, bool) {
				if c.Mods.Shift {
					return order{20}, true
				}
				return order{2}, true
			}).InChords(),
		control.Build(control.KeyPress{Key: control.KeyW, Mods: control.Mods{Shift: true}},
			"weather", orderOf(3)),
	)
	ecs := r.start()
	key := func(k control.Key, a control.KeyAction, shift bool) {
		ev := &control.InputEvents{KeyEvents: []control.KeyEvent{{Key: k, Action: a}}}
		ev.Modifiers.Shift = shift
		r.handle(ev)
	}
	tick := func() []int {
		ecs.Tick(time.Second / 60)
		return cellsOf(r.drained())
	}
	key(control.KeyShift, control.ActionPress, true)
	key(control.KeyW, control.ActionPress, true)
	key(control.KeyD, control.ActionPress, true)
	if got := tick(); !slices.Equal(got, []int{3, 20}) {
		t.Errorf("Shift+W and D held with Shift gave %v, want the shortcut and the sprint, "+
			"no scroll", got)
	}
	key(control.KeyShift, control.ActionRelease, false)
	if got := tick(); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("Shift let go, W and D still held, gave %v, want the scroll and the walk", got)
	}
}

func cellsOf(orders []command.Issued[order]) []int {
	var got []int
	for _, o := range orders {
		got = append(got, o.Command.Cell)
	}
	slices.Sort(got)
	return got
}

func TestDrag_FiresOnReleaseAndButtonHeldKnowsWhereItBegan(t *testing.T) {
	r := newRig(t)
	r.bind(control.Build(control.Drag{Button: control.MouseButtonLeft}, "box", func(c control.Context) (order, bool) {
		return order{int(c.Start.X)*1000 + int(c.Cursor.X)}, true
	}), control.Build(control.ButtonHeld{Button: control.MouseButtonLeft}, "dragging", func(c control.Context) (order, bool) {
		return order{-(int(c.Start.X)*1000 + int(c.Cursor.X))}, true
	}))

	press := &control.InputEvents{MousePos: geom.NewVec(10, 10)}
	press.AddClickEvent(10, 10, control.MouseButtonLeft, control.ActionPress)
	r.handle(press)
	if got := r.drained(); len(got) != 0 {
		t.Fatalf("a press alone issued %v", got)
	}

	r.handle(&control.InputEvents{MousePos: geom.NewVec(40, 60), CursorDelta: geom.NewVec(30, 50)})
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != -(10*1000+40) {
		t.Errorf("mid-drag issued %v, want one ButtonHeld order from 10 to 40", got)
	}

	release := &control.InputEvents{MousePos: geom.NewVec(60, 60)}
	release.AddClickEvent(60, 60, control.MouseButtonLeft, control.ActionRelease)
	r.handle(release)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 10*1000+60 {
		t.Errorf("the release issued %v, want one order from 10 to 60", got)
	}
}

func TestWorldBox_StaysNarrowAcrossATorusSeam(t *testing.T) {
	cam := cameratest.NewFromSpace(1000, 1000, aabbworld.Torus)
	cam.SetViewport(200, 200)
	cam.MoveTo(950, 500)
	ctx := control.Context{Camera: cam}

	box := ctx.WorldBox(geom.NewVec(0, 0), geom.NewVec(200, 10))
	if width := box.BottomRight.X - box.TopLeft.X; width > 200 {
		t.Errorf("WorldBox is %v wide across the seam, want the 200 that was dragged", width)
	}
}

func cameraRig(t *testing.T) (*rig, *goke.ECS) {
	t.Helper()
	r := newRig(t)
	r.bind(cameras.DefaultKeys().Bindings()...)
	return r, r.start()
}

func (r *rig) move(ev *control.InputEvents, ecs *goke.ECS) {
	r.handle(ev)
	ecs.Tick(time.Second / 60)
}

func TestCamera_WheelZooms(t *testing.T) {
	r, ecs := cameraRig(t)
	cam := r.cam
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
		r, ecs := cameraRig(t)
		cam := r.cam
		cam.SetViewport(200, 200)
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
		r, ecs := cameraRig(t)
		cam := r.cam
		cam.SetViewport(200, 200)
		cam.MoveTo(400, 400)
		cam.ZoomIn(zoom, 500, 500)
		before := cam.Bounds()

		// near the right edge of the 200-pixel window
		r.move(&control.InputEvents{MousePos: geom.NewVec(190, 100)}, ecs)

		moved := (cam.Bounds().TopLeft.X - before.TopLeft.X) * float64(zoom)
		if moved != float64(cameras.DefaultScrollSpeed) {
			t.Errorf("zoom %v: an edge scroll moved %v pixels of world, want %v", zoom, moved, cameras.DefaultScrollSpeed)
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
			cam := r.cam
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
	if r.p.Serializable() == nil {
		t.Error("players save their Settings")
	}
}

type riding struct {
	camera.Camera
	on bool
}

func (r *riding) Fasten(camera.Fastening) {}

func (r *riding) Fastening() camera.Fastening {
	if r.on {
		return camera.Fastening{Entity: 1, How: camera.Inside}
	}
	return camera.Fastening{}
}

func TestBind_OneKeyDoesWhatTheCamerasHowSays(t *testing.T) {
	r := newRig(t)
	players.CaptureWith(r.p, func(bool) {})
	cam := &riding{Camera: r.cam}
	r.look(cam)
	r.bind(
		control.Build(control.KeyPress{Key: control.KeyA}, "scroll", orderOf(1)).In(camera.Outside),
		control.Build(control.KeyPress{Key: control.KeyA}, "turn", orderOf(2)).In(camera.Inside),
	)
	if err := r.local.Bind(control.Build(control.KeyPress{Key: control.KeyA}, "both", orderOf(3))); err == nil {
		t.Error("a binding on A in every How beside ones in each was accepted")
	}
	press := func() []command.Issued[order] {
		ev := &control.InputEvents{}
		ev.AddKeyEvent(control.KeyA, control.ActionPress)
		ev.AddKeyEvent(control.KeyA, control.ActionRelease)
		r.handle(ev)
		return r.drained()
	}
	if got := press(); len(got) != 1 || got[0].Command.Cell != 1 {
		t.Errorf("A with the camera free issued %v, want order 1 alone", got)
	}
	cam.on = true
	if got := press(); len(got) != 1 || got[0].Command.Cell != 2 {
		t.Errorf("A with the camera riding issued %v, want order 2 alone", got)
	}
}

func TestCursorMove_LooksRoundWhileTheCameraRides(t *testing.T) {
	r := newRig(t)
	var caught []bool
	players.CaptureWith(r.p, func(on bool) { caught = append(caught, on) })
	cam := &riding{Camera: r.cam}
	r.look(cam)
	r.bind(control.Build(control.CursorMove{}, "look", func(c control.Context) (order, bool) {
		return order{int(c.Delta.X)}, true
	}).In(camera.Inside))
	move := func(x, y int) []command.Issued[order] {
		ev := &control.InputEvents{MousePos: geom.NewVec(float64(x), float64(y)), CursorDelta: geom.NewVec(7, 0)}
		r.handle(ev)
		return r.drained()
	}
	if got := move(10, 10); len(got) != 0 || len(caught) != 0 {
		t.Errorf("free, a move issued %v and caught the cursor %v, want nothing", got, caught)
	}
	cam.on = true
	if got := move(10, 10); len(got) != 0 || len(caught) != 1 || !caught[0] {
		t.Errorf("riding, the first pass issued %v and caught %v, want the cursor caught and no move taken", got, caught)
	}
	if got := move(-5000, 9000); len(got) != 1 || got[0].Command.Cell != 7 {
		t.Errorf("riding, a move far off the screen issued %v, want a look by 7", got)
	}
	cam.on = false
	if got := move(10, 10); len(got) != 0 || len(caught) != 2 || caught[1] {
		t.Errorf("free again, a move issued %v and caught %v, want the cursor let go and nothing issued", got, caught)
	}
}

func (c *installCtx) Hosts(h ...host.Host)   { c.hosts = append(c.hosts, h...) }
func (c *installCtx) Draws(...render.Source) {}
func (c *installCtx) Looks(func())           {}
func (*installCtx) Pool() *parallel.Pool     { return nil }

func (c *installCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }

func TestKeyReleased_FiresWithHowLongTheKeyWasHeldAndNotAfterAChord(t *testing.T) {
	r := newRig(t)
	now := time.Unix(0, 0)
	players.ClockWith(r.p, func() time.Time { return now })
	r.bind(
		control.Build(control.KeyReleased{Key: control.KeyShift}, "throw",
			func(c control.Context) (order, bool) {
				return order{int(c.Held / time.Millisecond)}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyX, Mods: control.Mods{Shift: true}}, "boxes",
			orderOf(-1)),
	)
	r.start()
	key := func(k control.Key, a control.KeyAction, shift bool) {
		ev := &control.InputEvents{KeyEvents: []control.KeyEvent{{Key: k, Action: a}}}
		ev.Modifiers.Shift = shift
		r.handle(ev)
	}
	key(control.KeyShift, control.ActionPress, true)
	now = now.Add(700 * time.Millisecond)
	key(control.KeyShift, control.ActionRelease, false)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != 700 {
		t.Errorf("held 700 ms and let go: %+v, want one order of 700", got)
	}
	key(control.KeyShift, control.ActionPress, true)
	key(control.KeyX, control.ActionPress, true)
	key(control.KeyX, control.ActionRelease, true)
	now = now.Add(time.Second)
	key(control.KeyShift, control.ActionRelease, false)
	if got := r.drained(); len(got) != 1 || got[0].Command.Cell != -1 {
		t.Errorf("Shift+X and Shift let go: %+v, want the chord's order alone", got)
	}
}

func TestCamera_AtAResolutionTheCursorReachesTheBindingsInTheCamerasPixels(t *testing.T) {
	r := newRig(t)
	r.cam.(camera.Resolved).SetResolution(0.5)
	r.cam.SetViewport(800, 600)
	r.wire.Over(geom.NewAABBAt(geom.NewVec(0, 0), 800, 600), render.NewFeed(r.cam, nil))
	var caught []control.Context
	catch := func(c control.Context) (order, bool) {
		caught = append(caught, c)
		return order{1}, true
	}
	r.bind(control.Build(control.CursorAtEdge{}, "edge", catch),
		control.Build(control.Drag{Button: control.MouseButtonLeft}, "drag", catch))
	ecs := r.start()
	r.move(&control.InputEvents{MousePos: geom.NewVec(790, 300)}, ecs)
	if len(caught) != 1 {
		t.Fatalf("the cursor 10 shown pixels from the right edge fired %d bindings, want the edge",
			len(caught))
	}
	c := caught[0]
	if c.Cursor != geom.NewVec(395, 150) || c.Screen != geom.NewVec(400, 300) ||
		c.Resolution != 0.5 {
		t.Errorf("the edge's context has the cursor at %v of a screen %v at %v, want (395, 150) "+
			"of (400, 300) at 0.5", c.Cursor, c.Screen, c.Resolution)
	}
	caught = nil
	ev := &control.InputEvents{MousePos: geom.NewVec(400, 300)}
	left := control.MouseButtonLeft
	ev.ClickQueue = []control.ClickEvent{
		{Button: left, Action: control.ActionPress, Pos: geom.NewVec(100, 200)},
		{Button: left, Action: control.ActionRelease, Pos: geom.NewVec(400, 300)},
	}
	r.move(ev, ecs)
	if len(caught) != 1 || caught[0].Start != geom.NewVec(50, 100) ||
		caught[0].Cursor != geom.NewVec(200, 150) {
		t.Errorf("a drag from (100, 200) to (400, 300) reached the bindings as %+v, want from "+
			"(50, 100) to (200, 150)", caught)
	}
}
