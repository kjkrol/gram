// Package unit is an entity on a board: the cell it is [At], how it moves over the cells and what
// it weighs (a [Mover]) and where it stands at a step as a rule gets it ([Standing], with its
// Heading, so a rule's board.Ahead(n) turns to the cells in front of it; its words [Fallen] where
// its domain may not be, [StandingOn] a cell under an effect, [OnKind] a cell of a kind).
// [Entered] and [Airborne] are markers of the board's [States] every unit carries: on for the
// step a unit came into another cell, and while it is in the air.
//
// # Vocabulary
//
// Moments, for a role's rules: [Standing] (every unit a step; places board.Around and
// board.Ahead; its words [Fallen], [StandingOn], [OnKind]).
package unit
