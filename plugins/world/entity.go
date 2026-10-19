package world

import "github.com/kjkrol/gram/entity"

// What an entity carries, as package entity has it, under the world's own names.
type (
	Base     = entity.Base
	Position = entity.Position
	Velocity = entity.Velocity
	Z        = entity.Z
	Layers   = entity.Layers
	Eye      = entity.Eye
	Lie      = entity.Lie
)

// StepReach is how far an entity may travel in one tick, as a share of its own shorter side.
const StepReach = entity.StepReach

// LayersOf is the Layers an entity carries, every plane for one carrying none.
func LayersOf(p *Layers) Layers { return entity.LayersOf(p) }
