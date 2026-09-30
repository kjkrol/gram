// Package kind is how a game says what its entities are. A Spec lists the components a kind
// carries, each made in package comp — the same for all (comp.Const) or read from the entity's
// own row (comp.Load); Define registers it with a world, and the kind's Entry puts one entity on
// the roster.
//
// # Spec
//
// A [Spec] is the list of a kind's components, each a comp.Comp — see package comp for Const,
// Load, Tagged and Without. A world.Position and a world.Velocity must each appear once, or
// Define panics by name; so does a Load over a row type other than the kind's.
//
// # Define and Of
//
// [Define] registers a Spec under a name with a [Registry] — a world's, reached through
// world.Plugin.Kinds — and hands back the kind as [Of], typed by its row: Entry builds one
// [Entry] for world.Plugin.Seed with the row checked by the compiler, ID is the [ID] every entity
// of the kind carries, SpriteID is the atlas slot its entities are drawn from, Name is what it
// was defined as.
//
// # Roster
//
// A [Roster] is what the plugins of a world ask of the kinds a game defines, gathered as the
// plugins are made — a world's, reached through world.Plugin.Roster. Its [Role] for a unit lists
// what plugins require ([Require]: the game must supply a component of that type, a Load or a
// Const) and what they bring themselves (Role.Default: a constant the game may override with its
// own of the same type, or leave out with comp.Without). Role.Spec builds the Spec from the defaults and
// the game's own components and panics naming every requirement left unmet, by plugin and reason,
// so a kind defined without its Cell hears "board requires board.Cell (the cell it starts in)".
// A plugin added to the game later brings its requirements along.
//
// # Registry
//
// [Registry] is where kinds are kept; kind never imports world (world imports kind), which is
// why a kind's id is [ID] and the registry sits behind this interface. It also lets saves know
// every component type a kind gives its entities. One Registry holds at most [MaxKinds] kinds.
package kind
