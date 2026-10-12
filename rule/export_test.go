package rule

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/plugin"
)

var testTags = map[string]tag.Tag[Roles]{}

func Role(name string) *Part {
	t, ok := testTags[name]
	if !ok {
		t = tag.Tag[Roles](len(testTags))
		testTags[name] = t
	}
	return NewPart(name, t)
}

func Within[F any](t tag.Tag[F], r Rule) Rule { return within(t, r, "within") }

// PairedWith is Paired for a tag of any family.
func PairedWith[F any](t tag.Tag[F]) Cond { return sides{other: plugin.SideOf(t), paired: true} }
