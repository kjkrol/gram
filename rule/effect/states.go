package effect

import "github.com/kjkrol/gram/entity/tag"

// States is the family of an entity's effect markers: states switched by a bit, never by a
// component put on or taken off — Changed, and each effect's own (Effect.Mark). The world gives it
// to every unit (comp.Marks), the board to every cell; an entity without it gets it the first time
// it is needed.
type States struct{}

// Changed is on for the one step after an effect changed the entity's components — an Alter began,
// or ended and gave the original back — so whoever reads them sees it whatever the order of the
// plugins' passes.
const Changed tag.Tag[States] = 0

// changedName is the name the world defines Changed under, as the saves know it.
const changedName = "effect.changed"
