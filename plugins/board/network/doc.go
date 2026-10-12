// Package network is what runs from cell to cell across a board, rivers and roads, as a graph
// over its grid: the cells it runs through ([Network.Set], a [Node] of a board kind, a width and
// a fade) and which neighbours each runs on to; a road [Network.Link]s its cells both ways, water
// [Network.Flow]s down to where it leaves the network. It is a way to make a board's layout, not
// a plugin: a game works one out, by hand or as plugins/board/water drains a relief, and lays it
// on the board as its [Network.Ways]; [Network.Along] is how far down its flow a cell lies,
// [Network.Crossings] where two networks meet. A road is found over the grid with [Route] and
// laid along its cells with [Network.Path].
package network
