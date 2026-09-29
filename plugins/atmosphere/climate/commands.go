package climate

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
)

// Change has the weather go on to the next state now, thrown as when one runs out.
type Change struct{}

// Set has the weather go into the state named Name now; a name the climate lacks changes nothing.
type Set struct{ Name string }

// Queues are where Change and Set land.
func (c *Climate) Queues() []control.CommandQueue { return []control.CommandQueue{&c.change, &c.set} }

// DefaultBindings change the weather on Shift+W, with the camera free: riding in a unit, W with
// Shift sprints.
func (c *Climate) DefaultBindings() []control.Binding {
	return []control.Binding{
		control.Command(control.KeyPress{Key: ebiten.KeyW, Mods: control.Mods{Shift: true}}, "Change the weather", func(control.Context) (Change, bool) { return Change{}, true }).In(camera.Free),
	}
}
