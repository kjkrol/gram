package board

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
)

// The board's templates: a kind standing still on it, and its cells.
var (
	StandingTemplate = kind.NewTemplate("standing", world.EntityTemplate)
	CellTemplate     = kind.NewTemplate("cell", nil)
)
