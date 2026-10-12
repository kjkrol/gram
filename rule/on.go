package rule

import (
	"fmt"

	"github.com/kjkrol/gram/internal/steps"
)

// On is whom a Branch's Do acts on: the entity itself, the other of its pair or the subject of
// its fact, or the places round it. A Branch at the top of a rule or a plan must say; one inside
// another's Do acts on what the outer one does unless it says otherwise, and may switch once.
type On struct {
	kind  onKind
	rings int
}

type onKind uint8

const (
	inherit onKind = iota
	onSelf
	onOther
	onAround
)

var (
	// Self is the entity the rule or the plan is about.
	Self = On{kind: onSelf}
	// Other is the other of a pair the moment met, or the subject of the fact the branch
	// stands on.
	Other = On{kind: onOther}
	// Here is each place the entity stands on, in a rule of a moment that is plugin.Placed.
	Here = On{kind: onAround}
)

// Around is each place within rings of where the entity stands, in a rule of a moment that is
// plugin.Placed.
func Around(rings int) On { return On{kind: onAround, rings: rings} }

// String says whom.
func (o On) String() string {
	switch o.kind {
	case onSelf:
		return "Self"
	case onOther:
		return "Other"
	case onAround:
		if o.rings == 0 {
			return "Here"
		}
		return fmt.Sprintf("Around(%d)", o.rings)
	}
	return ""
}

// wrap is node acting on whom o says; top, an explicit Self is nothing
func (o On) wrap(node steps.Step, top bool) steps.Step {
	switch o.kind {
	case onSelf:
		if top {
			return node
		}
		return steps.NewSame(node)
	case onOther:
		return steps.NewForOther(node)
	case onAround:
		return steps.NewAround(o.rings, node)
	}
	return node
}
