// Package gravity has bodies fall, stand on what bears them and leap: a character's controller, as
// in a game where physics and not a rule says where one may go — never rigid bodies.
//
// # Pull and Body
//
// [NewPlugin] gives the world its gravity, a [Pull] on the plugin's own entity — a knob, so an
// effect's Alter turns it: gravity turned over, a moon's. A kind falls carrying a [Body], which
// says how high it leaps; the plugin gives it a world.Drift, the velocity it is carried at beyond
// its walk, which the world's move pass moves it by. Whatever carries a Body is a unit of the
// board (its markers): unit.Airborne is on while it is in the air, and a unit in the air has not
// fallen in where it is over (unit.Standing.Fallen).
//
// # Two modes
//
// The world's dimension is the mode (world.Config.Heights), and each refuses the other's facts. In
// a world with heights the pull is along Z: every step a body stands on the surface under its
// centre — the ground, or a deck carrying its domain within its step ([Plugin.WithSurfaces], the
// board's ground.Surfaces) — following the ground down a slope within its reach, never into the
// air; off a deck's edge, by a leap or a throw it flies, its head stopped by a slab over it, and
// lands. A flat world, a platformer's, pulls across it, any way: a body is borne by whatever
// collision found it touching (its collision.Collider) facing against the pull — a wall bears
// nothing — the drift running into them taken; the pull turned up, everything lands on the
// ceiling.
//
// # Commands and Landing
//
// [Leap] and [Away] are an entity's own (rule.Order, driving.Jump): a leap throws a body standing
// up to its Body's height, an Away throws it off across and up — from whoever the moment it is
// given on is about (bullet.Blast's shot), else back from the way it faces. A [Landing] is told
// the rules the plugin hosts once as a body lands, with how fast it fell: a rule says what a hard
// fall does. Run the plugin's RunPlan right after collision's, so the steps' contacts and the
// ground under the moved boxes are those it stands the bodies on.
package gravity
