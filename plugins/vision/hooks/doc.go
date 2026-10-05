// Package hooks holds ready-made rules of what an entity sees, for the roles of a game to obey:
// Flee gives way to what is on a collision course and runs from a threat, Chase goes after the
// nearest prey and Search looks round when there is none. They steer only by the commands an
// entity gives itself (steering.Away, Toward, Turn). A game wanting something else writes its own
// rule of a vision.Sighting (rule.Then).
//
//	prey := rule.Role("prey")
//	predator := rule.Role("predator").Obeys(hooks.Chase(prey), hooks.Search(prey, looked))
//	skittish := rule.Role("skittish").Obeys(hooks.Flee(predator, fleeing)...)
//
// # Flee
//
// [Flee] has those obeying it head away from the nearest one in view playing the role given, else
// from the nearest one in view when either is heading at the other. Its rules run During an
// effect the game defines and puts on the world (rule.Cast on entity.World), taking it off to
// switch the fleeing off (rule.Lift, or a Toggle).
//
// # Chase and Search
//
// [Chase] has those obeying it head at the nearest one in view playing the role given. [Search]
// has one that sees none turn a quarter aside, once every while the effect [Looked] defines
// lasts. A file using both collision's and vision's hooks imports them as chooks and vhooks.
package hooks
