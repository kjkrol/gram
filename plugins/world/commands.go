package world

import "github.com/kjkrol/gram/rule/effect"

// Despawn is the command an entity gives itself to leave the world: gone in the step it gives it.
type Despawn struct{}

// Apply is the command to put Effect on the world itself — its own entity, the clock's: a state
// of the whole game, a lever pulled, an alarm, which rules read with During. A player gives it
// from a binding; a rule or a plan may Order it too.
type Apply struct{ Effect effect.Effect }

// Dispel is the command to take Effect off the world itself: the state of the whole game Apply
// put on ends, the lever goes back.
type Dispel struct{ Effect effect.Effect }
