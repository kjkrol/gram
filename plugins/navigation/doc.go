// Package navigation moves entities along a MoveOrder's path across a board, re-pathing when the
// ground along the route changes, and carries out MoveTo commands for Selected entities.
//
// # MoveOrder and Path
//
// A [MoveOrder] on an entity sends it to a Target cell and then through up to [MaxWaypoints]
// queued [Goal]s ([MoveOrder.Enqueue]); the order is removed when the last is reached. A patrol
// is a plan of two loops (plan.Steps): the unit carries a [Round] of cells and gives itself
// [Onward] for the next until [Arrived] comes, then waits. Its [Path] is the cached route, consumed
// step by step, planned again when the entity stands off it; its [Leg] is the single step in
// flight. A navigated entity carries a steering.Steering profile, which navigation asks for a
// heading and a top speed; a MoveOrder is put on by the MoveTo command, or by the game at spawn.
//
// # Spacing
//
// How units keep out of each other's way is the plugin's [Spacing] ([Plugin.WithSpacing]):
//
//   - [CellSpacing] gives a unit a cell to itself in each domain, as the board's Occupancy says,
//     on each of a cell's levels; a group sent to one cell spreads a unit to a cell each.
//   - [BodySpacing] keeps units apart by their boxes, several standing in one cell; a group sent
//     to a point is given its spots round it, a box and a gap apart.
//   - [AutoSpacing], the default, is BodySpacing when the world's largest box is at most a third
//     of a cell's shorter side, CellSpacing otherwise.
//
// Under either a unit routes over the ground alone, not knowing where the others stand, and learns
// of them only by touching them: a step into a held cell refused, or its box striking theirs
// ([Plugin.WithCollision]).
//
// # Crowd
//
// How units get on among others is rules of the moment [Touch], which the plugin hosts: two units
// touching, handed to each of the two every tick they do, with whether each is on the move or
// giving way, allies or of one order, on the other's goal, head on, with room to step aside. A
// rule's commands are carried out for the unit alone: [StepAside], [Detour], [Pass], [Hold],
// [Settle], [Stop]. A Touch's words are of the unit — [PushedByAlly], [ReachedTheGroup],
// [InTheWay], [ClearsTheWay], [Blocks], [GoalTaken], [StrangerOnMyGoal], [WaitsFirst],
// [NoWayRound], [WaitedLong] — and the other's role is cond.Other.If(role). The plugin's own rules
// are StarCraft II's; a game gives its own with [Plugin.WithCrowd], or adds rules obeyed by the
// roles its units play. Whatever the rules, navigation keeps the last word: a unit making no
// headway plans afresh and, after a few stalls, stands where it is. A unit with a plan is told
// the facts [Blocked] (its words [Stuck], [Cornered], [StrangerOnGoal], [IdleAlly],
// [IdleAllyOnMyGoal]) and [Arrived].
//
// # The price of a step
//
// A route is the cheapest way over the cells: a step costs the destination kind's CostFor the
// unit's domain over the step's length, times the slope the board's Map prices unless the kind is
// Graded. A slantwise step not along a way cuts the corner over the ground bare of it and costs
// that ground, so a road is followed round its bend. In a world with heights a cell under a raised
// deck has two levels, and a route goes by the unit's Z: onto the deck within its step, along it,
// off it only onto a surface within the step, under it only where the unit fits; a step across a
// railing is never planned.
//
// # Commands
//
// A [MoveTo] sends every Selected entity of the player who gave it, those it owns alone, to a cell
// on a [Level], or with Append queues the cell behind their orders; a [LookAt] stops them and turns
// them towards a point. [Plugin.DefaultBindings] make a right click a MoveTo, Shift appending, a
// right drag a LookAt at every move, and Shift+R [Routes]. Driving a unit by hand is the driving
// plugin's, which [NewPlugin] takes: a unit a hand is on gives its order up.
//
// # Renderer
//
// [Plugin.WithRenderer] builds the renderer of routes: for every selected entity its goals, the
// entity's outline where it will stand, always, and its routes as thin lines over the ground while
// [Routes] shows them ([Plugin.ShowRoutes], [Plugin.RoutesShown]); [RouteStyle]
// ([Plugin.WithRouteStyle], [DefaultRouteStyle]) is the lines' colour and width and the goals'
// colour.
package navigation
