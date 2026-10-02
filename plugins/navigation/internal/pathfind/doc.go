// Package pathfind finds routes over a board's grid for the navigation: the cheapest way from cell
// to cell for an entity moving in a domain, at the price of every step — its kind's cost, the
// slope, the ground beside a way when the step cuts a corner — round what may not be entered,
// what is held and whatever the caller says is blocked, and the nearest free cell to a taken one.
package pathfind
