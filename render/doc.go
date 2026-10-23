// Package render is the drawing side of gram: primitives that know nothing of Stages, Scenes or
// plugins. What a Scene and a plugin draw are made of these. Everything is drawn on the GPU
// through WebGPU (internal/gpu), in WGSL shaders.
//
// # Drawers: Renderer and Picture
//
// A Scene's parts are [Drawer]s. A [Renderer] draws on the screen once a frame, in screen pixels;
// a [Picture] is what is in the world before a camera, drawn through the camera it is handed, so
// one picture serves a player's view, the halves of a split screen and a minimap alike.
// [SolidBackground] is a ready-made renderer; a [Reporter] adds lines to the telemetry panel
// (plugins/telemetry).
//
// # Feed and Surface
//
// A [Feed] is the world seen through a camera: a Picture drawn through the camera every frame
// into an image of the size whoever shows it gives it ([Surface]). It turns its pixels into the
// world and back (ToWorld, ToPixels), so whatever shows it knows nothing of its camera. A camera
// drawn in layers (camera.Layered) has a [Laid] picture composed once and drawn a layer at a time,
// each scaled up into its place; one whose figures are sharp (camera.Figured) has its ground drawn
// so, or whole at the camera's resolution, and what lies over it drawn at every shown pixel.
//
// # Font and DrawText
//
// A [Font] is a TrueType or OpenType face at a size ([NewFont]); [DefaultFont] is Go Regular at
// 14 pixels. [DrawText] draws a string in a colour, [Font.Measure] is how much room it takes.
// [DebugPrintAt] is the fixed debug text of the telemetry.
//
// # Atlas and AtlasSource
//
// An [Atlas] is a sprite sheet built lazily: Add records a [SpriteDrawer] in a [Sprited]'s slot at
// a texture size of its own, and Close lays the sheet out and bakes it. [Solid], [Border],
// [Diamond], [Cross], [Hexagon], [Dot] and [Arrow] are ready-made drawers. [AtlasSource] is what a
// Frame draws from: the sheet, each [SpriteID]'s rectangle and a white texel for plain colours.
//
// # Composer, Frame and Source
//
// A [Composer] is the Picture of a world, drawn from several [Source]s (a Stage's is
// world.Read.Picture, of every source its plugins draw). Each source hands its pieces to a
// [Frame] in screen pixels, each where it lies ([At]: [Frame.At], [Frame.Lifted] a height in the
// world; [OnTop] a mark at its level): sprites, lines, fans, soft quads, each in a [Shade] of
// [Light]; a [Frame.Edge]'s ends lie at heights of their own. What is drawn over what is the
// depth's: the opaque ground first, writing it, then the world back to front by where it lies,
// each piece tested against what lies nearer (its depth worked out on the GPU from its height),
// then what lies over the figures ([At.OverFigures], hidden by the ground alone), then the marks
// over all of it by level, at one level in the order they came. The order the sources come in
// changes nothing but what lies at exactly one place.
// Every piece is drawn with one shader, so a run of pieces on one sheet is one call. A [Direct]
// source draws a part of the picture itself, with a shader of its own, in the passes its Compose
// asks for ([Frame.DrawOpaque], [Frame.DrawAt], [Frame.ReadGround]), handed a [Target] and the
// frame's [Uniforms]; [NewMeshShaderWith] builds such a shader on the composer's library, drawn
// with [Image.DrawMesh] against a [Depth], whole or by ranges of its indices at one go
// (DrawMeshOptions.Ranges). [Frame.Branch] and [Frame.Append] let a source gather
// pieces on several goroutines. A Composer is [Laid]: Lay composes every source once, or all but
// the [Overlying] ones, which say their pieces lie over the figures (the figures, what falls or
// floats over them, the marks); DrawLayer draws the composition into a layer's image through the
// camera cropped to the layer's window, its pieces placed by the window's scale and offset, so
// they are as wide in shown pixels in every layer, the ground's depth kept when the Overlying ones
// were left out; DrawOver draws those onto the screen with the uniforms and images the
// composition set, each tested against the layers' depths placed in the screen's
// ([Image.PlaceDepth]). A [Glass] source draws on the glass over the picture
// ([Composer.DrawGlass], [Feed.DrawGlass]): onto the picture as it is shown, after whatever a
// scene lays over it, so nothing covers what it draws.
//
// A color.RGBA given to render — a piece, a fill, text, a sprite drawn on a [Canvas] — is
// straight alpha, as colours are written: {R: 255, G: 220, B: 60, A: 40} is a faint yellow. Go
// defines color.RGBA as premultiplied; render premultiplies it for the GPU itself.
//
// # Materials
//
// What is worked out per pixel is a material, WGSL a plugin or a game registers
// ([RegisterMaterials]); the composer's shader is its own part and every material registered,
// compiled once ([Compile], [ShaderSource]). [Frame.Overlay] lays over the sprite just added a
// quad for a material to work out, where its corners lie in the world ([World], [Box]);
// [Frame.Material] lays one on its own. A [Look] is a drawer or a [MaterialID]. A material's
// uniforms are set by a source ([Frame.Uniform]); the composer's own are Toward, Clock
// ([Frame.Time]), Pixel and Fog ([Frame.Fog]).
//
// # Frame images
//
// What every shader of a frame reads that is too big for uniforms is a frame's image: a name
// registered as a package starts ([RegisterImage]), read in WGSL as a texture_2d<f32> of that name
// (sampled through linear, or wrapped, repeating past its edges), handed by a source composing
// the frame ([Frame.Image]) to the composer and every Direct alike, through the [Uniforms]. An
// image whose pixels need the GPU is filled as a draw first reads it ([Image.Defer]). The
// ground's heights and the sun on it are render's own ([GroundHeights], [Frame.GroundHeights],
// [GroundShade], [Frame.GroundShade]): whoever draws the ground hands the heights, whoever bakes
// the shade the shade; a shader reads them with groundHeightAt (groundDrawnAt as the ground is
// drawn), and with groundShadeAt and standingShadeAt how much of the sun reaches a point at a
// height over the ground past the relief's shadows and the blocks'. What a drawn thing takes of
// them is its [Taking], bits ([Illumination] carries it with its light and the sun's share of
// it; WGSL takes(t, bit)).
//
// # Sprites
//
// [Sprites] is a Direct source's sprites
// drawn as instances, each turning into the frame's Fog colour as far as [Sprites.Fog] says and
// taking the shade the ground holds as [Sprites.Take] and [Sprites.Direct] say.
//
// # Appearance and Rules
//
// [Appearance] is the sprite an entity is drawn from and how it sways. A [Rule] says, every
// frame, how the entities a renderer draws are drawn, each reading one component of the entity:
// [Over], [As], [Swap], [With] and [Show], each taking conditions of it. A renderer runs them
// through [Rules]: Bind adds what they read to its query, [Own] shares a column it reads itself,
// Run settles each chunk.
package render
