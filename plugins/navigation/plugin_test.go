package navigation

import (
	"testing"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/internal/hosts"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/driving"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule"
)

// stubInstallCtx is a minimal plugin.Installer for tests that call Install directly.
type stubInstallCtx struct {
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
	ecs     *goke.ECS
	pending []func() []goke.System
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
	})
	boardPlugin := board.NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin)
	boardPlugin.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sel := selection.NewPlugin(worldPlugin)
	navPlugin := NewPlugin(boardPlugin, worldPlugin, sel, driving.NewPlugin(worldPlugin, sel))
	pl := players.NewPlugin(worldPlugin, navPlugin)
	local := pl.Local("tester")
	pl.Through(local).Over(geom.AABB{}, render.NewFeed(cameras.TopDown()(50, 50, 0, camera.Config{}), nil)) // as a scene showing it wires it
	if err := local.Bind(navPlugin.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	want := grid.CellIndex(2, 2)

	events := &control.InputEvents{}
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	pl.EventHandler().HandleEvents(events)
	var got []MoveTo
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 0 {
		t.Errorf("the right button going down issued %v, want nothing until it comes up", got)
	}
	events = &control.InputEvents{}
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionRelease)
	pl.EventHandler().HandleEvents(events)
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 1 || got[0] != (MoveTo{Cell: want, At: geom.NewVec(25, 25)}) {
		t.Errorf("a right click issued %v, want one MoveTo to %v at the point clicked", got, want)
	}

	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	events.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionRelease)
	pl.EventHandler().HandleEvents(events)
	got = got[:0]
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
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
	navPlugin.looks.Drain(func(i control.Issued[LookAt]) { looks = append(looks, i.Command) })
	got = got[:0]
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(looks) != 0 || len(got) != 1 {
		t.Errorf("a right click with Q held: %d LookAt and %d MoveTo, want none and one", len(looks), len(got))
	}
}

// A right drag turns the selected units to look where the cursor goes, a LookAt at every move
// past the click slop, and moves nothing when the button comes up; a click that shook within the
// slop is still a click.
func TestPlugin_DefaultBindings_ARightDragTurnsTheUnitsAndMovesNothing(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	boardPlugin := board.NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin)
	boardPlugin.Res.Logic.Board.SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	sel := selection.NewPlugin(worldPlugin)
	navPlugin := NewPlugin(boardPlugin, worldPlugin, sel, driving.NewPlugin(worldPlugin, sel))
	pl := players.NewPlugin(worldPlugin, navPlugin)
	local := pl.Local("tester")
	pl.Through(local).Over(geom.AABB{}, render.NewFeed(cameras.TopDown()(50, 50, 0, camera.Config{}), nil)) // as a scene showing it wires it
	if err := local.Bind(navPlugin.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	handle := func(ev *control.InputEvents) (looks []LookAt, moves []MoveTo) {
		pl.EventHandler().HandleEvents(ev)
		navPlugin.looks.Drain(func(i control.Issued[LookAt]) { looks = append(looks, i.Command) })
		navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { moves = append(moves, i.Command) })
		return looks, moves
	}
	press := &control.InputEvents{MousePos: geom.NewVec(25, 25)}
	press.AddClickEvent(25, 25, control.MouseButtonRight, control.ActionPress)
	if looks, moves := handle(press); len(looks) != 0 || len(moves) != 0 {
		t.Fatalf("the button going down issued %v and %v, want nothing", looks, moves)
	}
	for _, at := range []geom.Vec{{X: 35, Y: 30}, {X: 40, Y: 40}} {
		looks, moves := handle(&control.InputEvents{MousePos: at, CursorDelta: geom.NewVec(5, 5)})
		if len(looks) != 1 || looks[0].At != at || len(moves) != 0 {
			t.Errorf("the cursor at %v issued %v and %v, want one LookAt there and no move", at, looks, moves)
		}
	}
	release := &control.InputEvents{MousePos: geom.NewVec(40, 40)}
	release.AddClickEvent(40, 40, control.MouseButtonRight, control.ActionRelease)
	if looks, moves := handle(release); len(looks) != 0 || len(moves) != 0 {
		t.Errorf("the drag's release issued %v and %v, want nothing", looks, moves)
	}

	handle(press)
	if looks, _ := handle(&control.InputEvents{MousePos: geom.NewVec(27, 26), CursorDelta: geom.NewVec(2, 1)}); len(looks) != 0 {
		t.Errorf("a shake within the slop issued %v, want no LookAt", looks)
	}
	shaken := &control.InputEvents{MousePos: geom.NewVec(27, 26)}
	shaken.AddClickEvent(27, 26, control.MouseButtonRight, control.ActionRelease)
	want := grid.CellIndex(2, 2)
	if _, moves := handle(shaken); len(moves) != 1 || moves[0].Cell != want {
		t.Errorf("a shaken click issued %v, want one MoveTo to %v", moves, want)
	}
}

// Shift+P issues Routes, which shows the routes drawn and hides them again; the goals are drawn
// whatever it says.
func TestPlugin_DefaultBindings_ShiftPTogglesTheRoutes(t *testing.T) {
	grid := grid.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	boardPlugin := board.NewPlugin(grid, &cell.SingleOccupancy{}, worldPlugin)
	sel := selection.NewPlugin(worldPlugin)
	navPlugin := NewPlugin(boardPlugin, worldPlugin, sel, driving.NewPlugin(worldPlugin, sel))
	pl := players.NewPlugin(worldPlugin, navPlugin)
	local := pl.Local("tester")
	pl.Through(local).Over(geom.AABB{}, render.NewFeed(cameras.TopDown()(50, 50, 0, camera.Config{}), nil)) // as a scene showing it wires it
	if err := local.Bind(navPlugin.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	events := &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddKeyEvent(control.KeyP, control.ActionPress)
	pl.EventHandler().HandleEvents(events)
	n := 0
	navPlugin.routes.Drain(func(control.Issued[Routes]) { n++ })
	if n != 1 {
		t.Errorf("Shift+P issued %d Routes, want one", n)
	}
	navPlugin.WithRenderer(nil)
	if navPlugin.RoutesShown() || navPlugin.pathRenderer.RoutesShown() {
		t.Error("the routes start shown, want hidden until asked for")
	}
	navPlugin.ShowRoutes(true)
	if !navPlugin.RoutesShown() || !navPlugin.pathRenderer.RoutesShown() {
		t.Error("ShowRoutes(true) did not reach the renderer")
	}
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stubInstallCtx) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stubInstallCtx) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
