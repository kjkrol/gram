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
// through ([weather.State]s), the one a fresh game begins in and the seed of its dice. The
// weather now is a [Weather] on its own entity, saved with the game; [Climate.System] moves it on
// in every step of the simulation, throwing the next weather as one runs out and blending the
// wind, the clouds, what falls and the temperature towards it. [Climate.Air] is the air as the
// last step left it, for whoever draws. [Climate.Change] goes on to the next weather,
// [Climate.Set] into a named one; [Climate.SetRunning] stops what of it a game wants still
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
