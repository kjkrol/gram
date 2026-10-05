package rule

import "github.com/kjkrol/gram/entity/tag"

// testTags are the tags Role hands out, by name.
var testTags = map[string]tag.Tag[Roles]{}

// Role is the role named name with a tag of its own, one a name: what a Stage's world makes
// (world.Roles), for the tests of this package.
func Role(name string) *Part {
	t, ok := testTags[name]
	if !ok {
		t = tag.Tag[Roles](len(testTags))
		testTags[name] = t
	}
	return NewPart(name, t)
}

// Within is r for the entities carrying t alone, as a role's Obeys narrows its rules.
func Within[F any](t tag.Tag[F], r Rule) Rule { return within(t, r, "within") }
