# Movement: along a path, through steering, past walls and holes

[← Back to README](../README.md) · [Views, players and commands](views.md)

> A design sketch, not a contract: how a unit should move along a planned route, what it is
> made of, and what happens when the world pushes it somewhere it may not go. Nothing here is
> code yet.

## Where this started

When this document was written, navigation wrote straight into `Base.Vel`, aimed at cell centres
and stopped at every one; `world.Steering` took no part. Everything from §1 to §8 below has since
been built as described, and §13 records what came on top of it; §9, §10 and §12 are what is
left. What was there at the start and stayed: a unit knocked off its `Leg` releases it, drops its
`Path` and re-plans; a unit whose next cell is taken waits (`targetWaitTimeout`) and then settles
for the `nearestFree` cell.

## 1. One actuator: Steering

The only thing that moves a unit — turns it and speeds it up — is `world.Steering`. Everything
else asks: `Steering.Request(dir)` for a heading, `Steering.RequestSpeed(v)` for a speed. Navigation
stops writing `Vel.Dir` and `Vel.Value`. The profile is active when `MaxSpeed` is above zero;
without one, `Steering` steers headings only and `Vel.Value` stays whoever's it was — which is
what every unit that carries `Steering` today gets.

This settles the levels of the movement commands from [views.md](views.md): `MoveTo{Cell}` is a
goal for the planner (navigation), `Steer{Dir}` is a reactive intention (flee, chase); both end in
a `Request`. An AI may speak at either level. Speed is the unit's own: navigation asks for its
`MaxSpeed` and brakes from its `Accel`, and `navigation.NewPlugin` takes no speed — unit types
differ in tempo through their kind. A unit with a `MoveOrder` but no `Steering` profile is not
navigated.

### The motion profile lives in Steering

Beside `TurnRate` and `Reflex`, `Steering` carries how the unit accelerates:

| Field | Meaning |
|:---|:---|
| `MaxSpeed` | the unit's top base speed, world units a second; zero means no profile. A speed past the step `StepReach` allows (`Position.MaxSpeed(tps)`) is clipped by the move, tick by tick — a kind does not know the tick rate, so nothing refuses it earlier |
| `Accel` | how fast it speeds up, units a second² |
| `Brake` | how fast it slows down; zero brakes at `Accel`. A brake is what an effect weakens: on ice a unit slips |
| `V0` | the speed it has the instant it sets off from standing: a walker walks at once, a tank starts from nothing |
| `Speed`, `WantSpeed` | state: the current base speed, and the one asked for |

Each tick the `SteeringSystem` turns `Vel.Dir` towards `Want` by at most `TurnRate`; from
standing it jumps to `min(V0, WantSpeed)`, then moves `Speed` towards `WantSpeed` by `Accel·dt`,
never past `MaxSpeed`; and it **writes `Vel.Value = Speed` afresh every tick**, before the
`VelocitySystem` runs the `Moving` behaviors that scale it (terrain, a frozen tag). That last point
repairs a fragility of old: the behaviors scale `Vel.Value` in place, so whoever owns the base speed
must rewrite it every tick or they compound — navigation happened to, other units happened not to.

Everything below follows from the profile. The turning radius is `Speed·dt / TurnRate`: wide arcs
at full speed, tight ones when setting off. Braking follows `v = sqrt(2·Brake·d)`: navigation asks
for that speed at distance `d` from the goal, never below the speed braking leaves at the arrival
radius, and the unit comes to rest on the goal — a weaker brake means braking earlier, not
overshooting; the route itself carries no "brake here": every decision is read off the profile
as it is that tick, so an effect on the profile acts at once. The profile is
the kind's (`comp.Const(world.Steering{MaxSpeed: 120, Accel: 200, V0: 40, TurnRate: 0.1})`), so
unit types differ in how they move without any code.

## 2. Where to look: the lookahead point

Instead of the centre of the current cell, the unit aims at a point on its path a distance
`reach = Speed·dt / TurnRate` ahead — the turning radius: the way covered in a tick over the radians
turned in a tick. Walk the path's segments from the
unit's projection onto them, lay off `d`, take the point (on a torus through `shortestAxisDelta`,
as today). As the point passes a bend in the path, the requested heading starts to rotate before
the unit reaches the cell, and the `SteeringSystem` carries it round the arc. With `TurnRate = 0`
the point is the next waypoint and the unit moves as it does today.

Speed scales with `max(cos θ, 0.2)` of the angle between the heading and the requested
direction: straight on keeps full speed, a sharp bend slows the unit, a U-turn crawls — so the
turning radius shrinks with the turn and a unit turns round almost on the spot. Slowing to a
halt happens only at the end: `WantSpeed = 0` from the braking distance, then the nudge onto the
goal's centre as today.

