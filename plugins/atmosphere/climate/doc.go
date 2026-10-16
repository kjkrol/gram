// Package climate is the climate of a world: where in the world it lies, and the weather that
// goes by there.
//
// # Zone
//
// A [Zone] is where a climate lies: its latitude, and [Factor]s that shape it on top — a
// [SeaCurrent], a [DrySummer], a game's own. [Zone.Profile] is the climate in numbers.
// [Equatorial], [Tropical], [Mediterranean], [Temperate], [Cold] and [Polar] are the world's
// zones from the equator to the pole.
//
// # Climate
//
// [New] takes the world, the calendar and a [Config]: the zone, the weathers the climate goes
// through ([weather.State]s), the one a fresh game begins in, the seed of its dice, how far past
// the world its sky reaches (Horizon) and whether its weathers come lane by lane (Patchy). The
// weather is a [Weather] on its own entity, its air masses air.Patch entities, saved with the
// game; [Climate.System] moves them on in every step of the simulation. A new weather is thrown
// as the last runs out and born at the horizon the wind blows from; the wind carries it in over
// the world as the old one sails off, so the weather over the world changes as the front comes,
// and a weather lasts at least as long as the wind takes to carry it across the world. In a
// Patchy climate each [Lane] of the horizon throws its own, likelier its neighbours'. Without
// wind the weather settles where it is. [Climate.Change] goes on to the next weather,
// [Climate.Set] into a named one, either over the world and upwind of it within seconds.
// [Climate.Air] is the air as the last step left it, the weather now that over the world's
// middle, for whoever draws; [Climate.SetRunning] stops what of it a game wants still
// ([Running]).
//
// # Weathering
//
// [Climate.Rules] hosts rules of a [Weathering], told the weather and the season once a step
// about the atmosphere's own entity: they fire while the atmosphere plays their role.
//
// [Climate.Reporter] adds the weather to a scene's telemetry.
//
// # Vocabulary
//
// Moments, for a role's rules: [Weathering] (the atmosphere's own entity, once a step).
package climate
