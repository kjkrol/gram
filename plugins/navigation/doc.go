// Package navigation moves entities along a MoveOrder's path across a
// board, re-pathing automatically when terrain along the route changes,
// and carries out MoveTo commands for Selected entities;
// WithRenderer draws the remaining route.
//
// # MoveOrder and Path
//
// A [MoveOrder] on an entity commands it toward a Target cell and then through up to
// [MaxWaypoints] queued goals ([MoveOrder.Enqueue]), passing each without stopping; the order is
// removed when the last is reached. Its [Path] is the cached route, consumed step by step, at most [MaxPathLength] cells at
// a time with a longer route fetched in chunks; its [Leg] is the single step in flight — every
// cell it holds in Occupancy until it reaches the next centre. [CellEntered] is a one-tick tag
// added the tick an entity's Cell changes. A navigated entity carries a world.Steering profile:
// navigation only asks it for a heading at the lookahead point and for its own top speed, braking
// from the profile before the goal. An entity that struck someone (a Struck behavior navigation
// registers on the board's collision plugin) stops, plans again from where it stands and keeps
// that route for a while whatever bumps follow — MoveOrder.Bumped and Cooldown; the board's
// Occupancy, kept per domain, is what the new route goes round. Occupancy is seeded from every
// entity's Cell and Mover when the Stage is set up, fresh or loaded. The [Plugin], built over a
// board and a world, runs before the world's RunPlan.
//
// Navigation requires of every unit a world.Steering, the profile it is steered by, through the
// world's kind.Roster; a MoveOrder is put on by the MoveTo command, or by the game at spawn.
//
// # Commands
//
// A [MoveTo] sends every Selected entity to a cell, or with Append queues the cell behind their
// orders; the plugin is a plugin.Commander ([Plugin.Commands] is the inbox) and its command
// system issues or extends the orders. [Plugin.DefaultBindings] make a right click one, Shift +
// right click an appending one. A right click on the cell a Selected entity stands on turns it
// towards the point clicked (MoveTo.At), and a right click with S held is a [LookAt]: every
// Selected entity finishes its step, stops and turns there — both through [MoveOrder].Face, the
// point an entity turns towards on arrival.
//
// # Renderer
//
// [Plugin.WithRenderer] draws the remaining route of every selected entity, and the routes on to
// each queued goal, with the [PathRenderer]
// from a [PathSprites] set — one arrow per [Direction] (every [DirectionStep] degrees round the compass, so square and hex steps land on one exactly) and a dot; [RegisterDefaultPathSprites]
// bakes a default set.
package navigation