## 3. Passing a waypoint is not being near its centre

A waypoint is passed when the unit's projection onto the waypoint's segment goes beyond the
segment's length — it has crossed the plane perpendicular to the segment at the waypoint — or
when the unit is within the lookahead reach of it, since from there the lookahead already looks
past it. Without the second rule a route that folds back at the waypoint keeps the unit circling
in front of the plane forever. It never has to touch the centre. `arrivalEpsilon` remains for the
final goal only.

A new route that goes back the way the current leg came turns the leg round on the spot — the
same cells are held — so a unit ordered back does not first finish the step it was on.

## 4. Entering a cell is its own event

Entering a cell — for `Occupancy` and the `CellEntered` tag — is `CellAt(centre of the box)`
changing, as today, and has nothing to do with waypoints. The next cell is reserved (`Leg`) ahead
of time, when the lookahead point enters it; if `CanEnter` refuses, the unit asks for
`WantSpeed = 0` and waits (`targetWaitTimeout`, `nearestFree` as today) — the one legitimate halt
on a route. The other is a bump: a unit under orders that struck someone (collision's `Struck`,
which navigation listens for with a behavior of its own) stops for the tick, plans again from
where it stands and holds that route for `bumpInterval`, deaf to further bumps — without the
interval it would plan again every tick and never move. What the new route avoids is the
`Occupancy`'s to say: it is kept **per domain** — `SingleOccupancy` lets one entity per domain
into a cell, so two walkers head-on on a one-cell road find each other's cells taken, step aside
into the field and pass, while a hawk over them holds the Air layer and neither blocks nor is
blocked; `MultipleOccupancy` is tokens on a square, any number, and such tokens carry no
`Physics`, because bodies cannot overlap. A walker whose goal someone stands on gets no route,
waits `targetWaitTimeout` and settles beside. `Reflex` delays a reaction, it does not
stop anything: requests come every tick and `Steering` coalesces them.

## 5. Obstacles come from one source: the terrain — done

