// Package render is the drawing side of gram: pure primitives that know nothing of Stages,
// Scenes or plugins. A Scene's Layers and a plugin's Renderer are made of these.
//
// # Renderer
//
// A [Renderer] is Init once, at registration, and Draw every frame. [SolidBackground] fills the
// screen with one color; [CachedRenderer] draws an inner Renderer once into an offscreen image and
// reuses it until Invalidate or the screen changes size, for a board that rarely changes; [TelemetryRenderer] prints the
// tick rate, the entity count and collisions a second from a running total.
//
// # Atlas and AtlasSource
//
// An [Atlas] is a sprite sheet built lazily: Register (or RegisterAt, into a slot issued
// elsewhere) records a [SpriteDrawer] at a texture size of its own, and Close is when the sheet
// is laid out and baked — so a slot issued late is as welcome as an early one, as long as it
// comes before Close. The drawn size is the entity's box; the texture size is resolution.
// [Solid], [Border], [Diamond], [Cross], [Hexagon], [Dot] and [Arrow] are ready-made drawers. [AtlasSource]
// is what a batch draws from: the sheet and each [SpriteID]'s UV rectangle.
//
// # Sorted
//
// A [Sorted] is a Renderer over several [Submitter]s — renderers that hand their quads to a
// [Sink] with a depth each instead of drawing — and draws them back to front as one picture, one
// DrawTriangles per run of quads sharing a sheet. It is the layer of an isometric view, where the
// terrain and the entities interleave and a wall in front hides a unit behind it; the board's and
// the world's renderers submit. Ties keep submission order, so a unit follows the tile it stands on.
//
// # QuadBatch
//
// A [QuadBatch] gathers textured quads from an AtlasSource, transformed through a camera.Camera,
// into one DrawTriangles call; AppendCorners takes four screen points already projected.
// [ProjectCorners] projects a world box at a height through a camera, [Billboard] is a sprite
// standing upright on a projected point — how an isometric view draws its entities. [VisitWrapImages] visits each image of a box on a wrapping world,
// so a sprite straddling a seam is drawn on both sides.
package render
