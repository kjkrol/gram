# Rendering: one composer per view — a design note

[← Back to README](../README.md)

> A proposal, not a contract, and not built. It records how the world is drawn today, what a single
> composer per viewport would change, and the questions to settle before writing it.

## 1. How a frame is drawn today

- A **Scene** lists its layers bottom to top (`game.Scene.Layers`). A `render.Renderer` draws once
  on the screen, in pixels: a background, a menu, telemetry. A `render.WorldRenderer` shows the
  world and is drawn once per `render.Viewport` — a camera and a rectangle of the screen — of a
  Scene that is a `game.Viewer`; the engine hands it the viewport's camera and an image the size of
  the viewport. No renderer keeps a camera.
- **Order is list order.** The board, then the entities, then the vision fans, the routes, the
  selection outlines. That is enough from above, where nothing stands in front of anything.
- **An isometric view needs depth.** There `render.Sorted` gathers the quads of the board and the
  world (`Submitter.Submit` into a `Sink`, each quad with a depth) and draws them back to front, so a
  wall in front hides a unit behind it. Everything after the `Sorted` layer — fans, routes,
  outlines — is still drawn over the whole picture: a route behind a mountain shows through it.
- **Primitives in use.** Textured quads from an atlas (`QuadBatch`, `Sink`), and vector paths
  filled or stroked (`vector.FillPath`, `vector.StrokePath`): the vision fans and their shadows, the
  selection outlines, the grid lines. There are no shaders.

## 2. The proposal

One composer per viewport. World renderers stop drawing: each hands the composer **draw items**
for the frame, and the composer sorts them and draws them in as few calls as the sheets allow.

- **Key.** An item carries `(layer, depth)`. Layers are named constants, ordered by value, not by
  the Scene's list:
  - `Ground` — the tiles, their faces and the grid lines on them;
  - `Objects` — what stands: tall terrain (walls, forests) and entities, interleaved by depth, as
    `Sorted` does today;
  - `Overlays` — routes, selection outlines, vision fans and shadows; in an isometric view they
    may take the depth of what they lie on and so hide behind what stands in front;
  - `Marks` — what must always show: the box being dragged, labels.
  From above, depth within a layer is the world y, or nothing at all; the key still orders layers.
- **Items.** Two kinds cover everything drawn now:
  - a textured quad from an atlas sheet — what `Sink.Quad` and `Sink.Shaded` already take;
  - a coloured triangle mesh — a vector path turned into vertices and indices
    (`vector.Path.AppendVerticesAndIndicesForFilling` / `ForStroke`), for fans, shadows, outlines,
    lines and routes drawn as lines.
  The composer batches consecutive items of one sheet (quads) or one colour mode (meshes) into one
  `DrawTriangles`, as `Sorted` batches quads now.
- **Scenes.** A Scene would list one composer for its world instead of several world layers, and
  the plugins' renderers would become the composer's sources. Screen layers stay as they are.
- **Split screen and minimap.** Unchanged in spirit: the composer runs once per viewport, with that
  viewport's camera, over the same sources.

## 3. What it buys and what it costs

Buys:
- in an isometric view, overlays that respect depth: a route or a cone behind a mountain is hidden
  by it, a selection outline is not drawn through a wall;
- one sort and as few draw calls per viewport as the sheets and meshes allow, where today every
  overlay renderer issues its own vector draws;
- one place that knows the order of what is drawn, instead of the order of a list in each demo.

Costs:
- rewriting the renderers of vision, selection, navigation and the board's grid lines to emit
  meshes instead of drawing paths;
- a frame list of items with their vertices, kept between frames so it does not allocate, but
  larger than today's `Sink` of quads;
- anti-aliasing: `vector` draws paths anti-aliased; triangles drawn with `DrawTriangles` need its
  `AntiAlias` option or a wider stroke to look the same;
- a Scene loses the freedom to put a world layer between two others by listing it there; it
  chooses a layer constant instead.

## 4. Questions before code

1. Are four fixed layers enough, or does a game need its own between them (numbers with room, like
   100, 200, …)?
2. Should an overlay in an isometric view take the depth of the ground it lies on (hidden behind
   a mountain) or always draw on top of the objects (always readable)? Per item, or per layer?
3. Does the composer replace `render.Sorted` and the plain world layers at once, or live beside
   them while the plugins move over one by one?
4. Is a coloured mesh enough, or do the vision shadows want a shader (soft edges, a gradient)?
