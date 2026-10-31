// Package pathfind finds routes over a board's grid for the navigation: the cheapest way from cell
// to cell for an entity moving in a domain, round what may not be entered, what is held and what
// the caller says is blocked, and the nearest free cell to a taken one. A Field is the way to one
// goal from the cells round it, filled once for a group and walked as each of its units' route.
//
// A unit within a cell takes every step but one along a deck's band narrower than it is across
// the step; one bigger than a cell stands on a node only where every cell of its footprint's
// window takes it, laid round the node for an odd side and from it right and down for an even
// one, a slantwise step's flanks too.
package pathfind
