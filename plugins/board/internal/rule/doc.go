// Package rule runs the rules a game hooks on the board, written with package rule: Rules
// hosts those of a unit.Standing and of a cell.Now and runs them every step over the units on the
// board and over every cell, telling each, in its Tick, which cells lie round it (Here, Around);
// TerrainSpeed is the board's own rule on the world, slowing every unit by the ground under it.
package rule
