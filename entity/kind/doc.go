// Package kind is how a game says what its entities are. A [Spec] lists the components a kind
// carries, each made in package comp (the same for all, comp.Const, or read from the entity's
// own row, comp.Load); [Define] registers it with a world, and the kind's Entry puts one entity
// on the roster.
//
// # Define, Named and Of
//
// [Define] registers a Spec under a name with a [Registry], a world's; [Named] is the kind as
// [Of], typed by its row: Entry builds one [Entry] for world.Plugin.Seed with the row checked by
// the compiler, ID is the [ID] every entity of the kind carries, SpriteID the atlas slot its
// entities are drawn from. A world.Position and a world.Velocity must each appear once in a
// Spec, or Define panics by name; so does a Load over a row type other than the kind's.
//
// # Roster
//
// A [Roster] is what the plugins of a world ask of the entities a game makes, gathered as the
// plugins are made. Its [Template] for a unit lists what plugins require ([Require]: the game
// must supply a component of that type) and what they bring themselves (Template.Default, which
// the game may override or leave out with comp.Without); Template.Spec builds the Spec from both
// and panics naming every requirement left unmet. Its Template for a cell is what every cell a
// board makes carries besides the board's own components.
//
// # Registry
//
// [Registry] is where kinds are kept, behind an interface because kind never imports world. One
// Registry holds at most [MaxKinds] kinds.
package kind
