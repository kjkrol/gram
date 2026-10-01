// Package hooks holds ready-made rules of what an entity sees, for the vision plugin's Hook: Flee
// gives way to what is on a collision course and runs from a Threat, Chase goes after the nearest
// prey and searches when it sees none. A game wanting something else writes its own rule of a
// vision.Sighting (rule.On).
//
// # Flee
//
// [Flee] steers every [Skittish] entity away from whatever is closing on it, and from any
// [Threat] the moment it comes into view; hook its [Flee.Rule]. The tags come from [DefineTags].
// [Flee.SetEnabled] switches it off and on without unhooking it.
//
// # Chase
//
// [Chase] has every [Predator] steer at the nearest [Prey] it sees; seeing none, it turns a
// quarter aside every lookEvery, searching.
//
// # Tags
//
// [Predator] and [Prey] are the two sides of a hunt, [Skittish] the entities Flee steers, [Threat]
// what is fled on sight. A file using both collision's and vision's hooks imports them as chooks
// and vhooks.
package hooks
