// Package navigation moves entities along a MoveOrder's path across a
// board, re-pathing automatically when terrain along the route changes,
// and carries out MoveTo commands for Selected entities;
// WithRenderer draws the remaining route.
//
// # MoveOrder and Path
//
// A [MoveOrder] on an entity commands it toward a Target cell and then through up to
// [MaxWaypoints] queued [Goal]s ([MoveOrder.Enqueue]), passing each without stopping; the order is
// removed when the last is reached. Where in a cell the entity stops is its Spot, zero the cell's
// centre, and At the point the order was given for. Its [Path] is the cached route, consumed step
// by step, at most [MaxPathLength] cells at a time with a longer route fetched in chunks; its
// [Leg] is the single step in flight. [CellEntered] is a one-tick tag added the tick an entity's
// Cell changes. A navigated entity carries a steering.Steering profile: navigation only asks it for
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
//     step is refused, the cell held: it waits, asks the one standing there off it, and after
//     stallAfter of no headway notes the cell for its routes to go round and plans afresh; a
//     corner of a slantwise step held is gone round square at once. One standing, asked off its
//     cell, gives way where it can: to a free cell square off the way the other comes, else beside
//     them, never ahead of them — a GivingWay order aside with Linger, then home — and nobody gives
//     way to one giving way. Two coming at each other's cells: the one with the greater id goes
//     round, the other waits. Someone standing on a unit's goal is waited targetWaitTimeout for,
//     then the unit settles on the nearest free cell; someone passing over it is waited for. One
//     that struck someone bodily (a Struck behavior navigation registers on the board's collision
//     plugin) stops, plans again from where it stands and keeps that route for a while whatever
//     bumps follow — MoveOrder.Bumped and Cooldown. Occupancy is seeded from every entity's Cell
//     and Mover when the Stage is set up, fresh or loaded. Board games and units a cell large.
//   - [BodySpacing] keeps units apart by their boxes, several standing in one cell, and does not
//     ask the board's Occupancy. A unit routes over the ground alone, not knowing where the others
//     stand, and goes from cell centre to cell centre; it learns of the others only by striking
//     them (WithCollision). Striking someone, it steps round them towards its goal, never back
//     into them nor onto ground its domain may not take — with no ground aside it waits; one it
//     struck standing has its cell noted for the routes to go round, and when that one stands on
//     its spot it stands elsewhere round the same point. One standing, struck by one on the move,
//     gives way: it steps just off the line between them, to the side it stands on, where the
//     ground takes it, lingers there a second (MoveOrder.Linger) and goes back to where it stood,
//     facing as it did — an order marked GivingWay, to which nobody gives way in turn. Coming no
//     nearer the cell it heads for, or striking again one it knew stands there, is a stall: after
//     a few it stands where it is, as it does when it strikes a second one standing close by its
//     spot, among its group.
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
// # The price of a step
//
// A route is the cheapest way over the cells: a step costs the destination kind's CostFor the
// unit's domain over the step's length (√2 slantwise on a square grid), times the slope the board's
// Map prices (topography's Climbing) unless the kind is Graded (board.CellKind.Graded: a road cut
// into the slope costs its Cost alone). A slantwise step not along a way (board.Board.Along) cuts
// the corner beside the way, over the ground bare of it (board.Board.Bare), and costs that ground
// — none where it does not admit the unit, the water beside a bridge: a road is followed round
// its bend rather than cut across the grass, and a road laid slantwise is taken along its links at
// its own price.
//
// # Commands
//
// A [MoveTo] sends every Selected entity to a cell, or with Append queues the cell behind their
// orders; the plugin is a plugin.CommandHandler ([Plugin.Queues] is the queue) and its command
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
// cell by cell, with CellEntered.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the [PathRenderer], drawing, for every selected entity, its goals
// — the entity's outline where it will stand, on the ground there, on the render.Marks tier,
// always — and its routes when they are shown: the remaining route and the routes on to each
// queued goal, a thin line over the ground. In a world with heights, through a camera with Rays,
// the PathRenderer is a render.Direct laying the routes on the GPU on [RouteTier]: every pixel near
// a stretch finds the ground point drawn there from the frame's depth (shaders/route.wgsl), so the
// line follows every rise and a hill in front hides it. Otherwise it hands the frame the line in
// pieces of the ground's step on the render.Overlays tier, each at the depth of the ground under
// it, so the line runs straight through any camera. [Routes] (Shift+P) shows the routes
// and hides them again ([Plugin.ShowRoutes], [Plugin.RoutesShown]); [RouteStyle]
// ([Plugin.WithRouteStyle], [DefaultRouteStyle]) is the line's colour and width and the goal's
// colour.
package navigation
