# Roadmap

[← Back to README](../README.md)

Where the engine stands and what comes next, in the order it is meant to come. The reasoning
behind each item lives in [movement.md](movement.md) and [views.md](views.md).

## Done

- Movement through a motion profile: `world.Steering` with `V0`, `Accel`, `Brake` and
  `TurnRate`; navigation asks for headings at a lookahead point, passes waypoints by projection,
  brakes to rest on the goal, queues goals (Shift + right click) — [movement §1–§4, §7](movement.md).
- Terrain as the one truth of the board: solid cells are bodies built from the boxes of any grid
  (square and hex), domains say who may stand where (`Allows`, `Solid`, `Mover`), the board
  reports where everyone stands (`Standing`, `Fell`), costs are priced per domain, and routes are
  checked whenever the terrain changes — [movement §5, §6, §11](movement.md).
- Sight through terrain: a `Veil` per kind on a body carrying `vision.Transparency`, aabbworld's
  raycast spending its radius as a budget; planes — `world.Layers` read by collision and by sight
  through `Sight.Blockers`, so a hawk on `Air` looks over walls, forests and walkers and still
  sees them — [movement §12](movement.md). Heights: `world.Config{Quasi3D: true}`, `world.Z`,
  the board's cell entities as the world's `Ground`, sight from `Sight.Eye` over walls, forests
  and hills; collision stays on planes — [movement §14](movement.md). An isometric view:
  `camera.Projection`, `render.Sorted`, relief with sloping ground, billboards, `island-isometric-demo`.
- Tags as bits of families, one component per family; `Between(a, b, fn)` by value; behaviors
  built by the hosting plugin (`vision.Between`, `board.Each`, `world.Every`), `plugin/host` for
  plugin authors;
  `Selectable`/`Selected`, the vision behaviors' tags and terrain bodies on bits.
- Effects: `Grant` and `Alter` in a `Spec`, cast by entity id, saved with the entity; every cell
  an entity with `Ground` and `Plot`, so an effect can change terrain for a while —
  [movement §13](movement.md). The ground shaped as in Transport Tycoon: `Raise`, `Lower`, `Level`.
- Hex boards on screen, route arrows every 15°, camera panning in screen pixels, a `QuadBatch`
  that draws in chunks; six demos, `effect-demo` among them.
- Players: `plugins/players` with one local player over the world's camera, built over the
  `plugin.Commander`s; typed commands (`Select`, `MoveTo`, `Pan`, `Zoom`) owned and drained by the
  plugins that define them; labelled bindings with defaults shipped by the plugins — [views §2](views.md).

## Next

1. **Split screen** — a camera and a screen rectangle per local player, renderers per rectangle —
   [views §2](views.md).
2. **Hover** — what is under the cursor, a `Space.Query` at a point in the translator — [views §2](views.md).
3. **`RouteStyle`** — `CellArrows` by default, `SmoothRoute` opt-in, arcs from the profile,
   computed when the route changes — [movement §8](movement.md).
4. **Effects over the whole board** — weather and seasons as effects on an entity standing for
   the board — [movement §13](movement.md).
5. **Networking** — `netview` over players: deltas from the `View`, one mask per client, a frame
   a tick, a client without an ECS — [views §3](views.md).
6. **Turn-based movement** and **arbitration** — when a game needs them — [movement §9, §10](movement.md).

Also on the list: saves written before tag families do not load, to be noted at the next tag; that
tag, v0.3.0, once this state has been reviewed.
