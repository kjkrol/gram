// Package look is how a board is drawn. A [Map] — the board's simple map, a topography's — says how
// the cells lie on the screen ([Look]), what lies over them beyond their sprites ([Dressing]) and
// how high they stand; the [Renderer] reads the board as a [Board] and hands the Look every visible
// cell as a [Tile] — its box, its sprite, its kind — which the Look lays on the render.Ground
// tier: from above its sprite over its box ([FlatLook], the simple map's), or as the Map has it.
// A kind with a Sway — trees, set by an effect when the wind blows — leans its top with the wind
// ([Tile.Sway]). A Map in relief draws its ground itself and its Look lays nothing ([Nothing]).
//
// # Dressing
//
// The Map's Dressing lays what lies on the tiles beyond their sprites: the renderer hands it each
// frame first and takes from it the sheet the tiles are drawn from ([Tile].Atlas: the board's atlas
// or a sheet of the dressing's with the atlas on it), the tile asks it its [Tile.Base] and its
// [Tile.Light] and [Tile.FaceLight], and the Look has it lay what lies on the tile ([Tile.Dress]).
// The simple map's lays the ways as bands; a topography's the grounds blending, coasts, water
// glinting and running and the ways drawn across the cells, painted for its ground on the GPU.
// Without a Dressing's light a tile is drawn as it is; a sky over a flat board
// (atmosphere.Plugin.WithBoard) lights it by the hour.
//
// # Composed once
//
// A flat map seen from above whose Dressing lights every tile alike ([EvenLit]: the simple map's,
// a sky's over it) is composed once instead, every cell, and kept on the GPU (render.Still),
// composed anew only when a cell changes (Board.Changes); the renderer, a render.Direct at the
// Ground tier, draws it every frame in the Dressing's light, again past a wrapping world's seam,
// and the grid over it on the GPU.
//
// # The grid
//
// [RenderState] holds the renderer's live toggles, such as the grid: over a board composed once a
// shader draws it, a square grid's tiles darkened along their edges, a hex grid's edges as lines;
// over tiles composed every frame, on a square grid each tile outlined by the shader along its own
// edges at no piece of its own (render.Frame.Tile) — where a Dressing lays grounds or ways over it
// ([Tile.Covered]), outlined by the dressing over them instead (render.Frame.OutlineOn); on a hex
// grid the cells' outlines as lines on a tier just above the tiles. It is left out where a cell
// spans fewer than [MinGridCell] pixels on screen.
//
// # Parallel
//
// A Dressing that is [Parallel], under a Look that is a [ParallelLook], dresses the tiles on
// several goroutines at once ([Renderer.Workers]; as many as there are CPUs unless told otherwise,
// none for a few tiles): the renderer Warms every visible tile on its own goroutine, has the
// dressing Ready itself, then shares the tiles out in runs, each drawn by a Worker of the dressing
// and of the look into a frame of its own, appended in order — the picture one goroutine would
// draw, piece for piece — for a game's own dressing; gram's dress their tiles once.
package look
