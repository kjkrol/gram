# Roadmap

[← Back to README](../README.md)

What is left to do. Take an item out when it lands.

## Engine
- **A dictionary of idioms** — `Once` (a thing done at most once a while, an effect the memory),
  `Either` (a coin), the patrol (`Onward`, `Until[Arrived]`, `Wait`), the asker's "no answer"
  (`Until` under a `Timeout` in a `OneOf`): named shapes written out of the bricks, in
  `doc/behaviour.md` or as functions a game copies, never steps of the engine.
- **Effects, what is left** — an effect taking a tag off while it runs (a `Revoke` beside
  `Grant`); places (`cond.At`) in plans; a step shared by the moments of several plugins; more than
  8 effects on one entity and 63 defined; an entity in more than one group; which effects a player
  may apply (any, today, on whatever a command names — a game over a network will want a list).
- **Messages passed on** — a reply to a message passed on goes to the one who passed it on, not to
  the first asker; passing the answer back is the game's branch (ceremony's `plan/message_test.go`,
  the giver). A loop (A asks B, B asks A) ends by `MaxChain` and by a branch of a fact firing once
  per coming; whether `Hops` should count across types is open.
- **The minimap, further** — marks picked by what a player's units see (`vision.Seen`), the
  other players' views outlined; beyond the bands a picture kept of what was last seen in place
  of the live world.
- **ui, what is left** — `Dialog`, `Toast`, `MenuBar`/`Menu`/`ContextMenu`, `Tabs`, `List`,
  `Radio`; `Canvas` (a tech tree), `Tooltip`, drag and drop, focus moved by keys and pads
  over the controls too; the UI drawn into one `render.Frame` in place of a draw an element, once
  a profile asks for it. Settings of a player's own (its camera's resolution and sight) on the
  settings screen, beside the plugins'.