Terrain is the one truth about the board, and anyone may write it: `Board.Set` is permanent,
saved, versioned (a write that changes nothing does not count), and the bodies, the sprites and
the planner follow. A rock is a solid kind on its cells; a building that must also be an entity
writes its cells when it is built and restores them when it falls; an ice witch is an `Each`
over `Standing` that turns the cells under her `Box` (`Grid.CellsUnder`, exact on a square and on
a hex) into snow and the water into ice (undoing it in time is the coming effects plugin's job). `Allows` and
`Mover` alone decide who may plan where, and `CellKind.Costing` makes a kind cheaper for some
domains (`CostFor` is what the planner and the terrain's `Moving` behavior charge), so the witch is fast on her
own snow and elves feel no forest; the solver keeps units out of whatever is solid.

## 6. Pushed onto forbidden ground — done

What happens when a collision pushes a unit towards a cell it may not enter is the cell kind's
decision (`board.CellKind`). Walls, holes and water are done; sight through terrain is §12.

- **A wall — done.** Impassable cells are **terrain bodies**: entities with a `Base`, a
  `collision.Collider`, a `collision.Physics{Mass: +Inf}` and the `board.Body` tag, no
  `Appearance` (the board draws the cell), no kind (`world.Kinds.Reserve`, spawned through
  `world.Bodies`). A body is made of boxes: each grid says what boxes cover one of its cells
  (`Grid.CellBoxes`) — a square is one box, a hex is a middle band plus `HexCapStrips` strips over
  each cap, as wide as the hex is at the strip's wider edge, so the cover is never smaller than
  the cell; an irregular region (a province) will give the boxes of its raster. Touching boxes of
  one kind are merged into rectangles, at most `MaxBodyCells` cells along either axis, so a
  wall column is one entity and no body is ever large enough to confuse the wrapped images the
  space and the raycast work with. The collision solver then never leaves a unit inside a wall —
  `Static` means infinite mass — and the bodies are in the space, so vision sees them and they
  occlude. A game turns this on explicitly: `board.NewPlugin(...).WithCollision(c)`; the bodies
  are rebuilt whenever `TerrainMap.Version` moves, and once after a load, where the saved ones are
  replaced by what the terrain says. Their `Caps` are settled by collision on the next tick, so an
  edit to the terrain mid-game is solid one tick late.
- **A hole, water — done, as domains.** Who may stand where is a relation between the unit and
  the terrain, not a property of the cell: a `CellKind` says which `board.Domain`s it admits
  (`Land`, `Water`, `Air`, a game's own bits) and whether it is `Solid`; a unit's `board.Mover`
  says which it moves in. The planner keeps a unit to cells admitting its domain, so water and a
  hole are forbidden ground for a land unit and open water for a boat. Neither is a body — a body
  would occlude sight and push — so a collision can shove a land unit into either. Every tick,
  after collisions, the board reports `Standing{ID, Cell, Kind}` (the cell under the unit's
  **centre**) to `board.Each` behaviors — `board.Each[board.Mover]`, so the reaction holds the unit's
  domain — and `Standing.Fell(domain)` says the unit stands where its domain may not. It is a state, not an event, because `Each` runs for every entity the host
  walks — as `Struck` does in collision. The reaction is the game's: despawn, teleport, damage.
- **A forest.** `CellKind.Veil` makes a passable cell a body without a `Collider`: it dims sight
  and nothing else — how much, and for whom (`Veils`), is §12's business.
- **After a push, and after the ground changes.** The existing re-plan on being knocked off a
  `Leg` covers it, and covers being pushed onto a passable cell off the route as well; a push
  that keeps the unit on its leg — two units pushing each other along a road — is a bump, §4,
  which with occupancy per domain is what stopped island-demo's units from shoving each other
  for ever. Collision knows domains too: `world.Layers` are a unit's domain bits and a wall's
  the bits of whoever it keeps out, so a flyer passes over walls and walkers. When the
  terrain's version moves, every route is checked against `Admits` and dropped at the first step
  that no longer takes the unit; a `Leg` whose far cells stop admitting it while the unit is
  still on its near cell is let go and the unit asked to stop — whether it stops in time is its
  brake's business, which is where a slipping unit still ends up in the water. A unit standing
  where its domain may not — frozen in — keeps its order and waits; the order is given up as
  unreachable only from ground the unit may stand on.

## 7. Waypoints

A `MoveOrder` keeps a queue of up to `MaxWaypoints` goals behind its `Target`. A goal with more
behind it is passed by projection like a waypoint, the next becomes the `Target` and is aimed at in
the same tick, so the unit does not stop at the intermediate ones; it brakes only before the
last. Navigation's default bindings, in the sense of [views.md](views.md): right click →
`MoveTo{Cell}`, "Move here" (replaces the order and its queue); Shift + right click →
`MoveTo{Cell, Append: true}`, "Add a waypoint" (appends; an idle unit gets a fresh order) —
`navigation.Plugin.DefaultBindings()`, bound on a player. The route renderer draws the way to every queued goal, planning
each leg once and keeping it until the goals change.

## 8. Drawing the route is a choice of style

`PathRenderer` draws the route cell by cell today (`PathSprites`: one arrow per direction, a
`Dot`), and that stays the default, because it is cheap: one sprite per cell. How a route is
drawn becomes a style, as `vision.ConeStyle` and `selection.HighlightStyle` are: a
`navigation.RouteStyle` with two ready-made — `CellArrows`, today's, and `SmoothRoute`, the line
the unit will actually follow: arcs of radius `Speed / TurnRate` through the lookahead points,
the queued waypoints marked, the last segment to the goal's centre — plus a `RouteStyleFn` for
one's own, chosen at `WithRenderer`/`WithStyle`.

`SmoothRoute`'s cost is held down two ways: routes are drawn for selected units only, as today;
and the polyline is a function of the path and the motion profile, so it is computed when the path
is re-planned or the waypoint queue changes and kept beside the `Path` — a frame only draws it.
The lookahead point in navigation and the arc sampling in the style share one piece of code, so
what is drawn and what is driven cannot drift apart.

## 9. Turn-based movement — open, its own plan

A strictly turn-based game wants every unit to move at one tempo, not its own, and wants to say
who moves when. That is a layer above navigation: it sets the tempo for the duration of a move
(overriding the profile's speed while the move lasts) and issues `MoveTo` one unit at a time,
waiting for each arrival. Nothing in navigation needs to know; it is planned separately, after
the waypoint queue.

## 10. Arbitration — open

The planner and a reaction may want to steer the same unit in the same tick: a unit on its route
while `Flee` says "sideways". The rule to start with: the reactive request wins the tick, and the
planner, finding itself off the route, re-plans (that exists). The alternative — a `Steering`
that sums weighted requests, in the manner of classic steering behaviors — is more machinery,
worth it only once the simple rule fails somewhere real.

## 11. What changed

`navigationSystem` is a source of `Request` calls and no longer writes `Vel`; navigated units
carry `Steering` (their kind gives it); `arrivalEpsilon` applies to the goal only. `Steering`
holds the motion profile (`V0`, `Accel`, `Brake`, `TurnRate`) and `SteeringSystem` writes
`Vel.Value` every tick. `MoveOrder` keeps a queue of goals; Shift + right click appends. The board
makes solid terrain into bodies from the boxes of any grid (`Grid.CellBoxes`, `WithCollision`),
says who may stand where through domains (`Allows`, `Mover`), and reports where each unit
stands (`Standing`, `Fell`). Routes are checked against the terrain whenever it changes. Still
open: `RouteStyle` (§8), turn-based movement (§9), arbitration (§10), sight through terrain and
flying (§12).

The tests that pin it: a smooth turn (the heading's angle changes monotonically and the speed
never drops to zero at a bend); acceleration from `V0` to `MaxSpeed` by `Accel`; braking that
ends on the goal's centre, with a weak brake too; a terrain behavior that does not compound
across ticks; passing a waypoint by projection; entering a cell independently of waypoints; a
push into a wall leaves no unit inside it, on a square and on a hex grid; a land unit driven onto
a hole has fallen and a boat on water has not; an ice bridge melting ahead re-routes without a
step into the water; a unit stuck where it may not be keeps its order.

## 12. Sight through terrain, sight range and flying units — done

Terrain limits sight by kind, not switching it off: a forest takes range, a hill none, a wall all.
A world without heights is a set of planes — `world.Layers` — and sight and collision follow them;
heights are the next step, §14.

- **A budget, not a switch.** aabbworld v1.6.0's `Cone.Transparency` gives each entry a τ. A ray
  starts with the cone's radius as a budget: an empty stretch costs its length, a stretch through
  an entry costs its length divided by τ, an entry at τ ≤ 0 cuts. A forest at τ = 0.4 takes 2.5×
  its depth; a see-through entry the ray enters within its budget is seen like anything else
  (aabbworld v1.7.0 — v1.6.0 kept forests out of `Entities`, and a hawk looking over a walker
  needs them in); `Depths` and `Outline` show the shortened reach, so a cone fades into a forest
  instead of stopping at its edge. The choice of a budget over a multiplied attenuation is what
  keeps the sweep exact for walls and cheap for forests: the opaque-only scan measures the same as
  before, a scene with three entries in ten see-through costs about 8% more.
- **Veil per kind, transparency per body.** `CellKind.Veil` in 0..1 replaces `Opaque`; a veiled
  body carries a `vision.Transparency` of 1 - Veil, and the scan reads it off the entity it is
  about to cross — nothing else carries one, so walls and units still cut. The transparency is
  ECS state on the body, not a table beside it, and the terrain bodies did not need a `TypeID`
  per kind after all.
- **Range is the unit's.** `Sight.Radius`; `MaxSightRadius` only sizes the outline buffer, a
  longer sight sees as far as it says with a coarser outline.
- **Flying is a plane.** `Mover{Domain: Air}` keeps the planner on cells admitting `Air` (the
  demos admit it over the wall and the forest) and `Costing(Air, 1)` keeps the forest from slowing
  it. Everything else follows from `world.Layers`, the planes an entity is on: the hawk carries
  `Air`, walkers `Land`, a solid body the bits of whoever its kind keeps out (`^Allows`, so a wall
  admitting Air is on `Land|Water`), a veiled body its kind's `Veils` (a forest veils `Land`).
  Collision pairs only entities whose layers meet, so the hawk keeps its `Physics` and other
  flyers push it while walls and walkers pass under. Sight reads the same bits through
  `Sight.Blockers`, the layers that cut or dim an observer at all: the hawk's are `Air`, so a
  wall, a forest or a walker is as empty to it — looked over, and still seen when the ray enters
  it; a walker's are `Land`, so the hawk above shades nothing. `Sight.Clear` is gone: "over the
  veils" was one case of "on another plane". What a plane cannot say — a wall lower than the
  eye, a hill, a tower seen over the wall — is §14's, the 2.5D world, which is a conscious
  choice in `world.Config` and costs the scan about three times as much.

## 14. Heights — done for sight, open for collision

A world has heights when its game says so: `world.Config{Quasi3D: true}`. Nothing guesses the mode
from the data — a flat game pays nothing for heights, and a game that wants them says it once.

- **What carries height.** An entity's `world.Z{Altitude, Height}`: its bottom and its rise. A
  flat world carries no `Z` at all (`Kinds.Register` refuses one), so saves of flat games do not
  change. `Position` stays what it was, the box in XY.
- **Where it comes from.** The game states shape and movement as facts of a kind, through
  `board.NewUnits(brd, board.Shape{Size, Height}, at)` and `units.Define(name, board.Mover{Domain,
  Lift}, …)`: the factory gives every unit a `Z{Height}`, and the board writes `Z.Altitude` every
  tick — the ground under the unit's centre plus its `Mover.Lift` (`altitudeSystem`, Quasi3D only).
  A unit standing on terrain never declares its altitude; a hawk declares only how high it flies.
  Terrain kinds have an `Altitude` (the ground level) and a `Height` (what stands on the cell: a
  wall 10, a forest 8); the board's bodies carry `Z{Altitude, Height}` of their kind.
- **The ground is a raster, not bodies.** `Board` keeps one height per cell (`Grid.Ordinal`),
  rebuilt when the terrain's `Version` moves, and is the world's `Ground` (`GroundAt`, `Step` = a
  cell's shorter side). A hill is only a number in the raster — so the cost of a scan depends on
  the radius and the step, never on how many hills a game has. This is the rule that decided
  against modelling relief as bodies. On a square grid the ground slopes: each corner stands at
  the mean of the cells meeting there (`Board.Corners`), `GroundAt` interpolates between a cell's
  corners, so a hill is a smooth rise, a unit on its slope stands at the slope's height, and the
  isometric tiles are drawn tilted — Transport Tycoon's terrain without its corner editing. A
  game must keep its eyes in proportion to its relief: the smoothed edge of a plateau is a slope a
  unit or two below its top, and an eye lower than that difference sees the rim, not the valley
  (island-isometric-demo: hills 20, mountains 40, eyes 6).
- **Sight with heights.** aabbworld v1.7.0's `Cone.Eye/Elevation/Ground/GroundStep`: an entity is
  seen when the line from the eye (`Z.Altitude + Sight.Eye`) to its top clears every nearer ground
  sample and every nearer blocking band within the budget; the reach of an angle is the farthest
  lit ground. A walker (eye 1.5) is stopped by a wall 10 tall and by a hill; a hawk 40 up looks
  over both, and over the forest, which costs it nothing where its line passes above the band.
  `Sight.Blockers` has no place here and a Quasi3D world refuses it, as a flat one refuses
  `Sight.Eye`. The scan costs about three times the flat one at the same radius (~13 µs against
  ~4 µs at radius 300, measured in aabbworld's ladder benchmark); `vision.Plugin.WithGroundStep`
  trades ground samples for speed.
- **Collision stays on planes — open.** Two entities meet where their `world.Layers` share a bit,
  in both worlds; a hawk passes over a wall because it is on `Air`, not because it is 40 up. A veto
  by Z overlap would make collision follow height too (a hawk landing, a projectile clearing a
  wall) — a real change to the solver, left for when a game needs it.

## 13. Effects, tags and the profile — done

Anything temporary about a unit is an **effect** (`plugins/effects`): a tag granted for a while,
a component altered and restored — `Grant` and `Alter` in a `Spec`, `Lasts` or until `Dispel`,
cast from anywhere by entity id, saved with the entity. What an effect means for movement is the
reader's: a frozen unit carries a tag a `Moving` behavior reads as "stand still"; a slipping unit has
its `Brake` altered, so navigation brakes earlier before a goal and may not stop before ground
that turned against it. Tags are bits of families (`plugin.Tags[F]`), one component per family,
so granting one is a value write and the component budget stays for data.

The board joins in through **cell entities**: `board.Plugin.CellEntity(c)` gives a cell an entity
with a `Ground` the board copies into the terrain each tick, so an `Alter` of `Ground` is a
temporary change of terrain — an ice witch's frost — and the board drops the entity itself once it
finds it `Idle` after its last effect, with no wiring between the two plugins. Weather and seasons over the whole board are the same idea
on an entity standing for the board; not built yet.

## Who owns what

| Concept | Owner | Promises |
|:---|:---|:---|
| turning, accelerating, braking, the motion profile | `world.Steering`, `SteeringSystem` | "asked for a heading and a speed, the unit gets there as its profile allows, and `Vel` is rewritten every tick" |
| the route, the lookahead point, waypoints, when to brake, whether the route still holds | `navigation` | "the unit is asked, every tick, for the heading and speed that keep it on its route, and a route the ground no longer takes is dropped" |
| terrain, who may stand where, walls as bodies, who fell in, cell entities | `board` | "what may not be entered is a body in the world or ground that admits nobody, and the planner knows it; `Standing` says where everyone stands" |
| pushing apart, contacts, `Static` and `Sensor` | `collision` | "no unit ends a tick inside a wall" |
| what is temporary about an entity | `effects` | "a granted tag or an altered component holds while the effect runs, and the original comes back" |
