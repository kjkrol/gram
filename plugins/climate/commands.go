package climate

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/control"
)

// Change has the weather go on to the next state now, thrown as when one runs out.
type Change struct{}

// Set has the weather go into the state named Name now; a name the climate lacks changes nothing.
type Set struct{ Name string }

// Queues are where Change and Set land.
func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.change, &p.set} }

// DefaultBindings change the weather on W.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyW}, "Change the weather", func(control.Context) (Change, bool) { return Change{}, true }),
	}
}
