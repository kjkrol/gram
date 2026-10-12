// Package gravity has bodies fall, stand on what bears them and leap: a character's controller,
// never rigid bodies.
//
// # Pull and Body
//
// [NewPlugin] gives the world its gravity, a [Pull] on the plugin's own entity: a knob an effect's
// Alter turns. A kind falls carrying a [Body], which says how high it leaps; the plugin gives it
// a world.Drift, the velocity it is carried at beyond its walk. Whatever carries a Body is a unit
// of the board: unit.Airborne is on while it is in the air.
//
// # Two modes
//
// The world's dimension is the mode (world.Config.Heights), and each refuses the other's facts.
// In a world with heights the pull is along Z: every step a body stands on the surface under its
// centre ([Plugin.WithSurfaces], the board's ground.Surfaces), follows the ground down a slope
// within its reach, flies off a deck's edge, by a leap or a throw, and lands. A flat world, a
// platformer's, pulls across it any way: a body is borne by whatever collision found it touching
// against the pull, a wall bearing nothing.
//
// # Commands and Landing
//
// [Leap] and [Away] are an entity's own (rule.Apply, driving.Jump): a leap throws a standing body
// up to its Body's height, an Away throws it off across and up, away from the subject of the
// moment it is given on. A [Landing] is told the rules the plugin hosts once as a body lands,
// with how fast it fell ([HarderThan]). Run the plugin's RunPlan right after collision's.
package gravity
