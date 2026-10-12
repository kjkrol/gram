package board

import "github.com/kjkrol/gram/control"

// Grid shows the board's grid, or hides it: a look, not saved.
type Grid struct{}

// Holds shows the cells the occupancy holds, or hides them: a look, not saved (ShowHolds).
type Holds struct{}

func (Grid) Command()  {}
func (Holds) Command() {}

var (
	_ control.Command = Grid{}
	_ control.Command = Holds{}
)

// Queues are where Grid and Holds land.
func (p *Plugin) Queues() []control.CommandQueue {
	return []control.CommandQueue{&p.grids, &p.holds}
}

// DefaultBindings show and hide the grid on B and the cells held on Shift+O.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyB}, "Show or hide the grid",
			func(control.Context) (Grid, bool) { return Grid{}, true }),
		control.Build(control.KeyPress{Key: control.KeyO, Mods: control.Mods{Shift: true}},
			"Show or hide the cells held", func(control.Context) (Holds, bool) {
				return Holds{}, true
			}),
	}
}
