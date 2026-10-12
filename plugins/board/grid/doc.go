// Package grid is a board's topology: a [Grid] answers which cells neighbour which and in which
// direction, where a cell lies, how far apart two are, which cells a box covers and every cell in
// turn; [DefaultGrids] makes a square or a hex one. A grid knows its own directions, and nothing
// outside it spells them out: [Way] and [Link] name the direction to a neighbour, [Direction] and
// [Nearest] the one of an offset or a vector, [Exit] and [Rim] where it leaves a cell, [Straight]
// the one links run straight through by. [ShapeOf] tells a default grid's [Shape].
package grid
