package navigation

import (
	"reflect"
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type stubInstallCtx struct {
	hosts   []host.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
	drawn   []render.Source
	pool    *parallel.Pool // nil: one goroutine
}

func (c *stubInstallCtx) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(si *goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}
func (c *stubInstallCtx) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *stubInstallCtx) RegSys(factory func() goke.System) goke.Runnable {
	return c.ecs.RegSys(factory())
}
func (c *stubInstallCtx) ECS() *goke.ECS { return c.ecs }

func TestPlugin_DefaultBindings_TurnARightClickIntoMoveTo(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(worldPlugin)
	boardPlugin := board.NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin).
		WithMotion(m, cell.DefaultHiking)
	boardPlugin.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	sel := selection.NewPlugin(worldPlugin)
	navPlugin := NewPlugin(boardPlugin, worldPlugin, m, sel, driving.NewPlugin(worldPlugin, m, sel))
	pl := players.NewPlugin(worldPlugin, navPlugin)
	local := pl.Local("tester")
	// as a scene showing it wires it
	pl.Through(local).Over(geom.AABB{}, render.NewFeed(cameratest.NewFromSpace(50, 50, 0), nil))
	if err := local.Bind(navPlugin.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	want := grid.At(2, 2)

	events := &control.InputEvents{}
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	pl.EventHandler().HandleEvents(events)
	var got []MoveTo
	navPlugin.moves.Drain(func(i command.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 0 {
		t.Errorf("the right button going down issued %v, want nothing until it comes up", got)
	}
	events = &control.InputEvents{}
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionRelease)
	pl.EventHandler().HandleEvents(events)
	navPlugin.moves.Drain(func(i command.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 1 || got[0].Cell != want || got[0].At != geom.NewVec(25, 25) || got[0].Append {
		t.Errorf("a right click issued %v, want one MoveTo to %v at the point clicked", got, want)
	}

	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionRelease)
	pl.EventHandler().HandleEvents(events)
	got = got[:0]
	navPlugin.moves.Drain(func(i command.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 1 || !got[0].Append {
		t.Errorf("a Shift right click issued %v, want one MoveTo that appends", got)
	}

	// another key held does not get in the way of a click
	events = &control.InputEvents{}
	events.AddKeyEvent(control.KeyQ, control.ActionPress)
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionRelease)
	pl.EventHandler().HandleEvents(events)
	var looks []LookAt
	navPlugin.looks.Drain(func(i command.Issued[LookAt]) { looks = append(looks, i.Command) })
	got = got[:0]
	navPlugin.moves.Drain(func(i command.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(looks) != 0 || len(got) != 1 {
		t.Errorf("a right click with Q held: %d LookAt and %d MoveTo, want none and one", len(looks), len(got))
	}
}

// dragRig is navigation over a small board with one local player bound to its defaults, the
// selection plugin ready to say whether the player has one
type dragRig struct {
	grid  grid.Grid
	w     *world.Plugin
	sel   *selection.Plugin
	nav   *Plugin
	pl    *players.Plugin
	local *players.Player
	ctx   *stubInstallCtx
}

func newDragRig(t *testing.T) *dragRig {
	t.Helper()
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(grid, &cell.SingleOccupancy{}, w).WithMotion(m, cell.DefaultHiking)
	brd.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	sel := selection.NewPlugin(w)
	nav := NewPlugin(brd, w, m, sel, driving.NewPlugin(w, m, sel))
	pl := players.NewPlugin(w, nav)
	local := pl.Local("tester")
	// as a scene showing it wires it
	pl.Through(local).Over(geom.AABB{}, render.NewFeed(cameratest.NewFromSpace(50, 50, 0), nil))
	if err := local.Bind(nav.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	return &dragRig{grid: grid, w: w, sel: sel, nav: nav, pl: pl, local: local,
		ctx: &stubInstallCtx{ecs: goke.New()}}
}

// selected installs the world and the selection with one unit of the player's, selected
func (r *dragRig) selected(t *testing.T) {
	t.Helper()
	if err := r.w.Install(r.ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.sel.Install(r.ctx); err != nil {
		t.Fatal(err)
	}
	type walker struct{}
	kind.Define[walker](r.w.Kinds(), "walker", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(20, 20), 4, 4)}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Layers(cell.Land)),
		comp.Tagged(r.sel.Tags().Selectable, r.sel.Tags().Selected),
		comp.Tagged(owner.Of(r.local.ID)),
	})
	r.w.Write().Seed(kind.Named[walker](r.w.Kinds(), "walker").Entry(walker{}))
	if err := r.w.Populate(); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range r.ctx.pending {
		systems = append(systems, produce()...)
	}
	r.ctx.ecs.Setup(systems...)
	if !r.sel.Read(0).Any(r.local.ID) {
		t.Fatal("the player has no selection, want the walker seeded selected")
	}
}

func (r *dragRig) handle(ev *control.InputEvents) (looks []LookAt, moves []MoveTo) {
	r.pl.EventHandler().HandleEvents(ev)
	r.nav.looks.Drain(func(i command.Issued[LookAt]) { looks = append(looks, i.Command) })
	r.nav.moves.Drain(func(i command.Issued[MoveTo]) { moves = append(moves, i.Command) })
	return looks, moves
}

func rightPress(x, y int) *control.InputEvents {
	press := &control.InputEvents{MousePos: geom.NewVec(float32(x), float32(y)),
		CursorInWindow: true}
	press.AddClickEvent(x, y, control.MouseButtonRight, control.ActionPress)
	return press
}

func TestPlugin_DefaultBindings_ARightDragTurnsTheSelectedUnitsAndMovesNothing(t *testing.T) {
	r := newDragRig(t)
	r.selected(t)
	if looks, moves := r.handle(rightPress(25, 25)); len(looks) != 0 || len(moves) != 0 {
		t.Fatalf("the button going down issued %v and %v, want nothing", looks, moves)
	}
	if looks, _ := r.handle(&control.InputEvents{MousePos: geom.NewVec(27, 26),
		CursorDelta: geom.NewVec(2, 1), CursorInWindow: true}); len(looks) != 0 {
		t.Errorf("a shake within the slop issued %v, want no LookAt", looks)
	}
	for _, at := range []geom.Vec{{X: 35, Y: 30}, {X: 40, Y: 40}} {
		looks, moves := r.handle(&control.InputEvents{MousePos: at, CursorDelta: geom.NewVec(5, 5),
			CursorInWindow: true})
		if len(looks) != 1 || looks[0].At != at || len(moves) != 0 {
			t.Errorf("the cursor at %v issued %v and %v, want one LookAt there and no move", at,
				looks, moves)
		}
	}
	release := &control.InputEvents{MousePos: geom.NewVec(40, 40), CursorInWindow: true}
	release.AddClickEvent(40, 40, control.MouseButtonRight, control.ActionRelease)
	if looks, moves := r.handle(release); len(looks) != 0 || len(moves) != 0 {
		t.Errorf("the drag's release issued %v and %v, want nothing", looks, moves)
	}
}

func TestPlugin_DefaultBindings_WithoutASelectionARightDragTurnsNothing(t *testing.T) {
	r := newDragRig(t)
	handle := r.handle
	press := rightPress(25, 25)
	if looks, moves := handle(press); len(looks) != 0 || len(moves) != 0 {
		t.Fatalf("the button going down issued %v and %v, want nothing", looks, moves)
	}
	for _, at := range []geom.Vec{{X: 35, Y: 30}, {X: 40, Y: 40}} {
		looks, moves := handle(&control.InputEvents{MousePos: at, CursorDelta: geom.NewVec(5, 5),
			CursorInWindow: true})
		if len(looks) != 0 || len(moves) != 0 {
			t.Errorf("the cursor at %v issued %v and %v, want nothing without a selection: the "+
				"drag is left to what yields to it", at, looks, moves)
		}
	}
	release := &control.InputEvents{MousePos: geom.NewVec(40, 40), CursorInWindow: true}
	release.AddClickEvent(40, 40, control.MouseButtonRight, control.ActionRelease)
	if looks, moves := handle(release); len(looks) != 0 || len(moves) != 0 {
		t.Errorf("the drag's release issued %v and %v, want nothing", looks, moves)
	}

	handle(press)
	if looks, _ := handle(&control.InputEvents{MousePos: geom.NewVec(27, 26),
		CursorDelta: geom.NewVec(2, 1), CursorInWindow: true}); len(looks) != 0 {
		t.Errorf("a shake within the slop issued %v, want no LookAt", looks)
	}
	shaken := &control.InputEvents{MousePos: geom.NewVec(27, 26)}
	shaken.AddClickEvent(27, 26, control.MouseButtonRight, control.ActionRelease)
	want := r.grid.At(2, 2)
	if _, moves := handle(shaken); len(moves) != 1 || moves[0].Cell != want {
		t.Errorf("a shaken click issued %v, want one MoveTo to %v", moves, want)
	}
}

func TestPlugin_TheRoutesAreTheSettingsScreensAndSaved(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	m := motion.NewPlugin(worldPlugin)
	boardPlugin := board.NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin).
		WithMotion(m, cell.DefaultHiking)
	sel := selection.NewPlugin(worldPlugin)
	navPlugin := NewPlugin(boardPlugin, worldPlugin, m, sel, driving.NewPlugin(worldPlugin, m, sel))
	for _, b := range navPlugin.DefaultBindings() {
		if b.Type() == reflect.TypeFor[Routes]() {
			t.Errorf("Routes is bound to %v, want the settings screen alone", b.Trigger)
		}
	}
	if navPlugin.read.RoutesShown() || navPlugin.pathRenderer.RoutesShown() {
		t.Error("the routes start shown, want hidden until asked for")
	}
	navPlugin.write.ShowRoutes(true)
	if !navPlugin.read.RoutesShown() || !navPlugin.pathRenderer.RoutesShown() {
		t.Error("ShowRoutes(true) did not reach the renderer")
	}
	routes := navPlugin.Settings()[0].(control.Switch)
	if !routes.Get() || routes.Set(false) != (Routes{Shown: false}) {
		t.Errorf("the setting says %v and sets %v, want shown and Routes hidden", routes.Get(),
			routes.Set(false))
	}
	saved := append([]byte(nil), *navPlugin.Serializable().Persisted()[0].(*[]byte)...)
	navPlugin.write.ShowRoutes(false)
	loading := navPlugin.Serializable().Persisted()
	*loading[0].(*[]byte) = saved
	navPlugin.Restore()
	if !navPlugin.read.RoutesShown() || !navPlugin.pathRenderer.RoutesShown() {
		t.Error("loaded, the routes are hidden, want shown as saved and reaching the renderer")
	}
}

func (c *stubInstallCtx) Hosts(h ...host.Host)     { c.hosts = append(c.hosts, h...) }
func (c *stubInstallCtx) Draws(s ...render.Source) { c.drawn = append(c.drawn, s...) }
func (c *stubInstallCtx) Looks(func())             {}
func (c *stubInstallCtx) Pool() *parallel.Pool     { return c.pool }

// as the engine does with the roles played once a Stage's Init returns
func (c *stubInstallCtx) Deliver(rules ...act.Rule) error { return ceremony.Deliver(c.hosts, rules...) }

func TestPlugin_DrawsItsRoutes(t *testing.T) {
	w := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 128, Height: 128},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 32},
		Heights:  true,
	})
	m := motion.NewPlugin(w)
	brd := board.NewPlugin(grid.DefaultGrids{}.Square(4, 4, 32), &cell.SingleOccupancy{}, w).
		WithMotion(m, cell.DefaultHiking)
	sel := selection.NewPlugin(w)
	nav := NewPlugin(brd, w, m, sel, driving.NewPlugin(w, m, sel))
	ctx := &stubInstallCtx{ecs: goke.New()}
	if err := nav.Install(ctx); err != nil {
		t.Fatal(err)
	}
	if len(ctx.drawn) != 1 || ctx.drawn[0] != render.Source(nav.pathRenderer) {
		t.Errorf("navigation draws %v, want its routes", ctx.drawn)
	}
}
