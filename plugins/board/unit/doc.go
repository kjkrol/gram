// Package unit is an entity on a board: the cell it is [At], how it moves over the cells (a
// [Mover]) and where it stands at a step as a rule gets it ([Standing]; its words [Fallen] where
// its domain may not be, [StandingOn] a cell under an effect, [OnKind] a cell of a kind).
// [Entered] and [Airborne] are markers of the board's [States] every unit carries: on for the
// step a unit came into another cell, and while it is in the air.
package unit
