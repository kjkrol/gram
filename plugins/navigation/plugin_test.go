package navigation

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/selection"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
)

// stubInstallCtx is a minimal plugin.Installer for tests that call Install directly.
type stubInstallCtx struct {
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
	grid := board.DefaultGrids{}.Square(5, 5, 10)
	worldPlugin := world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 50, Height: 50},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	boardPlugin := board.NewPlugin(grid, &board.SingleOccupancy{}, worldPlugin)
	boardPlugin.Res.Logic.Board.SetAll(board.CellKind{Cost: 1, Allows: board.Land})
	navPlugin := NewPlugin(boardPlugin, worldPlugin, selection.NewPlugin(worldPlugin))
	pl := players.NewPlugin(worldPlugin, navPlugin)
	local := pl.Local("tester")
	if err := local.Bind(navPlugin.DefaultBindings()...); err != nil {
		t.Fatal(err)
	}
	want, _ := grid.CellIndex(2, 2)

	events := &control.InputEvents{}
	events.AddClickEvent(25, 25, ebiten.MouseButtonRight, control.ActionPress)
	pl.EventHandler().HandleEvents(events)
	var got []MoveTo
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 1 || got[0] != (MoveTo{Cell: want, At: geom.NewVec(25, 25)}) {
		t.Errorf("a right click issued %v, want one MoveTo to %v at the point clicked", got, want)
	}

	events = &control.InputEvents{}
	events.Modifiers.Shift = true
	events.AddClickEvent(25, 25, ebiten.MouseButtonRight, control.ActionPress)
	pl.EventHandler().HandleEvents(events)
	got = got[:0]
	navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
	if len(got) != 1 || !got[0].Append {
		t.Errorf("a Shift right click issued %v, want one MoveTo that appends", got)
	}

	// S held: a right click looks there instead of going; another key held does not get in the way.
	for _, tc := range []struct {
		held       ebiten.Key
		look, move int
	}{{ebiten.KeyS, 1, 0}, {ebiten.KeyQ, 0, 1}} {
		events = &control.InputEvents{}
		events.AddKeyEvent(tc.held, control.ActionPress)
		events.AddClickEvent(25, 25, ebiten.MouseButtonRight, control.ActionPress)
		pl.EventHandler().HandleEvents(events)
		var looks []LookAt
		navPlugin.looks.Drain(func(i control.Issued[LookAt]) { looks = append(looks, i.Command) })
		got = got[:0]
		navPlugin.moves.Drain(func(i control.Issued[MoveTo]) { got = append(got, i.Command) })
		if len(looks) != tc.look || len(got) != tc.move {
			t.Errorf("right click with %v held: %d LookAt and %d MoveTo, want %d and %d", tc.held, len(looks), len(got), tc.look, tc.move)
		}
		events = &control.InputEvents{}
		events.AddKeyEvent(tc.held, control.ActionRelease)
		pl.EventHandler().HandleEvents(events)
	}
}
