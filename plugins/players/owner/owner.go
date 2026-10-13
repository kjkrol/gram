package owner

import (
	"fmt"

	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/tag"
)

// Family is the owners' tag family: a tag a player, the players an entity belongs to.
type Family struct{}

// Players is how many players can own entities: one tag each in the family.
const Players = tag.MaxTagsPerFamily

// Of is the tag of player id; ids run from 1 to Players.
func Of(id command.Issuer) tag.Tag[Family] {
	if id == command.Nobody || int(id) > Players {
		panic(fmt.Sprintf("owner: player %d owns nothing; ids run from 1 to %d", id, Players))
	}
	return tag.Tag[Family](id - 1)
}

// Name is the name the tag of player id is saved by.
func Name(id command.Issuer) string { return fmt.Sprintf("player %d", id) }

// Obeys reports whether an entity owned by owners takes commands from player by: from its owners
// alone, and one nobody owns from command.Nobody alone.
func Obeys(owners tag.Tags[Family], by command.Issuer) bool {
	if owners == 0 {
		return by == command.Nobody
	}
	return by != command.Nobody && int(by) <= Players && owners.Has(Of(by))
}

// Allies reports whether entities owned by a and by b are on one side: they share an owner, or
// nobody owns either.
func Allies(a, b tag.Tags[Family]) bool {
	return a == 0 && b == 0 || a&b != 0
}
