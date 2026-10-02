package plan

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/rule/internal/engine"
	"github.com/kjkrol/uid"
)

// Plans runs the plans of every entity with a Mind, once every step of the world's simulation, on
// the world's clock. The world makes and runs it; a game gives its kinds a plan with New.
type Plans struct{ e *engine.Plans }

// NewPlans is the plans over now, the world's clock's time, and world, the world's own entity,
// drawing their Chance from seed, casting the world's effects fx and giving the commands its plans
// order to commands, the world's carrier.
func NewPlans(now func() time.Duration, world func() uid.UID64, seed uint64, fx *effect.Effects, commands *control.Carrier) *Plans {
	return &Plans{e: engine.NewPlans(now, world, seed, fx, commands)}
}

// System is the plans' system, run in every step of the simulation.
func (p *Plans) System() goke.System { return p.e.System() }

// LoadComps lists what the plans save: the minds, and the asks of every plan written.
func (p *Plans) LoadComps() []goke.CompToken { return p.e.LoadComps() }
