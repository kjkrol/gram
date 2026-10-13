package cond

import "github.com/kjkrol/gram/entity/tag"

// Roles is the family of roles: one tag.Tags[Roles] on an entity holds every role it plays.
type Roles struct{}

// Role is a role an entity plays, a rule.Part: a state Self, Other or a place is in.
type Role interface{ Tag() tag.Tag[Roles] }
