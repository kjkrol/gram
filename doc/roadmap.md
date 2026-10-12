# Roadmap

[← Back to README](../README.md)

What is left to do. Take an item out when it lands.

## Next, in this order (the demos' TODOs)

1. **The notation of rules** — `rule.Then[world.Moving]("rage spreads", rule.All,
   rule.During(rage, rule.Keep(angry)))` reads badly, and a rule needing a comment to be read is
   a notation to change: variants to compare before deciding.

## Engine
- **A minimap plugin** — a `ui` element: a feed from a camera keeping the whole world in view
  (`camera.Config.Whole`), the game's picture under dots of its own by kind, picked by drawing
  rules (what my units see: `vision.Seen`), the players' views outlined on it, a key to show and
  hide it, a click on it panning the player's view; a round or many-sided one through `ui.Masked`.
  The click needs a picture a player only clicks through: the point under the cursor worked out
  through the minimap's camera, the camera commands still going to the player's view — today a
  player acts through one picture a scene, its wire (`players.Plugin.Through`) both.
- **ui, what is left** — `Dialog`, `Toast`, `MenuBar`/`Menu`/`ContextMenu`, `Tabs`, `Scroll`,
  `List`; `Canvas` (a tech tree), `Tooltip`, drag and drop, focus moved by keys and pads; the
  scenes-demo's menu and the players' list of shortcuts as ui (their keys need `game.Runtime`:
  switching the Stage, quitting); the UI drawn into one `render.Frame` in place of a draw an
  element, once a profile asks for it.
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
- **A demo of ways** — roads as `cell.Way` and a bridge standing on the board over the ground, a
  unit taking the road round the mud, in the dress of the effect demo.
- **Shots and decks** — a shot's ground (`bullet.Plugin.WithGround`) is the heights alone: it flies
  through a deck and falls by its own `Gravity`, which gravity turned over does not touch; both
  through `ground.Surfaces` and the world's `gravity.Pull`.
- **Leaps, climbing and swimming planned** — the planner never plans a leap (a railing, a gap),
  a climb or a swim; a unit under gravity does them only by hand.
- **Bridges that turn** — a bridge runs straight; a knee (two links not opposite) is refused by
  the board's rules and not made by `cmd/gramap`.
- **Roads with a tool** — a player lays a road as a bridge (`control.Tool`, a road's rules beside
  `bridges/site`'s), and takes a bridge down.
- **Blocks' shadows over hex prisms** — the terrain over a hex grid bakes no shade, so a
  bridge's deck and its piers cast no shadow there.
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

## Landscape and Madeira

- **The surf follows the cells** — the line of breaking waves (`water.Shore`) runs along the
  cells' edges, not the rounded coast.
- **Roads in the isometric view** — dark and thin, covered by the routes: their colour and width.
- **Two mouths side by side** — two rivers reaching the sea next to each other look like a "U" at
  the water: the drainage joins them by the shore.
- **Forests come back** — with a plugin for plants; the `forest` kind stays for it.

- **More knobs of the sky** — the sun and the weather as knobs on the atmosphere's entity, as
  the moon is (`sky.Moon`), so an eclipse or a spell of fog is an effect.

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
- **The next tag, v0.3.0** — once this state has been reviewed; note that saves written before
  tag families, the cell entities and what stands on the board (`cell.Built`) do not load.
