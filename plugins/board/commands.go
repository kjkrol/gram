package board

import "github.com/kjkrol/gram/control"

// Grid shows the board's grid, or hides it: a look, not saved.
type Grid struct{}

func (Grid) Command() {}

var (
	_ control.Command = Grid{}
)

// Queues are where Grid lands.
func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.grids} }

// DefaultBindings show and hide the grid on B.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Build(control.KeyPress{Key: control.KeyB}, "Show or hide the grid", func(control.Context) (Grid, bool) { return Grid{}, true }),
	}
}
