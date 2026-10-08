# Roadmap

[← Back to README](../README.md)

What is left to do, in no particular order yet. Take an item out when it lands.

## Pictures, cameras and the Stage's order

Agreed on 2026-10-08 (the user's word), reading `examples/split-screen-demo`: a camera's kind and
window are how a picture is shown, so they are chosen where the scene is built, not with the
players; a player is wired to the pictures it acts through, and owns no camera.

- **The chain** — `Plugins`, `Players`, `Effects`, `Rules`, `Commands`, `Kinds`, `Controls`,
  `Restore`, `Spawn`, `Scenes`, `Shows`, `Update`: the scenes come last, as the engine already
  builds their layers after Restore or Spawn.
  - `Cells` goes into `Kinds(steps...)`, run in order: `Kinds(s.defineCells, s.defineKinds)`,
    `cell.Kinds.Define` checking `section.Kinds` — a kind of cell is a kind, and the board's word
    leaves the core as `Layout` does.
  - `Layout` and `Units` become one link, `Spawn(steps...)`, run in order, and one part,
    `section.Spawn`, which `board.Seed`, `world.Seed` and `topography.Seed` check: `Layout` was
    the board's type named in the core, and both seeds only declare (`Populate` does the work),
    so the split guarded nothing. A demo names its steps by what they spawn, as `defineKinds`
    says what it defines: `Spawn(s.spawnCells, s.spawnUnits)`, board-topography's `spawnGround`.
  - `Scenes` after `Spawn`, with no new method of `game.Stage` and no change to the engine: what
    ends `Init` today (the game's scenes, the plugins', the stack, the first shown or `Shows`, the
    Composition tracked) is a private step of `game/stage`, run once after the world — at the end
    of `Restore` when it loaded a save, else at the end of `Spawn`. The first scene shown is the
    first `defineScenes` gives, or those `.Shows(...)` names; a loaded game shows those shown when
    it was saved. The scenes are made before `Populate`: nothing in them reads the entities spawned.
  - A value tracked after a Load gets its saved state as it is tracked (the engine keeps the
    loaded groups by name), so the Composition and the ui scenes, tracked now after the Load, come
    back from it.
  - `ui.NewScene(name, screen *Element)`: no `pictures`/`screen` callbacks — the scene is made when
    everything exists, and initialises once each picture its feeds show (a Feed knows its
    picture), so no picture is handed from one function to another through the arena. A scene's
    name is a constant (`MainScene`), as every other name.
- **`render.WorldRenderer` → `render.Picture`** — what is in the world, before a camera;
  `render.Image` is taken: the GPU texture a Feed draws the picture into. 31 files.
- **Cameras made in Scenes** — `render.NewFeed(s.cameras.New(maker, cfg), picture)`: the camera's
  kind (`cameras.TopDown()`, `topography.Views(start)`) and config beside the picture it shows.
  - `camera.Config` loses `ViewportWidth/Height`: the pixels are the layout's (`Feed.Resize` every
    frame; split-screen's half configured as 640 is laid as 639). It says the camera's window
    instead: a start scale (zoom), or `Whole` — the whole world, zoom `min(w/W, h/H)` worked out
    at every resize, the world centred, the background in bars along the longer axis. Neither the
    cover floor (`minZoom`, `max(w/W, h/H)`) nor `fitAxis` (which sticks a world smaller than the
    window to its top left) may hold for `Whole`. One zoom for both axes keeps a cell square
    whatever the window's proportions.
  - A start fastening, `camera.Config{Follow: entity.Named(RedBlock)}` (the unit named by
    `kind.Entry.Named`), which the cameras' system takes up at the first tick; a loaded camera
    keeps its own state. Following is the camera's mode, not a player's: a minimap following the
    hero is nobody's. Out go `Told(cameras.Follow{…})` at spawn, the minimap's camera made in
    `usePlugins` and its `ZoomOut(1e6)` + `CenterOn` in `screen()`.
  - Cameras are made after a Load: the cameras plugin keeps the states loaded and gives each to
    the camera made in its place (the order made, as saved).
  - Every camera saves its fastening (`camera.Fastening{Entity, How}`) beside its window and zoom:
    none does today, so a camera following a unit stands still after a load. One riding `Inside`
    comes back inside: `perspCamera.Restore` stops letting go, the topography's camera system
    fastens it again from the fastening loaded.
  - Every viewport is the layout's, so the plugin's sizing of unsized cameras (`unsized`,
    `plugin.Screen`) goes.
- **Players wired to pictures** — `players.Local(name)`, no camera. The wire is
  `ui.Image(feed).Input(s.players.Through(pl))`: the socket ui's (`ui.Input`, `Looker`, `Owner`),
  the plug players' (`through`, `plugins/players/ui.go`). ui still knows no camera:
  - `ui.Input.Over(area, shown render.Surface)` hands the plug what its element shows; players asks
    a `*render.Feed` for its camera (`Feed.Camera()`) and keeps, for the player, the pairs {area,
    camera} its pictures were laid at this frame.
  - The event handler — players' alone; selection, navigation and the rest only give bindings,
    which read a ready `control.Context` — builds the context from the pair under the mouse, and
    for keys from the player's view in the active scene (`covers`, `localPoint`, `screenOf` read
    the pair). `Select`, `MoveTo`, `Pan` and every binding stay as they are.
  - Only the active scene's pictures wire a player's view (a city scene's view while it is
    active; a scene shown under a modal does not): today two shown scenes with `Through(pl)`
    overwrite `pl.area`, the last drawn winning.
  - `through.LookAt` (`ui.OffScreen(GoToIt)`) moves the camera of its own picture.
  - `Player.Camera` goes, and `Player.View`, which nothing reads.
  - A local player no picture shows has no camera — a menu Stage has neither players nor
    cameras; whatever reads the context's camera bears its absence (`screenOf` calls
    `Viewport()` on it today).
  - A picture is to a player its view (`Through`: the keys, `Pan`, `Zoom`, `Follow`, `Ride` and
    the list under K are about its camera), one it clicks through (a minimap: the point under the
    cursor through the minimap's camera, the camera commands still the view's — see "A minimap
    plugin"), or one it only watches (no Input: split-screen's minimap). The second is not built
    yet; the wire leaves room for it.
- **ui: sizes relative to the parent, proportions locked** — a feed in `Whole` reports the world's
  W:H as what its element needs, so a scene gives only its share of the screen and the element
  keeps the world's proportions as the window changes. Split-screen's minimap: a share of the
  screen, `Whole`, fastened to nothing (fastened, it could show one player alone).
- **Every demo** — split-screen's shape: `definePlayers` without cameras; `defineScenes` building
  the picture, the cameras, the feeds and the layout in one place.

## Engine

- **A Stage's plugins without `ctx`** — `stage.New(name).World(cfg).Plugins(func(*world.Plugin)
  []plugin.Plugin)`: the stage calls `UseWorld` and `Use` itself, in the list's order; the game
  constructs its plugins and nothing else. Every demo's `usePlugins` is that already, the
  `Initializer` reached only for the two calls.
- **A minimap plugin** — a `ui` element: a feed from a camera from above fitted to a window of the
  world, the game's picture under dots of its own by kind, picked by drawing rules (what my units
  see: `vision.Seen`), the players' views outlined on it, a key to show and hide it, a click on it
  panning the player's camera; a round or many-sided one through `ui.Masked`.
- **ui, what is left** — `Dialog`, `Toast`, `MenuBar`/`Menu`/`ContextMenu`, `Tabs`, `Scroll`,
  `List`; `Canvas` (a tech tree), `Tooltip`, drag and drop, focus moved by keys and pads; the
  scenes-demo's menu and the players' list of shortcuts as ui (their keys need `game.Runtime`:
  switching the Stage, quitting); a label's text read off its pinned entity; non-rule commands
  about `ui.It`; the UI drawn into one `render.Frame` in place of a draw an element, once a
  profile asks for it.
- **Hover** — what is under the cursor: a `Space.Query` at a point, the players' translator's
  work, no collision involved.
- **Canals and building on shaped ground** — a cell lowered to the sea turns to water; a preview of
  a shaping drag (lost with the players' marquee); the costs of shaping.
- **`RouteStyle`** — how a route is drawn becomes a style, as `vision.ConeStyle` is: `CellArrows`,
  today's arrow per cell, by default; `SmoothRoute` opt-in, the line the unit will follow — arcs
  of radius `Speed / TurnRate` through the lookahead points, the queued waypoints marked. Drawn for
  selected units only, computed when the route or the waypoints change and kept beside the `Path`,
  sharing the lookahead's code with navigation so what is drawn and what is driven cannot drift.
- **Weather and seasons as effects over the whole board** — planned as effects on an entity
  standing for the board. `plugins/atmosphere` does weather and seasons another way (the weather
  on its own entity, snow and ice as the weathering's effects on cells, the seasons the
  calendar's over the clock): decide whether that settles it.
- **Covers** — a state of the ground drawn over its cells (`board.Plugin.Covering`) is the simple
  map's on a square grid: in relief (the topography's painter and its sheets), on a hex grid
  along a line of its own, a ragged edge (noise on the threshold), a cover narrowed to the cells
  that can take it (a lake's shore), one that moves (a storm, a whirlpool). What stands on the
  ground and moves — fire, smoke — is sprites over it, not a cover.
- **A demo of ways** — roads and a bridge as `cell.Way` and `cell.Crossing` over the ground, a unit
  taking the road round the mud, in the dress of the effect demo.
- **Knobs on the world** — a plugin's global knobs as components on the world's own entity (the
  moon's colour and strength, for a blood moon: an effect altering `sky.Moon`, cast by a rule of
  `clock.Moment` on a full moon — a predicate of the calendar's phase), and a vocabulary in every
  plugin's doc: its moments, its commands, its knobs.
- **Gamepads** — a trigger vocabulary for pads, so split screen is not only a keyboard's.
- **Networking** — `plugins/netview` over players; the server is one engine, a remote client a
  player whose translator decodes frames:
  - deltas from the player's `View`: entered, updated, left; a client that joins gets all as
    entered;
  - one walk for all clients: a `uint64` mask per entity for up to 64 clients;
  - a binary frame a tick per client: the tick, the camera state echoed, entered
    `[id, kind, sprite, box]`, updated `[id, box]`, left `[id]`;
  - a client without an ECS: a camera, an atlas, a `render.Composer` over the frames, commands
    going out; the transport behind an interface, tests through memory;
  - open: the server's tick against the client's frame rate (interpolation), joining mid-game (a
    snapshot), trust (a LAN to begin with);
  - commands that carry a camera today (`Select`, `Follow`, `Marquee`) carry the player instead.
- **Turn-based movement** — every unit at one tempo and a say in who moves when: a layer above
  navigation setting the tempo for a move and issuing `MoveTo` one unit at a time, waiting for
  each arrival; navigation need not know.
- **Arbitration** — the planner and a reaction (`Flee`) steering one unit in one tick: to start
  with, the reaction wins the tick and the planner re-plans; summed weighted requests only if that
  fails somewhere real.
- **A wall on a steep slope** — collision stops an entity in a solid cell whose band, from below up
  to its kind's `Height` over the cell's level, meets the entity's; on a slope steeper than that
  `Height` over half a cell a unit coming downhill may be partly inside the cell before its band
  meets, and the push out, through the shallowest open side, may put it on the far side. No demo
  has a solid cell on a slope; a wall's band from the lowest corner, or a sealed side, when one does.
- **A sweep across the seam** — a swept entity (`collision.Sweep`) in a wrapping world: its
  stretch in the space as the fragments the seam cuts it into, the segment refined per image.
  Collision refuses one today, with a panic at the first it meets.
- **Shots that go on** — a shot through its target (`Body.Pierces`: the nearest contact a Landing
  that does not end the flight), a trail drawn behind it, a mine that feels a tread (a landed
  shot touching), a wounded unit slowed by its own Z in `collision.Field.Overhang`.
- **A unit spawned in the game on the board** — `world.Spawn` of a unit with `At` and `Mover` does
  not enter it into the board's `cell.Occupancy` (the board seeds it at Setup alone).
- **The board keeps every unit's cell** — `unit.At` and `unit.Entered` follow a unit only where
  navigation or the driving moves it; one pushed by collision, or moved by a game's own system,
  keeps a stale cell. The board's units' pass knows the cell under each centre already.
- **Live hydrology** — the water worked out as the game goes: rivers swelling after rain, drying
  in summer, courses changing with the weather and the season.

## Landscape and the islands

- **The surf follows the cells** — the line of breaking waves (`water.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
- **Two mouths side by side** — two rivers reaching the sea next to each other look like a "U" at
  the water: the drainage joins them by the shore.
- **Forests come back** — with a plugin for plants; the `forest` kind stays for it.

- **More knobs of the sky** — the sun and the weather as knobs on the atmosphere's entity, as
  the moon is (`sky.Moon`), so an eclipse or a spell of fog is an effect.

## Housekeeping

- **`BENCHMARKS.md`** — the island's numbers predate the roads, the folded quads and the outline
  fix: measure again on an idle machine, alternately against a baseline.
- **A thumbnail in a save** — the frame at the moment of saving, for a load screen.
- **The next tag, v0.3.0** — once this state has been reviewed; note that saves written before
  tag families, the cell entities and the crossings (`cell.Crossing`) do not load.
