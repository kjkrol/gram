// Package cell is a board's cell as a place: what it is, who may stand on it, what runs across it,
// the game's tags it carries and the moment of it rules are written for. The board (plugins/board)
// lays its grid of them and keeps their entities.
//
// # ID, Kind and Domain
//
// An [ID] names one cell. A [Kind] is a named terrain: whom it admits (Allows, of the [Domain]s
// [Land], [Water] and [Air], a game adding bits of its own), what a step costs, whether it is
// solid, how much it veils sight, how it looks; a board's [Kinds] holds those a game created,
// by [Name]. [Ground] is the cell's kind on its entity, beside its [Plot] — an effect altering it
// alters the board — and a [Way] (a road, a river) runs across it under any [Crossing] (a bridge).
// [Terrain] is what a cell answers about its kind: a board is one.
//
// # Terrain in maps and the Layout's entries
//
// A [TerrainMap] is terrain in plain maps — kinds, ways, crossings, tags, roles and wires by cell:
// a board's seed until the ECS makes every cell an entity out of it, and a [Terrain] of its own
// where no board is wanted. The board's Layout names kinds by name: an [Entry] per cell, a
// [WayEntry] per way or crossing. An Entry also gives its cell, for good, the roles it plays
// (Entry.Roles), whose rules it obeys, and the wire it is wired to (Entry.Wired, defined with
// world.Plugin.Wire), which its entity carries as a rule.Wired:
//
//	cell.Entry{Kind: "boards", Cell: c, Roles: []*rule.Part{trapdoor}, Wired: west}
//
// # Tags and Now
//
// A cell carries for good the game's tags of places ([Family]: a [Tag], [Tags]) — a zone, a
// district — given in the Layout (Entry.Tags). [Now] is the cell at a step as a rule gets it: the
// board runs the rules of it for every cell, filtered by those tags or by effects' markers
// (rule.Self), or obeyed by the roles the cell plays (rule.Part.Obeys); it is plugin.Placed, so a
// rule's Here acts on the cell and Around on the rings round it. Now.Trodden says the centre of a
// unit lies on the cell this step, and [Now.Stood] is that for a rule's If — a plate putting its
// wire on while someone stands on it:
//
//	m.If(cell.Now.Stood, m.OnWire(m.Apply(on)))
//
// # Occupancy
//
// An [Occupancy] tracks who holds each cell and in which domains — the bookings navigation makes
// when it keeps units a cell each: [SingleOccupancy] lets one entity per domain in, a walker and a
// hawk sharing one, [MultipleOccupancy] any number. The board lets go of the holds of whoever left
// the world every step (Occupancy.Release).
package cell
