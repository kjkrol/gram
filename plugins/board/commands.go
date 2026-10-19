package board

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Grid shows the board's grid, or hides it (Settings.Grid). B by default, in turn.
type Grid struct{ Shown bool }

// Holds shows the cells the occupancy holds, or hides them (Settings.Holds). Shift+O by default,
// in turn.
type Holds struct{ Shown bool }

// Settings is what the board lets the players adjust, saved with the game: the looks of the
// grid and of the cells held.
type Settings struct {
	Grid  bool
	Holds bool
}

func (Grid) Command()  {}
func (Holds) Command() {}

var (
	_ command.Command = Grid{}
	_ command.Command = Holds{}
)

// Queues are where Grid and Holds land.
func (p *Plugin) Queues() []command.CommandQueue {
	return []command.CommandQueue{&p.grids, &p.holds}
}

// DefaultBindings show and hide the grid on B and the cells held on Shift+O.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyB}, "Show or hide the grid",
			func(control.Context) (Grid, bool) { return Grid{Shown: !p.GridShown()}, true }),
		control.Build(control.KeyPress{Key: control.KeyO, Mods: control.Mods{Shift: true}},
			"Show or hide the cells held", func(control.Context) (Holds, bool) {
				return Holds{Shown: !p.HoldsShown()}, true
			}),
	}
}

// Settings are the grid and the cells held (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Grid",
			Get:   p.GridShown,
			Set:   func(on bool) command.Command { return Grid{Shown: on} },
		},
		control.Switch{
			Label: "Cells held",
			Get:   p.HoldsShown,
			Set:   func(on bool) command.Command { return Holds{Shown: on} },
		},
	}
}
