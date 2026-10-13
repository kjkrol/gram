package act

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
)

func Within[F any](t tag.Tag[F], r Rule) Rule { return within(t, r, "within") }

// WithOther is the pair rule r with the other's side a tag of any family.
func WithOther[F any](t tag.Tag[F], r Rule) Rule {
	return Build(r).(interface{ sided(plugin.Side) Rule }).sided(plugin.SideOf(t))
}
