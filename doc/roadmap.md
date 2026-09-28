# Roadmap

[← Back to README](../README.md)

What is left to do, in no particular order yet. Take an item out when it lands.

## Engine

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
- **Collision with heights** — two entities meet where their `world.Layers` share a bit; a veto by
  `Z` overlap would let collision follow height (a hawk landing, a projectile clearing a wall) — a
  real change to the solver, when a game needs it.
- **Live hydrology** — the water worked out as the game goes: rivers swelling after rain, drying
  in summer, courses changing with the weather and the season.

## Landscape and the islands

- **The surf follows the cells** — the line of breaking waves (`topography.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
- **Two mouths side by side** — two rivers reaching the sea next to each other look like a "U" at
  the water: the drainage joins them by the shore.
- **Forests come back** — with a plugin for plants; the `forest` kind stays for it.

## Housekeeping

- **`BENCHMARKS.md`** — the island's numbers predate the roads, the folded quads and the outline
  fix: measure again on an idle machine, alternately against a baseline.
- **A thumbnail in a save** — the frame at the moment of saving, for a load screen.
- **The next tag, v0.3.0** — once this state has been reviewed; note that saves written before
  tag families, the cell entities and the crossings (`board.Crossing`) do not load.
