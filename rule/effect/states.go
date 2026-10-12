package effect

import "github.com/kjkrol/gram/entity/tag"

// States is the family of an entity's effect markers: Changed and each effect's own
// (Effect.Mark).
type States struct{}

// Changed is on for the one step after an effect changed the entity's components, as an Alter
// began or ended.
const Changed tag.Tag[States] = 0

const changedName = "effect.changed"
