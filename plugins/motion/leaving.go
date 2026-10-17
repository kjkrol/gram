package motion

import (
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// Outside marks an entity wholly past an open edge, taken off once it is back inside.
type Outside struct{}

// Leaving is the moment of an entity carrying Outside, every tick it does. With no rule of it
// motion despawns the leaver.
type Leaving struct {
	ID   uid.UID64
	Base *world.Base
}

// Who is the entity leaving.
func (l Leaving) Who() uid.UID64 { return l.ID }

var (
	_ host.About = Leaving{}
)
