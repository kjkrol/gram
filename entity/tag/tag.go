package tag

// Tags is one family of tags on an entity: a component holding up to 64 bits, one per tag of
// the family F — an empty type a plugin or a game names the family by. An entity carries the
// family when it has any of its tags, so a query over Tags[F] narrows to those entities, and
// giving or taking a tag inside the family is a value write, seen the same tick.
type Tags[F any] uint64

// Tag is one bit of a family: what world.Kinds.DefineTag hands out and Between matches on.
type Tag[F any] uint8

// MaxTagsPerFamily is how many tags one family holds, bounded by Tags.
const MaxTagsPerFamily = 64

// Has reports whether t is set.
func (s Tags[F]) Has(t Tag[F]) bool { return s&(1<<t) != 0 }

// In reports whether s carries t: t.In is a condition of the entities carrying it, for a
// render.Rule.
func (t Tag[F]) In(s Tags[F]) bool { return s.Has(t) }

// With returns s with every tag set.
func (s Tags[F]) With(tags ...Tag[F]) Tags[F] {
	for _, t := range tags {
		s |= 1 << t
	}
	return s
}

// Without returns s with every tag cleared.
func (s Tags[F]) Without(tags ...Tag[F]) Tags[F] {
	for _, t := range tags {
		s &^= 1 << t
	}
	return s
}

// Anything is the family of Any, the tag every entity carries.
type Anything struct{}

// Any stands for "whatever it is" on one side of Between, or both.
var Any Tag[Anything]
