package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/gram/control"
)

// CoarseShadows switches the terrain's shadows between their full detail and a coarser bake,
// half as fine a side.
type CoarseShadows struct{}

func (CoarseShadows) Command() {}

var (
	_ command.Command = CoarseShadows{}
)

// Queues are where the view's, the shaping's and the shadows' commands land.
func (p *Plugin) Queues() []command.CommandQueue {
	return append(append(p.camQueues.queues(), p.shaper.queues()...), &p.coarse)
}

// DefaultBindings are the views' keys, = and - raising and lowering the ground under the cursor,
// a left drag with L held levelling it, and H switching the shadows' detail.
func (p *Plugin) DefaultBindings() []control.Binding {
	at := func(c control.Context) geom.Vec { return c.World(c.Cursor) }
	return append(cameraBindings(p.cfg.Perspective),
		control.Build(control.KeyPress{Key: control.KeyEqual}, "Raise the ground", func(c control.Context) (Raise, bool) { return Raise{At: at(c)}, true }),
		control.Build(control.KeyPress{Key: control.KeyMinus}, "Lower the ground", func(c control.Context) (Lower, bool) { return Lower{At: at(c)}, true }),
		control.Build(control.Drag{Button: control.MouseButtonLeft, Mods: control.Mods{}.Holding(control.KeyL)}, "Level the ground",
			func(c control.Context) (Level, bool) {
				return Level{From: c.World(c.Start), To: c.World(c.Cursor)}, true
			}),
		control.Build(control.KeyPress{Key: control.KeyH}, "Coarser shadows, a quarter of the work, or fine again",
			func(control.Context) (CoarseShadows, bool) { return CoarseShadows{}, true }),
	)
}
