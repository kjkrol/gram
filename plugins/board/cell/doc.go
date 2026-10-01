// Package cell is a board's cell as a place: what it is, who may stand on it, what runs across it,
// the game's tags it carries and the moment of it rules are written for. The board (plugins/board)
// lays its grid of them and keeps their entities; this package is what is said of one.
//
// # ID, Kind and Domain
//
// An [ID] names one cell. A [Kind] is a named terrain: whom it admits (Allows, of the [Domain]s
// [Land], [Water] and [Air], a game adding bits of its own), what a step costs, whether it is
// solid, how much it veils sight, how it looks; a board's [Kinds] holds those a game created,
// by [Name]. [Ground] is the cell's kind on its entity, beside its [Plot] — an effect altering it
// alters the board — and a [Way] (a road, a river) runs across it under any [Crossing] (a bridge).
//
// # Tags and Now
//
// A cell carries for good the game's tags of places ([Family]: a [Tag], [Tags]) — a trapdoor, a
// plate, a zone — given in the board's Layout. [Now] is the cell at a step as a rule gets it: the
// board runs the rules of it hooked on its plugin for every cell, filtered by those tags or by
// effects' markers (rule.Self); it is rule.Placed, so a rule's Here acts on the cell and Around on
// the rings round it.
package cell
