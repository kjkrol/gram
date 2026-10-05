// Package hooks holds ready-made rules of what an entity sees, for ctx.Hook
// (game.Initializer.Hook): Flee gives way to what is on a collision course and runs from a Threat,
// Chase goes after the nearest prey and Search looks round when there is none. They steer only by
// the commands an entity gives itself (steering.Away, Toward, Turn). A game wanting something else
// writes its own rule of a vision.Sighting (rule.On).
//
// # Flee
//
// [Flee] has every Skittish entity head away from the nearest Threat it sees, else from the
// nearest one in view when either is heading at the other. Its rules run During an effect the
// game defines and puts on the world (rule.Cast on entity.World), taking it off to switch the
// fleeing off (rule.Lift, or a Toggle).
//
// # Chase and Search
//
// [Chase] has every Predator head at the nearest Prey it sees. [Search] has one that sees none
// turn a quarter aside, once every while the effect [Looked] defines lasts.
//
// # Tags
//
// Of the [Tags], Predator and Prey are the two sides of a hunt, Skittish the entities Flee steers,
// Threat what is fled on sight; [DefineTags] makes them. A file using both collision's and
// vision's hooks imports them as chooks and vhooks.
package hooks
