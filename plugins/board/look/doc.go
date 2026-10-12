// Package look is how a board is drawn. A [Map] says how the cells lie on the screen ([Look]),
// what lies over them beyond their sprites ([Dressing]) and how high they stand; the [Renderer]
// reads the board as a [Board] and hands the Look every visible cell as a [Tile], which the Look
// lays on the render.Ground tier: from above its sprite over its box ([FlatLook], the simple
// map's), or as the Map has it. A Map drawing its ground some other way has the Look [Nothing].
//
// # Dressing
//
// The Map's Dressing lays what lies on the tiles beyond their sprites: the sheet they are drawn
// from, their [Tile.Base], their light ([Tile.Light], [Tile.FaceLight]) and whatever lies on
// them ([Tile.Dress]). A flat map seen from above whose Dressing lights every tile alike
// ([EvenLit]) is composed once and kept on the GPU, composed anew only when a cell changes.
//
// # The grid
//
// [RenderState] holds the renderer's live toggles, the grid among them. It is left out where a
// cell spans fewer than [MinGridCell] pixels on screen.
//
// # Parallel
//
// A Dressing that is [Parallel], under a Look that is a [ParallelLook], dresses the tiles on
// several goroutines at once ([Renderer.Workers]), each drawing its run into a frame of its own
// appended in order, so the picture is the one goroutine would draw.
package look
