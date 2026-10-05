// Package climate is the climate of a world: where in the world it lies, and the weather that
// goes by there — the wind, the clouds, rain and snow, the temperature.
//
// # Zones
//
// A [Zone] is where a climate lies: its Latitude, degrees from the equator, which above all sets
// how warm it is and how its seasons go, and other [Factor]s that shape it on top — a [SeaCurrent]
// warming or cooling it, a [DrySummer] — which a game adds to as its world needs. [Zone.Profile]
// is the climate in numbers: the year's mean temperature, how far summer and winter go above and
// below it, how far day and night, how wet each season is. [Equatorial], [Tropical],
// [Mediterranean], [Temperate], [Cold] and [Polar] are the world's zones from the equator to the
// pole; [Temperate] is central Europe and southern Scandinavia before the warming.
//
// # Weather
//
// [New] takes the world, the calendar and the [Config] — the Zone, the weathers the climate goes
// through (package weather: its [weather.State]s, [weather.Default] when none), the one a fresh
// game begins in, the seed of its dice and how long the air takes to settle into a new weather.
// The weather now is a [Weather] on its own entity, made at Setup or found after a load, so it is
// saved with the game, dice and all: the same seed gives the same weather. It goes by in the
// simulation's time — [Climate.System] runs in every step, under the clock's tempo, standing in
// the tactical pause — on the calendar's day and season. Every step it counts its weather down
// and throws the next from its weights, how likely each comes in the season and, for one that
// brings rain or snow, how wet the zone has the season; brings the wind, the clouds, what falls
// and the temperature — the zone's for the time of year and the hour, and the weather's own
// Warmth — towards the weather's, the wind's way wandering slowly; carries the clouds on the wind;
// lets what falls come down as snow below 1°C and as rain above; and keeps the air as it stands
// ([Climate.Air], an air.Weather), which the renderers draw: the clouds' shadows drifting over the
// ground, the sea as rough as the wind, whatever sways swaying. A weather's wind, cloud cover and
// heaps are thrown within its State's ranges as it comes. A fresh game begins in the Start
// weather, or in one thrown as the zone and the season have them. [Climate.SetRunning] stops what
// of it a game wants still ([Running]: the changes, the wind, the clouds, what falls), the air
// left without it, the weather going on underneath.
//
// [Climate.Host] hosts a rule (rule.Then) told the weather and the season ([Weathering]) once a
// step, about the world's own entity: an effect it applies is a state of the whole game. A moment
// of the world as a whole (plugin.StepRules), its rule takes no filter and obeys no role — Host
// refuses one filtered or narrowed with plugin.ErrUnhosted; read the world's effects with During,
// or write the rule over entities. [Change] (Shift+W) goes on to the next weather now, [Set] into
// a named one — a game scripting its weather ([Climate.Queues], [Climate.DefaultBindings]).
// [Climate.Reporter] adds the weather to the telemetry. The atmosphere plugin (plugins/atmosphere)
// puts it all together.
package climate
