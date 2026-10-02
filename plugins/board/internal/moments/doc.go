// Package moments runs the board's passes over its units and cells: Rules hosts the rules a game
// hooks of a unit.Standing and of a cell.Now and runs them every step over the units on the board
// and over every cell, telling each, in its Tick, which cells lie round it (Here, Around). The
// same pass over the units writes each one's steering.Pace — the cost and the slope of the ground
// under it — and logs a fall when told to.
package moments
