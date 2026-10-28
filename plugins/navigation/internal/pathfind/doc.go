// Package pathfind finds routes over a board's grid for the navigation: the cheapest way from cell
// to cell for an entity moving in a domain, round what may not be entered, what is held and what
// the caller says is blocked, and the nearest free cell to a taken one. A Field is the way to one
// goal from the cells round it, filled once for a group and walked as each of its units' route.
//
// A unit is no bigger than a cell's side (board.NewUnits refuses a bigger one), so every step
// takes it but one along a deck's band narrower than the unit is across the step.
package pathfind
