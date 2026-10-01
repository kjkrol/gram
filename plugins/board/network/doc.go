// Package network is what runs from cell to cell across a board — rivers, roads — as a graph over
// its grid: the cells it runs through ([Network.Set], a [Node] of a board kind, a width and a
// fade) and which neighbours each runs on to. A road [Network.Link]s its cells both ways; water
// [Network.Flow]s down, each cell to the one its water goes to, the last on to where it leaves
// the network — the sea — which is no part of it.
//
// A network is a way to make a board's layout, not a plugin: a game works one out — by hand, or
// as plugins/board/water drains a relief — and lays it on the board as its [Network.Ways], a
// cell.Way across every cell it runs through, running on as [Network.Links] says. Down a flow,
// [Network.Along] is how far a cell lies, 0 at the head of the longest flow into it to 1 at the
// last cell before it leaves: a river taking on the sea's look as it nears it (cell.Way.Mix).
// Where two networks run through the same cells ([Network.Crossings]) a road meets a river: a
// ford, a bridge.
//
// A road is found over the grid ([Route]: the cheapest way from cell to cell at a cost a game
// says, round what may not be crossed) and laid along its cells ([Network.Path]). Laid over a
// river ([Network.Across]) it is a way of its own where the river does not run and a
// cell.Crossing — a bridge — over the river's way where it does.
package network
