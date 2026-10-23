// Package look is how a board is drawn. A [Map] says how the cells lie ([Look]: [Flat],
// [Swaying] or [Nothing], for a Map drawing its ground some other way), what lies over them
// beyond their sprites ([Dressing]) and how high they stand; the [Renderer] reads the board as a
// [Board] and lays its tiles on the GPU every frame out of an image of its cells, a cell written
// anew only when it changes: its sprite over its box, leant with the wind where the Look is
// [Swaying], lit per pixel as it takes the frame's light, faded into the fog of the sectors seen;
// a board of no world, or seen through a camera without lines of sight, draws nothing.
// [Renderer.Whole] draws the whole board from above into an image.
//
// # Dressing
//
// The Map's Dressing picks the sheet the tiles are drawn from and each cell's base sprite; a
// [Laid] one hands the ways and the covers, an [Overlaid] one a material over some tiles, and a
// [GroundDressing] lays on the ground itself, right after it, what is part of it: the clouds'
// shadows. Where tiles overlap, each one and what lies on it lie over the cells before it, row by
// row, the ways over all of them.
//
// # The grid
//
// [RenderState] holds the renderer's live toggles, the grid among them, and the line the grid is
// drawn in ([RenderState.Grid]). It is left out where a cell spans fewer than [MinGridCell]
// pixels on screen.
package look
