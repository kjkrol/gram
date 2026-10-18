// Package cell is a board's cell as a place: what it is, who may stand on it, what runs across
// it and the moment of it rules are written for. The board (plugins/board) lays its grid of them
// and keeps their entities.
//
// # ID, Kind and Domain
//
// An [ID] names one cell. A [Kind] is a named terrain: whom it admits (Allows, of the [Domain]s
// [Land], [Water] and [Air], a game adding bits of its own), what a step costs, whether it is
// solid, how much it veils sight; a board's [Kinds] holds those a game defined, by [Name].
// [Ground] is the cell's kind on its entity, beside its [Plot], and a [Way] (a road, a river)
// runs across it. [Terrain] is what a cell answers about its kind: a board is one.
//
// # Terrain in maps and the Layout's entries
//
// A [TerrainMap] is terrain in plain maps: a board's seed, and a [Terrain] of its own where no
// board is wanted. The board's Layout names kinds by name, an [Entry] per cell and a [WayEntry]
// per way; an Entry also gives its cell what it is called and the roles it plays beyond its
// kind's:
//
//	cell.Entry{Kind: "well", Cell: c, Name: "well"}
//	water.Entry(c).Plays(lakeRole)
//
// # What stands on a cell
//
// An entity standing on the board carries [On], the cell it stands on. Its knobs [Floor], [Wall]
// and [Rail] build over the cell — a trapdoor over a pit, a bridge's deck, a gate, railings —
// gathered on the cell's entity as [Built], whose Over is the kind as whoever crosses the cell
// meets it. An effect opens one by altering a Floor's Allows or a Wall's Keeps to 0.
//
// # Now
//
// [Now] is the cell at a step as a rule gets it, run for every cell playing a role that obeys
// one: board.Around(0) acts on the cell, board.Around(n) on the rings round it.
//
// # Occupancy
//
// An [Occupancy] tracks who holds each cell, on which level and in which domains:
// [SingleOccupancy] lets one entity per domain in on each level, [MultipleOccupancy] any number.
//
// # Vocabulary
//
// Moments, for a role's rules: [Now] (every cell a step; places board.Around). Knobs, for an
// effect's Alter: [Ground], [Floor], [Wall], [Rail].
package cell
