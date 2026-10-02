package rule

import "github.com/kjkrol/gram/entity/tag"

// Within is r for the entities carrying t alone, as a role's Obeys narrows its rules.
func Within[F any](t tag.Tag[F], r Rule) Rule { return within(t, r, "within") }
