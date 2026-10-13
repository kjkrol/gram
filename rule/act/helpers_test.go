package act_test

import (
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/cond"
)

var testTags = map[string]tag.Tag[cond.Roles]{}

// Role is the role named name, one tag a name.
func Role(name string) *rule.Part {
	t, ok := testTags[name]
	if !ok {
		t = tag.Tag[cond.Roles](len(testTags))
		testTags[name] = t
	}
	return rule.NewPart(name, t)
}
