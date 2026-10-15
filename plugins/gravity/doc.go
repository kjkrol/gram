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
// The pull's way is the mode, kept as long as the world lasts. Along Z (a world with heights) every
// step a body stands on the surface under its centre ([Plugin.WithSurfaces], the board's
// ground.Surfaces), follows the ground down a slope within its reach, flies off a deck's edge, by
// a leap or a throw, and lands. Across X and Y (a platformer's side view, its Z the depth) a body
// is borne by whatever collision found it touching against the pull, a wall bearing nothing.
//
// # Commands and Landing
//
// [Leap] and [Away] are an entity's own (act.Self.Do, driving.Jump): a leap throws a standing body
// up to its Body's height, an Away throws it off across and up, away from the subject of the
// moment it is given on. A [Landing] is told the rules the plugin hosts once as a body lands,
// with how fast it fell ([HarderThan]). Run the plugin's RunPlan right after collision's.
//
// # Vocabulary
//
// Moments, for a role's rules: [Landing] (once as a body lands; its word [HarderThan]). Knobs, for
// an effect's Alter: [Pull] (on the plugin's own entity). Commands, an entity's own: [Leap], [Away]
// (aimed at the moment's subject, host.Aimed).
package gravity
