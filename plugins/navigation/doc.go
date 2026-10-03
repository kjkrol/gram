// Package navigation moves entities along a MoveOrder's path across a
// board, re-pathing automatically when terrain along the route changes,
// and carries out MoveTo commands for Selected entities;
// WithRenderer draws the remaining route.
//
// # MoveOrder and Path
//
// A [MoveOrder] on an entity commands it toward a Target cell and then through up to
// [MaxWaypoints] queued [Goal]s ([MoveOrder.Enqueue]), passing each without stopping; the order is
// removed when the last is reached — unless it has a [Round], a patrol ([Patrol]): then, reached
// or given up, it goes on to the round's next goal, standing its pause on each one reached, for
// ever. A kind gives a wanderer or a guard its own round (comp.Load). Where in a cell the entity stops is its Spot, zero the cell's
// centre, and At the point the order was given for. Its [Path] is the cached route, consumed step
// by step, at most [MaxPathLength] cells at a time with a longer route fetched in chunks; its
// [Leg] is the single step in flight. Its markers ([States], carried for good — the plugin gives
// them to every unit the world's roster makes) have [Entered] on for the step its At changed:
// the At says which cell it entered. A navigated entity carries a steering.Steering profile: navigation only asks it for
// a heading at the lookahead point and for its own top speed, braking from the profile before the
// goal. The [Plugin], built over a board and a world, runs before the world's RunPlan.
//
// Navigation requires of every unit a steering.Steering, the profile it is steered by, through the
// world's kind.Roster; a MoveOrder is put on by the MoveTo command, or by the game at spawn.
//
// # Spacing
//
// How units keep out of each other's way is the plugin's [Spacing] ([Plugin.WithSpacing]):
//
//   - [CellSpacing] gives a unit a cell to itself in each domain, as the board's Occupancy says. A
//     Leg holds every cell its step touches until it reaches the next centre, a group sent to one
//     cell spreads a unit to a cell each, and every unit stands at its cell's centre. A unit routes
//     over the ground alone, not knowing where the others stand, and learns of them only when a
//     step is refused, the cell held: a [Touch] of whoever holds it. Refused for stallAfter,
//     whatever the rules do, it notes the cell for its routes to go round and plans afresh; a
//     corner of a slantwise step held is gone round square at once. One that struck someone bodily
//     (navigation reads its collision.Collider contacts in its own pass) stops, plans again
//     from where it stands and keeps that route for a while whatever bumps follow —
//     MoveOrder.Bumped and Cooldown. Occupancy is seeded from every entity's Cell and Mover when the
//     Stage is set up, fresh or loaded. Board games and units a cell large.
//   - [BodySpacing] keeps units apart by their boxes, several standing in one cell, and does not
//     ask the board's Occupancy. A unit routes over the ground alone, not knowing where the others
//     stand, and goes from cell centre to cell centre; it learns of the others only by striking
//     them (WithCollision): a [Touch]. It steps round the solid ground it strikes; coming no nearer
//     the cell it heads for is a stall, answered by a fresh route, and after a few it stands where
//     it is.
//   - [AutoSpacing], the default, is BodySpacing when the world's largest box is at most a third of
//     a cell's shorter side, CellSpacing otherwise.
//
// Under BodySpacing a group sent to a point is given its spots round it, planned over the ground
// and the group alone: a lattice round the point, a box and a box's gap apart, filling the point's
// cell first and then the cheapest cells round it; no spot where the ground does not take the unit
// or lies on a step — its top under a corner of the box more than half the unit's height off the
// top under its middle; the rows furthest along the way the group comes going to the units
// furthest on, so none passes one of its group standing already.
//
// # Crowd
//
// How units get on among others is rules: rules of the moment [Touch], which the plugin hosts
// (ctx.Hook, or [Plugin.Hook]; rule.On over a Touch). Navigation perceives and carries out; the
// rules decide. A Touch is two units touching, handed to each of the two every tick they do:
// whether each is on the move or giving way, whether they are allies (players/owner.Allies) or of
// one MoveTo — the order each is under ([MoveOrder].Group), or the last it came to the end of
// ([LastOrder]) — whether the other stands on the unit's goal, whether the two come head on,
// whether the one standing has room to step aside. A rule's commands (Order, aimed at the other)
// are carried out for the unit alone: [StepAside] steps it off the other's way where the ground
// takes it — never into water or a hole, off a cliff (no steeper than yieldClimb) or into a wall —
// and it stays there, or, on the move, a while and on; [Detour] goes round the other — Touch and
// Blocked say it is cornered with no way round; [Pass] goes on past one making way; [Hold] waits
// for the way ahead to clear, a while at most; [Settle] stands beside the goal; [Stop] ends the
// order where the unit stands, as come to the end of it.
//
// The crowd's rules, the plugin's own, are StarCraft II's, hooked unless a game gives its own
// ([Plugin.WithCrowd]): an ally standing makes way and stays aside while the one on the move goes
// on past it; one on the move stops on touching one of its order that has arrived, so a group
// gathers round its point and nobody fights for its exact spot; one on the goal who does not make
// way — a stranger, an ally with no room — has the unit stand beside it; anyone else in the way is
// gone round — with no way round, the unit steps aside a while; of two head on the first waits. A
// game's own rules of Touch — narrowed by tags as any rule's — go beside them (ctx.Hook, or
// [Plugin.Hook]). Whatever the rules, navigation keeps the last word: a unit making no headway
// plans afresh and, after a few stalls, stands where it is. A unit with a plan (package rule) is
// told the facts [Blocked] while someone blocks it and, once its order is over, [Arrived]; a
// [MoveTo] or [LookAt] it gives itself orders it alone.
//
// # The price of a step
//
// A route is the cheapest way over the cells: a step costs the destination kind's CostFor the
// unit's domain over the step's length (√2 slantwise on a square grid), times the slope the board's
// Map prices (topography's Climbing) unless the kind is Graded (cell.Kind.Graded: a road cut
// into the slope costs its Cost alone). A slantwise step not along a way (board.Board.Along) cuts
// the corner beside the way, over the ground bare of it (board.Board.Bare), and costs that ground
// — none where it does not admit the unit, the water beside a bridge: a road is followed round
// its bend rather than cut across the grass, and a road laid slantwise is taken along its links at
// its own price. The routes are found by the plugin's own path finder (internal/pathfind); the
// routes on the GPU are drawn by its own lines (internal/routes): neither is the game's to use.
//
// # Commands
//
// A [MoveTo] sends every Selected entity of the player who gave it — those it owns alone
// (players/owner.Obeys) — to a cell, or with Append queues the cell behind their orders; the plugin is a plugin.CommandHandler ([Plugin.Queues] is the queue) and its command
// system issues or extends the orders. [Plugin.DefaultBindings] make a right click one — the
// button up where it went down, within a few pixels — Shift + right click an appending one, and a
// right drag a [LookAt] at every move of the cursor: every Selected entity stops and turns to
// look where the cursor goes, through [MoveOrder].Face, the point an entity turns towards on
// arrival, and the drag's release moves nothing. Under CellSpacing a right click on the cell a Selected entity stands on turns
// it towards the point clicked (MoveTo.At) and a LookAt lets it finish its step; under BodySpacing
// MoveTo.At is where the group stands round, a click in the entity's own cell moves it there, and a
// LookAt stops it where its braking ends.
//
// # Driven by hand
//
// An entity carrying a steering.Driven — written every tick by whoever steers it, the isometric
// camera fastened to it — is carried out after the orders: it turns by hand (Turn, or towards Face:
// where an eye riding in it looks), walks on the way it faces while the ground just ahead is a
// cell its domain may stand on and nobody is in the way — the occupancy lets it into the cell, or,
// under BodySpacing, it touches nobody just ahead — and stops dead otherwise, so it never walks into
// the sea; with no hand on it, it brakes. A hand ends any order it had, giving up the cells of the
// step in progress; with none, the order goes on. Its Cell and its hold on the occupancy follow it
// cell by cell, with Entered.
//
// A player's hand is the command [Drive]: every tick a key is held ([DriveBindings], W, S, A and D,
// which a game binds in place of the camera's own keys on them) the player's selected units get
// their Driven written, several Drives in a tick adding up, and the marker [Driving]; a tick
// without one brakes a Driving unit, and once it stands — or has an order to go on with — its
// Driven is taken off, so it steps aside for others again. A Driven navigation did not give, a
// camera's, is left alone. A unit under orders struck by what was only sensed — a shot, a sensor —
// is not Bumped by it: a sensor blocks nobody.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the renderer of routes, drawing, for every selected entity, its goals
// — the entity's outline where it will stand, on the ground there, on the render.Marks tier,
// always; a step aside is no goal — and its routes when they are shown: the remaining route and the routes on to each
// queued goal, a thin line over the ground. In a world with heights, through a camera with Rays,
// the renderer is a render.Direct laying the routes on the GPU, over the ground and under what
// stands on it: every pixel near a stretch finds the ground point drawn there from the frame's
// depth (its route shader), so the
// line follows every rise and a hill in front hides it. Otherwise it hands the frame the line in
// pieces of the ground's step on the render.Overlays tier, each at the depth of the ground under
// it, so the line runs straight through any camera. [Routes] (Shift+P) shows the routes
// and hides them again ([Plugin.ShowRoutes], [Plugin.RoutesShown]); [RouteStyle]
// ([Plugin.WithRouteStyle], [DefaultRouteStyle]) is the line's colour and width and the goal's
// colour.
package navigation
