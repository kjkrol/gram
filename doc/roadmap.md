# Roadmap

[← Back to README](../README.md)

What is left to do. Take an item out when it lands.

## Engine
- **A dictionary of idioms** — `Once` (a thing done at most once a while, an effect the memory),
  `Either` (a coin), the patrol (`Onward`, `Until[Arrived]`, `Wait`), the asker's "no answer"
  (`Until` under a `Timeout` in a `OneOf`): named shapes written out of the bricks, in
  `doc/behaviour.md` or as functions a game copies, never steps of the engine.
- **Effects, what is left** — an effect taking a tag off while it runs (a `Revoke` beside
  `Grant`); `Around` in plans; a step shared by the moments of several plugins; more than 8
  effects on one entity and 63 defined; an entity in more than one group; which effects a player
  may apply (any, today, on whatever a command names — a game over a network will want a list).
- **Messages passed on** — a reply to a message passed on goes to the one who passed it on, not to
  the first asker; passing the answer back is the game's branch (ceremony's `plan/message_test.go`,
  the giver). A loop (A asks B, B asks A) ends by `MaxChain` and by a branch of a fact firing once
  per coming; whether `Hops` should count across types is open.
- **A minimap plugin** — a `ui` element: a feed from a camera keeping the whole world in view
  (`camera.Config.Whole`), the game's picture under dots of its own by kind, picked by drawing
  rules (what my units see: `vision.Seen`), the players' views outlined on it, a key to show and
  hide it, a click on it panning the player's view; a round or many-sided one through `ui.Masked`.
  The click needs a picture a player only clicks through: the point under the cursor worked out
  through the minimap's camera, the camera commands still going to the player's view — today a
  player acts through one picture a scene, its wire (`players.Plugin.Through`) both.
- **ui, what is left** — `Dialog`, `Toast`, `MenuBar`/`Menu`/`ContextMenu`, `Tabs`, `Scroll`,
  `List`; `Canvas` (a tech tree), `Tooltip`, drag and drop, focus moved by keys and pads; the
  UI drawn into one `render.Frame` in place of a draw an element, once a profile asks for it.
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
- **Baked shadows over hex prisms** — the terrain over a hex grid bakes no shade, so what casts
  a Baked shadow (a bridge's deck and its piers) casts none there, and what stands there receives
  only the clouds'.
- **Figures from above in the shade** — seen from above the figures are the world's flat
  sprites (`render.Sprites`), which receive no shade of the relief, the blocks or the clouds.
- **Shadows by distance** — `topography.ShadowPolicy` is the seam; no game sets one yet.
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

## Landscape and Madeira

- **The surf follows the cells** — the line of breaking waves (`water.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
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

- **`world.Leaving`** — a moment no game uses: the world tells its rules of an entity past an open
  edge and despawns it with none. Should it stay a moment, or should the world simply despawn
  whoever leaves?
- **The frozen light** freezes the hour alone, and is not saved: should it freeze the date too,
  and come back after a load (a component on the clock's entity)?
- **Big groups arriving** still strike each other where the column fans out to its spots.

## Housekeeping

- **`BENCHMARKS.md`** — the old island's numbers predate the roads, the folded quads and the outline
  fix: measure again on an idle machine, alternately against a baseline.
- **A thumbnail in a save** — the frame at the moment of saving, for a load screen.
- **The next tag** — after goke v3.4.1 and ceremony v0.2.1 are tagged and `go.mod` bumped; note
  that saves written before the cell entities and what stands on the board do not load.
