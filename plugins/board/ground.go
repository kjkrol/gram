package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
)

// Heights is the height of the board's ground at a point and how far apart it is sampled along a
// ray: the Map's, a topography's relief; nil on a flat map. Sight and navigation read it.
type Heights interface {
	At(p geom.Vec) float64
	Step() float64
}

// Cover is what stands on the board and holds sight back — walls, forests — walked along a ray:
// Walk calls visit, nearest first, for every stretch of the ray from origin along the unit dir, up
// to length, inside a cell whose cover dims the sight of an observer on the layers blockers (zero:
// every layer), with the band it spans (bottom, top; ±Inf in a flat world) and how see-through it
// is (tau: ≤ 0 blocks); visit returning false ends the walk. The Board is one; sight reads it.
type Cover interface {
	Walk(origin, dir geom.Vec, length float64, blockers world.Layers, visit func(near, far, bottom, top, tau float64) bool)
}

// Readied is a Cover that reads the board as it is walked and can read it all at once instead —
// for whoever walks it from several goroutines at a time, on their goroutine before they do. The
// Board is one.
type Readied interface {
	Ready()
}

var _ Cover = (*Board)(nil)
var _ Readied = (*Board)(nil)
var _ collision.Field = (*Board)(nil)

// Heights is the board's ground heights: its Map's, nil on a flat map.
func (p *Plugin) Heights() Heights { return p.mapping.Heights() }

// Cover is what stands on the board and holds sight back: the Board itself.
func (p *Plugin) Cover() Cover { return p.Res.Logic.Board }

// WithCollision makes the board's Solid cells the solid ground c pushes colliders out of; call
// before Use.
func (p *Plugin) WithCollision(c *collision.Plugin) *Plugin {
	c.WithField(p.Res.Logic.Board)
	return p
}
