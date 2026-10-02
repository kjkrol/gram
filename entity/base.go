package entity

import (
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/gram/entity/kind"
)

// Base is what every entity in the world is made of: where it is, how it moves, which kind it was
// spawned from and what the space may do with it.
type Base struct {
	Pos    Position
	Vel    Velocity
	TypeID kind.ID
	Caps   aabbworld.Capability
}
