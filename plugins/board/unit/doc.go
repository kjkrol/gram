// Package unit is an entity on a board: the cell it is [At], how it moves over the cells (a
// [Mover]: its domains and, in a world with heights, how high it keeps) and where it stands at a
// step as a rule gets it ([Standing]: the cell under it, the cell's kind and the game's tags of its
// place, its box, its domain; [Standing.Fallen] where its domain may not be). The board
// (plugins/board) keeps them: board.NewUnits gives At and Mover to a game's units, and the board
// runs the rules of a Standing every step, for every entity carrying At.
package unit
