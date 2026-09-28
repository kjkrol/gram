package vision

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/control"
)

// Cones hides every view drawn — the cones of sight and the ground out of sight — or shows them
// again. A look at the world, like the camera's turn: at once, in the pause too, not saved.
type Cones struct{}

// Queues are where Cones lands.
func (p *Plugin) Queues() []control.CommandQueue { return []control.CommandQueue{&p.cones} }

// DefaultBindings show and hide the cones of sight on Shift+C.
func (p *Plugin) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyC, Mods: control.Mods{Shift: true}}, "Show or hide the cones of sight", func(control.Context) (Cones, bool) { return Cones{}, true }),
	}
}
