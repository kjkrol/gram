package world

import "github.com/kjkrol/gram/entity/kind"

// The world's templates: every kind it spawns, and what moves of them.
var (
	EntityTemplate = kind.NewTemplate("entity", nil)
	UnitTemplate   = kind.NewTemplate("unit", EntityTemplate)
)
