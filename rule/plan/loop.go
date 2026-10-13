package plan

import (
	"time"

	"github.com/kjkrol/gram/rule/act"
	"github.com/kjkrol/gram/rule/cond"
)

// Loop is what an entity does over time on the fact P (any for none): Then run from the start
// as When comes to hold, going on until Until holds, failing after Timeout, not beginning
// again within Cooldown of its last run. A loop of a fact begins once as the fact comes to hold
// its When and runs to the end; one of any runs while its When holds.
type Loop[P any] struct {
	Name     string        // required at the top of a plan
	When     cond.Cond     // required: cond.Self.If(cond.Always) for every time
	Then     Part          // required
	Until    cond.Cond     // ends well once it holds; a fact in it is one come afresh
	Timeout  time.Duration // fails once it has run this long
	Cooldown time.Duration // does not begin again within this of its last run
}

// Lasting is the loop as the plan is built (Part).
func (l Loop[P]) Lasting() act.Lasting {
	var then act.Lasting
	if l.Then != nil {
		then = l.Then.Lasting()
	}
	return act.NewLoop[P](l.Name, l.When, then, l.Until, l.Timeout, l.Cooldown)
}
