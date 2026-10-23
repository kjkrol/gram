// Package render is the drawing side of gram, knowing nothing of Stages, Scenes or plugins: what
// a Scene and a plugin draw with. Everything is drawn on the GPU (internal/gpu), in WGSL.
//
// A Scene's parts are [Drawer]s. A [Renderer] draws on the screen once a frame; a [Picture] is
// what is in the world before a camera, drawn through the camera it is handed and shown by a
// [Feed] at a size ([Surface]): whole, in layers or with its figures sharp. A [Reporter] adds
// lines to the telemetry panel.
//
// The world's picture is a [Composer] of [Source]s, each handing its pieces to a [Frame] where
// they lie ([At]), drawn back to front against the depth; a [Direct] source draws its part
// itself with a shader of its own ([NewMeshShaderWith], [Image.DrawMesh]), [Sprites] as
// instances. What is worked out per pixel is a material ([RegisterMaterials]), what every shader
// of a frame reads beside the uniforms an image ([RegisterImage], [Frame.Image]).
//
// Pictures are [Image]s on the GPU, drawn into and filled ([Image.FillRect] and kin); sprites
// are painted on the CPU into an [Atlas] ([SpriteDrawer], [Solid] and kin), text in a [Font]. A
// [Look] is a drawer, a material or frames ([Phases], [Loops]); an [Appearance] and its [Rule]s
// say how an entity is drawn. A color.RGBA given to render is straight alpha; render
// premultiplies it for the GPU.
//
// The machinery is render/internal's: wgsl (the shaders' library, the materials and the frame's
// images), gfx (images, depths, meshes, shaders), atlas, text, feed, compose (the composer and
// its frame) and instanced (sprites as instances); render names their types by alias.
package render
