# Roadmap

[← Back to README](../README.md)

Where the engine stands and what comes next, in the order it is meant to come. The reasoning
behind each item lives in [movement.md](movement.md), [views.md](views.md) and
[rendering.md](rendering.md).

## Done

- Movement through a motion profile: `world.Steering` with `V0`, `Accel`, `Brake` and
  `TurnRate`; navigation asks for headings at a lookahead point, passes waypoints by projection,
  brakes to rest on the goal, queues goals (Shift + right click), holds a wide turner's leg until
  it reaches the waypoint, and turns a unit to look (`LookAt`, `MoveOrder.Face`) —
  [movement §1–§4, §7](movement.md).
- Terrain in the ECS: every cell is an entity for good (`board.Plot` with its `Relief`, `Ground`
  with its kind), made at Setup or found after a load; the board reads and writes them and counts
  every change in its `Version`, effects included. Domains say who may stand where (`Allows`,
  `Solid`, `Mover`), the board reports where everyone stands (`Standing`, `Fell`), costs are
  priced per domain, routes are checked whenever the terrain changes — [movement §5, §6, §11](movement.md).
- Terrain as fields: the board is the world's solid ground and cover, read from the cells by
  collision (aabbworld v1.9.0's `collide.Config.Field`, pushed out only through open sides) and by
  sight (`Cone.Cover`, the cells a ray crosses); `Solid` and `Veil` independent, no terrain
  entities, a change counts from the next tick, and the cost no longer depends on how the terrain
  is laid out — [movement §6, §12](movement.md).
- The ground shaped as in Transport Tycoon: heights from `Layout.Heights`, `Raise`, `Lower` and
  `Level` within `Shaping.MaxStep`, the ground round about following.
- Sight through terrain: a `Veil` per kind, aabbworld's raycast spending its radius as a budget;
  planes — `world.Layers` read by collision and by sight through `Sight.Blockers` — [movement §12](movement.md).
  Heights: `world.Config{Quasi3D: true}`, `world.Z`, the board as the world's `Ground`, sight from
  `Sight.Eye` over walls, forests and hills, and the ground out of sight kept as shadows, holes in
  the drawn view (aabbworld v1.8.0) — [movement §14](movement.md).
- An isometric view: `camera.Projection`, `render.Sorted`, sloped and shaded tiles, billboards,
  picking on the ground and where entities are drawn, `island-isometric-demo`.
- Tags as bits of families, one component per family; `Between(a, b, fn)` by value; behaviors
  built by the hosting plugin (`vision.Between`, `board.Each`, `world.Every`), `plugin/host` for
  plugin authors.
- Effects: `Grant` and `Alter` in a `Spec`, cast by entity id, saved with the entity;
  `Active.Altered` and `Idle` tell the owner of an altered component — [movement §13](movement.md).
- Commands: a `plugin.CommandHandler` keeps a `control.Queue` of each command it defines and
  drains it in its own pass; players carries them there from bindings, an AI or a network.
  Triggers include `KeyHeld`, once a tick while a key is down.
- Players: local players, each able to look through a camera of its own, saved with the game;
  `players.Viewports` splits the screen (`Columns`, `WithLayout`); keys reach every local
  player, the mouse the one under it; the input layer is players' `eventHandler`. The selection
  box is selection's (`Marquee`), per camera. Follow a unit with the camera, `CenterOn` a point —
  [views §2](views.md).
- Scenes and viewports: a Scene lists screen layers and world layers; world layers are drawn once
  per `render.Viewport` of a `game.Viewer` scene, through the camera handed at draw time — a
  player's view, split-screen halves, a minimap. The cameras live in `internal/camera` and come
  from the world. A resizable window and fullscreen (Shift+F).
- Twelve demos, `split-screen-demo` (two players, WSAD and arrows, a minimap) the latest.

## Next

1. **One composer per view** — decide the questions in [rendering.md](rendering.md) (layers,
   depth of overlays, how to migrate), then build it: overlays hidden behind what stands in front
   in an isometric view, one sort and few draw calls per viewport.
2. **Hover** — what is under the cursor, a `Space.Query` at a point in players' event handler —
   [views §2](views.md).
3. **Canals and building on shaped ground** — turn a cell lowered to the sea into water, a
   preview of a shaping drag (lost with the players' marquee), the costs of shaping.
4. **`RouteStyle`** — `CellArrows` by default, `SmoothRoute` opt-in, arcs from the profile,
   computed when the route changes — [movement §8](movement.md).
5. **Effects over the whole board** — weather and seasons as effects on an entity standing for
   the board — [movement §13](movement.md).
6. **Gamepads** — a trigger vocabulary for pads, so split screen is not only a keyboard's.
7. **Networking** — `netview` over players: deltas from the `View`, one mask per client, a frame
   a tick, a client without an ECS — [views §3](views.md). Commands that carry a camera today
   (`Select`, `Follow`, `Marquee`) will carry the player instead.
8. **Turn-based movement** and **arbitration** — when a game needs them — [movement §9, §10](movement.md).

Also on the list: a thumbnail in a save (the frame at the moment of saving, for a load screen);
saves written before tag families and before the cell entities do not load, to be noted at the
next tag; that tag, v0.3.0, once this state has been reviewed.
