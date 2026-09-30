package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/topography/relief"
)

// CoarseShadows switches the terrain's shadows between their full detail and a coarser bake: half
// as fine a side, a quarter of the work the GPU does while the sun goes on.
type CoarseShadows struct{}

// Queues are where the view's, the shaping's and the shadows' commands land.
func (p *Plugin) Queues() []control.CommandQueue {
	return append(append(p.cameras.Queues(), p.shaper.Queues()...), &p.coarse)
}

// DefaultBindings are the view's (cameras.Control.Bindings); = raises and - lowers the ground under
// the cursor, a left drag with L held levels it to where the drag began; H makes the shadows
// coarser or fine again.
func (p *Plugin) DefaultBindings() []control.Binding {
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	return append(p.cameras.Bindings(),
		control.Command(control.KeyPress{Key: control.KeyEqual}, "Raise the ground", func(c control.Context) (relief.Raise, bool) { return relief.Raise{At: at(c)}, true }),
		control.Command(control.KeyPress{Key: control.KeyMinus}, "Lower the ground", func(c control.Context) (relief.Lower, bool) { return relief.Lower{At: at(c)}, true }),
		control.Command(control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{}.Holding(control.KeyL)}, "Level the ground",
			func(c control.Context) (relief.Level, bool) {
				return relief.Level{From: c.World(c.Start), To: c.World(c.Cursor)}, true
			}),
		control.Command(control.KeyPress{Key: control.KeyH}, "Coarser shadows, a quarter of the work, or fine again",
			func(control.Context) (CoarseShadows, bool) { return CoarseShadows{}, true }),
	)
}
