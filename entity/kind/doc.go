// Package kind is how a game says what its entities are. A [Spec] lists the components a kind
// carries, each made in package comp (the same for all, comp.Const, or read from the entity's
// own row, comp.Load); [Define] registers it with a world, and the kind's Entry puts one entity
// on the roster.
//
// # Define, Named and Of
//
// [Define] registers a Spec under a name with a [Registry], a world's; [Named] is the kind as
// [Of], typed by its row: Entry builds one [Entry] for world.Write.Seed with the row checked by
// the compiler, ID is the [ID] every entity of the kind carries, SpriteID the atlas slot its
// entities are drawn from. A world.Position and a world.Velocity must each appear once in a
// Spec, or Define panics by name; so does a Load over a row type other than the kind's.
//
// # Roster
//
// A [Template] is a sort of kind a plugin brings in by name ([NewTemplate]: what the world
// spawns, what moves, a board's cell), under a parent whose asks come with its own; the package
// knows none. A [Roster] is what the plugins of a world ask of the kinds of each template: what
// they require ([Require]: the game must supply a component of that type) and what they bring
// themselves ([Roster.Default], which the game may override or leave out with comp.Without). It
// is written once, as it is first read, by every plugin's KindTemplates in the order made, and
// refuses a write anywhere else; [Roster.Spec] builds a kind's Spec of a template and panics
// naming every requirement left unmet.
//
// # Registry
//
// [Registry] is where kinds are kept, behind an interface because kind never imports world. One
// Registry holds at most [MaxKinds] kinds.
package kind
