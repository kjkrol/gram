// Package moments runs the board's passes over its units and cells: Rules hosts the rules of a
// unit.Standing and of a cell.Now and runs them every step, telling each which cells lie round it
// (Here, Around). The pass over the units writes each one's steering.Pace and logs a fall when
// told to.
package moments
