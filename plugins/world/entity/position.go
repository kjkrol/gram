package entity

import "github.com/kjkrol/aabbworld/plane"

// Position is an entity's world-space rectangle — shared by every plugin
// that places or draws entities (physics, board, ...).
type Position struct {
	plane.AABB
}

// StepReach is how far an entity may travel in one tick, as a share of its own
// shorter side. Half a body keeps two entities from ever crossing centres unseen.
const StepReach = 0.5

// MaxStep is the furthest this entity moves in a single tick, whatever its Velocity says.
func (p Position) MaxStep() float64 { return StepReach * min(p.Size.X, p.Size.Y) }

// MaxSpeed is the fastest this entity can travel, in world units a second, at tps ticks a second.
func (p Position) MaxSpeed(tps int) float64 { return p.MaxStep() * float64(tps) }
