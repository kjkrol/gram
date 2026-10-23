// Package look is how a board is drawn. A [Map] says how the cells lie on the screen ([Look]),
// what lies over them beyond their sprites ([Dressing]) and how high they stand; the [Renderer]
// reads the board as a [Board] and hands the Look every visible cell as a [Tile], which the Look
// lays on the ground: from above its sprite over its box ([FlatLook], the simple map's), or as
// the Map has it. A Map drawing its ground some other way has the Look [Nothing].
//
// # Dressing
//
// The Map's Dressing lays what lies on the tiles beyond their sprites: the sheet they are drawn
// from, their [Tile.Base], their light ([Tile.Light], [Tile.FaceLight]) and whatever lies on
// them ([Tile.Dress]). A flat map seen through a camera of rays whose Dressing lights every tile
// alike ([EvenLit]) is laid on the GPU every frame out of an image of its cells, a cell written
// anew only when it changes: its sprite, its way and its covers where the Dressing is [Laid],
// what stands on it leant with the wind where the Look is [Swaying], lit per pixel as it takes
// the frame's light. Any other is composed on the CPU, its visible tiles every frame. A
// [GroundDressing] lays on the ground itself, right after it, what is part of it: the clouds'
// shadows.
//
// # The grid
//
// [RenderState] holds the renderer's live toggles, the grid among them, and the line the grid is
// drawn in ([RenderState.Grid]). It is left out where a cell spans fewer than [MinGridCell]
// pixels on screen.
package look
