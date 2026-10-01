package world

import "github.com/kjkrol/gram/plugins/world/rule/effect"

// Apply is the command to put Effect on the world itself — its own entity, the clock's: a state
// of the whole game, a lever pulled, an alarm, which rules read with During. A player gives it
// from a binding; a rule or a plan may Order it too.
type Apply struct{ Effect effect.Effect }
