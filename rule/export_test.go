package rule

import "github.com/kjkrol/gram/entity/tag"

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
