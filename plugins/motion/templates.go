package motion

import (
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugins/world"
)

// UnitTemplate is the template of the kinds that move, steered, a world's entity besides.
var UnitTemplate = kind.NewTemplate("unit", world.EntityTemplate)
