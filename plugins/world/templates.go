package world

import "github.com/kjkrol/gram/entity/kind"

// EntityTemplate is the template of every kind the world spawns, the parent of a plugin's own.
var EntityTemplate = kind.NewTemplate("entity", nil)
