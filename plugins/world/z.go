package world

import (
	"github.com/kjkrol/aabbworld/collide"
	"github.com/kjkrol/aabbworld/geom"
)

// Z is an entity's place in height: Altitude is its bottom, written by the board from the ground
// under it, Height its rise above that. Only a Quasi3D world carries it; see Config.Quasi3D.
type Z struct{ Altitude, Height float64 }

// Top is the entity's highest point.
func (z Z) Top() float64 { return z.Altitude + z.Height }

// Ground is the height of the world's ground at a point and how far apart it is sampled along a
// ray; the board gives a Quasi3D world one, sight reads it.
type Ground interface {
	At(p geom.Vec) float64
	Step() float64
}

// Cover is what stands on the ground of a world laid out on a grid and holds sight back — walls,
// forests — walked along a ray: Walk calls visit, nearest first, for every stretch of the ray
// from origin along the unit dir, up to length, inside a cell whose cover dims the sight of an
// observer on the layers blockers (zero: every layer), with the band it spans (bottom, top; ±Inf
// in a flat world) and how see-through it is (tau: ≤ 0 blocks); visit returning false ends the
// walk. The board gives the world one; sight reads it.
type Cover interface {
	Walk(origin, dir geom.Vec, length float64, blockers Layers, visit func(near, far, bottom, top, tau float64) bool)
}

// Field is the solid ground of a world laid out on a grid, for an entity on the layers given:
// Solid calls visit with every solid box that box may overlap, its cell and its sides that face
// open ground; visit returning false ends the walk. Boxes lie in the frame of the box asked
// about. The board gives the world one; collisions read it.
type Field interface {
	Solid(layers Layers, box geom.AABB, visit func(FieldBox) bool)
}

// FieldBox is one solid box of a Field; see collide.FieldBox, which it is.
type FieldBox = collide.FieldBox
