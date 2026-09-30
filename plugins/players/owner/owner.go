package owner

import (
	"fmt"

	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// Family is the owners' tag family: a tag a player, the players an entity belongs to.
type Family struct{}

// Players is how many players can own entities: one tag each in the family.
const Players = tag.MaxTagsPerFamily

// Of is the tag of player id; ids run from 1 to Players.
func Of(id control.PlayerID) tag.Tag[Family] {
	if id == control.Nobody || int(id) > Players {
		panic(fmt.Sprintf("owner: player %d owns nothing; ids run from 1 to %d", id, Players))
	}
	return tag.Tag[Family](id - 1)
}

// Name is the name the tag of player id is saved by.
func Name(id control.PlayerID) string { return fmt.Sprintf("player %d", id) }

// Obeys reports whether an entity owned by owners takes commands from player by: from its owners
// alone, and one nobody owns from control.Nobody alone.
func Obeys(owners tag.Tags[Family], by control.PlayerID) bool {
	if owners == 0 {
		return by == control.Nobody
	}
	return by != control.Nobody && int(by) <= Players && owners.Has(Of(by))
}

// Allies reports whether entities owned by a and by b are on one side: they share an owner, or
// nobody owns either — both are the virtual player's.
func Allies(a, b tag.Tags[Family]) bool {
	return a == 0 && b == 0 || a&b != 0
}