- **Canals and building on shaped ground** — a cell lowered to the sea turns to water; a preview of
  a shaping drag (lost with the players' marquee); the costs of shaping.
- **A smooth route** — an opt-in `RouteStyle` drawing the line the unit will follow: arcs of
  radius `Speed / TurnRate` through the lookahead points, the queued waypoints marked; computed
  when the route or the waypoints change and kept beside the `Path`, sharing the lookahead's code
  with navigation so what is drawn and what is driven cannot drift.
- **Covers** — a state of the ground drawn over its cells (`board.Plugin.Covering`) is the simple
  map's on a square grid: in relief (the topography's painter and its sheets), on a hex grid
  along a line of its own, a ragged edge (noise on the threshold), a cover narrowed to the cells
  that can take it (a lake's shore), one that moves (a storm, a whirlpool). What stands on the
  ground and moves — fire, smoke — is sprites over it, not a cover.
- **Shots and decks** — a shot's ground (`bullet.Plugin.WithGround`) is the heights alone: it flies
  through a deck and falls by its own `Gravity`, which gravity turned over does not touch; both
  through `ground.Surfaces` and the world's `gravity.Pull`.
- **Leaps, climbing and swimming planned** — the planner never plans a leap (a railing, a gap),
  a climb or a swim; a unit under gravity does them only by hand.
- **Bridges that turn** — a bridge runs straight; a knee (two links not opposite) is refused by
  the board's rules and not made by `cmd/gramap`.
- **Roads with a tool** — a player lays a road as a bridge (`control.Tool`, a road's rules beside
  `bridges/site`'s), and takes a bridge down.
- **Baked shadows over hex prisms** — the light bakes no shade over a hex grid's relief, so
  what casts a Baked shadow (a bridge's deck and its piers) casts none there, and what stands
  there receives only the clouds'.
- **The lamps' shadows in relief** — a lamp casts shadows on level ground alone; over a relief
  its cast would have to follow the ground as the sun's bake does.
- **Clouds shading themselves** — the cover's own shade is the one look of a cloud's thickness;
  a cloud lit by the sun from above and dark below is not drawn.
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
  meets, and the push out, through the shallowest open side, may put it on the far side. No demo has
  a solid cell on a slope; a wall's band from the lowest corner, or a sealed side, when one does.
- **A sweep across the seam** — a swept entity (`collision.Sweep`) in a wrapping world: its
  stretch in the space as the fragments the seam cuts it into, the segment refined per image.
  Collision refuses one today, with a panic at the first it meets.
- **Shots that go on** — a shot through its target (`Body.Pierces`: the nearest contact a Landing
  that does not end the flight), a trail drawn behind it, a mine that feels a tread (a landed
  shot touching), a wounded unit slowed by its own Z in `collision.Field.Overhang`.
- **Live hydrology** — the water worked out as the game goes: rivers swelling after rain, drying
  in summer, courses changing with the weather and the season.
- **Sight's cost** — the scan of `vision` reads the ground along every ray exactly
  (`ground.Profile`, the topography's `relief.Along`: about two points a crossed cell, each handed
  to aabbworld's raycast by a callback). `Benchmark_Vision_Relief` (2026-10-24): the profile 1.60 ms
  a tick, sampled a cell apart 1.31 ms, every 50 m 6.15 ms; of the scan the relief 53%, aabbworld's
  raycast 24%, the board's cover 14%. Every entity with a `Sight` and an `Eye` is scanned, though
  the views on screen are worked out on the GPU apart: in board/topography nothing reads the result.
  Ways, for the coming AI plugin that filters what its entities see: scanning only whom something
  reads (an AI's entities, a `Sighted` asked for, `SightOutline`, `Sighting` rules) — done
  2026-10-26 (`vision.Watched`, `WithScanAll`); skipping ground that cannot raise a ray's horizon (a
  max-height mip of the relief, the heightfield's walk told the angle to beat — aabbworld, since the
  scan walks `Cone.Heights` itself, 2026-10-26); fewer rays or sampling far off (with the LOD); the
  AVX2 kernels of the scan (slabs, the cone query, four rays over the field) on every hyperthread
  lower the clock for the scalar callbacks a sampled ground is made of (2–6% on
  `Vision_Relief/step=*` in parallel, faster on one CPU): 128-bit lanes would stay under the licence
  (aabbworld). Measured: `Along` clamping with comparisons and folding a cell's corners once, −3–4%
  of the scan, the same points.

## Landscape and Madeira

- **The surf follows the cells** — the line of breaking waves (`water.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
- **Mipmaps for the ground's details** — `gpu.Texture` holds one level, so a kind's
  `painter.Detail` fades out below 40 px a cell instead of growing coarse, and sampled minified
  it costs ~0.4 ms a frame on an Intel UHD 620; levels baked once (a blit a level) and
  `textureSampleGrad` by the derivatives before `fract` would let it show at every distance and
  under a low camera.
- **Two mouths side by side** — two rivers reaching the sea next to each other look like a "U" at
  the water: the drainage joins them by the shore.
- **Forests come back** — with a plugin for plants; the `forest` kind stays for it. The trees are
  to stand on the board as entities, as bridges do, so a unit walks among them in first person;
  that needs the world parted into sectors, sparing the GPU, the memory and the CPU.
- **Winter without the slowdown** — snow and ice on Madeira (`weathering`) slow the game down
  badly; a lead: `weathering` asks `air.FallsAt` of every cell once a second of game time, and
  since the clouds came in two layers it works out both layers' noise on the CPU.
- **More knobs of the sky** — the sun and the weather as knobs on the atmosphere's entity, as
  the moon is (`sky.Moon`), so an eclipse or a spell of fog is an effect.
- **Evaporation and fog** — the air masses carry their weather's damp and the rules of the
  board's cells gather it into clouds, but none rises from water (evaporation) and no fog lies
  low at night and at dawn.
- **The weather over a unit, for rules** — the board's cells hear the air over them
  (`weathering.Overhead`); a rule about the rain over a unit (taking shelter) needs it too.

## Open questions

- **`motion.Leaving`** — a moment no game uses: motion tells its rules of an entity past an open
  edge and despawns it with none. Should it stay a moment, or should motion simply despawn
  whoever leaves?
- **Big groups arriving** still strike each other where the column fans out to its spots.

## Housekeeping

- **`BENCHMARKS.md`** — the old island's numbers predate the roads, the folded quads and the outline
  fix: measure again on an idle machine, alternately against a baseline.
- **A thumbnail in a save** — the frame at the moment of saving, for a load screen.
- **The next tag** — after goke v3.4.1 and ceremony v0.2.1 are tagged and `go.mod` bumped; note
  that saves written before the cell entities and what stands on the board do not load.
