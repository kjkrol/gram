package world

import "github.com/kjkrol/gram/entity"

// What an entity carries, as package entity has it — re-exported, so the world's own name for
// each stays: Base, Position, Velocity, Z, Layers and Eye are the same types wherever they are
// named.
type (
	Base     = entity.Base
	Position = entity.Position
	Velocity = entity.Velocity
	Z        = entity.Z
	Layers   = entity.Layers
	Eye      = entity.Eye
)

// StepReach is entity.StepReach: how far an entity may travel in one tick, as a share of its own
// shorter side.
const StepReach = entity.StepReach

// LayersOf is the Layers an entity carries, every plane for one carrying none.
func LayersOf(p *Layers) Layers { return entity.LayersOf(p) }
