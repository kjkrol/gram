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
// alters the board — and a [Way] (a road, a river) runs across it.
// [Terrain] is what a cell answers about its kind: a board is one.
//
// # Terrain in maps and the Layout's entries
//
// A [TerrainMap] is terrain in plain maps — kinds, ways, tags, roles and labels by cell: a board's
// seed until the ECS makes every cell an entity out of it, and a [Terrain] of its own where no
// board is wanted. The board's Layout names kinds by name: an [Entry] per cell, a [WayEntry] per
// way. An Entry also gives its cell, for good, what it is called — a Name of its own, a Group it
// shares — which its entity carries as an entity.Label and commands find it by (entity.Named,
// Group); and the roles it plays, whose rules it obeys, beyond its kind's (Kinds.Define) —
// Entry.Plays, this one cell's own:
//
//	cell.Entry{Kind: "well", Cell: c, Name: "well"}
//	cell.Entry{Kind: "fence", Cell: d, Group: "east fence"}
//	water.Entry(c).Plays(lakeRole) // built on the kind's handle (Kinds.Named, Of.Entry)
//
// # What stands on a cell
//
// An entity standing on the board — a lever, a trapdoor, a gate, a plate — carries [On], the cell
// it stands on for good (board.NewEntities makes such kinds). Its knobs [Floor] and [Wall] build
// over the cell: a Floor carries the domains it Allows whatever the ground takes (a trapdoor over
// a pit, a ford waded at its Cost, a bridge over a river, Graded and at its Height in a world with
// heights — stepped onto from any side within a step, a road under it passing under its slab,
// Thickness thick), a Wall keeps the domains it Keeps off (a gate shut), a [Rail] stands railings
// along the sides it names (a bridge's long sides: a barrier for what drives, cleared by a leap
// over its Height). The board gathers them on
// the cell's entity as [Built], every step, and [Built.Over] is the kind as whoever crosses the
// cell meets it with them: the planner, a fall (unit.Standing.Fallen) and collision see them as
// they see the ground. An effect opens one — a Floor's Allows or a Wall's Keeps altered to 0:
//
//	fx.Define(OpenEf, effect.Spec{effect.Alter(func(f *cell.Floor) { f.Allows = 0 })})
//
// # Now
//
// [Now] is the cell at a step as a rule gets it: the board runs the rules of it for every cell,
// obeyed by the roles the cell plays (rule.Part.Obeys) or filtered by effects' markers
// (rule.Self); it is plugin.Placed, so a rule's Here acts on the cell and Around on the rings
// round it, on what stands on them too: fire spreading from cell to cell.
//
// # Occupancy
//
// An [Occupancy] tracks who holds each cell and in which domains — the bookings navigation makes
// when it keeps units a cell each: [SingleOccupancy] lets one entity per domain in, a walker and a
// hawk sharing one, [MultipleOccupancy] any number. The board lets go of the holds of whoever left
// the world every step (Occupancy.Release).
package cell
