package board

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// Grid shows the board's grid, or hides it (Settings.Grid). The settings screen alone.
type Grid struct{ Shown bool }

// Holds shows the cells the occupancy holds, or hides them (Settings.Holds). The settings screen
// alone.
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

// DefaultBindings are none: the grid and the cells held are the settings screen's.
func (p *Plugin) DefaultBindings() []control.Binding { return nil }

// Settings are the grid and the cells held (plugin.Tunable).
func (p *Plugin) Settings() []control.Setting {
	return []control.Setting{
		control.Switch{
			Label: "Grid",
			Get:   p.read.GridShown,
			Set:   func(on bool) command.Command { return Grid{Shown: on} },
		},
		control.Switch{
			Label: "Cells held",
			Get:   p.read.HoldsShown,
			Set:   func(on bool) command.Command { return Holds{Shown: on} },
		},
	}
}
