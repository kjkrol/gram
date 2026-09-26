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
// # Plugin
//
// [NewPlugin] takes the world, the sky and the [Config] — the Zone, the weathers the climate goes
// through (package weather: its [weather.State]s, [weather.Default] when none), the one a fresh
// Stage begins in, the seed of its dice and how long the air takes to settle into a new weather —
// and has the sky's sun go the way it goes at the zone's latitude (sky.Plugin.SetLatitude). The
// weather now is a [Weather] on the plugin's own entity, made at Setup or found after a load, so
// it is saved with the game, dice and all: the same seed gives the same weather. It goes by in
// the sky's time — as far as the sky's Day has moved since it last looked, at its Pace or jumped
// on while it stands, never back — so hurrying the day on hurries the weather and its seasons
// together. Every tick it counts its weather down and throws the next from its weights, how
// likely each comes in the season and, for one that brings rain or snow, how wet the zone has the
// season; brings the wind, the clouds, what falls and the temperature — the zone's for the time of
// year and the hour, and the weather's own Warmth — towards the weather's, the wind's way
// wandering slowly; carries the clouds on the wind; lets what falls come down as snow below 1°C
// and as rain above; and sets the world's weather (world.Weather), which the board's and the
// world's renderers draw: the clouds' shadows drifting over the ground, the sea as rough as the
// wind, whatever sways swaying. A fresh Stage begins in the Start weather, or in one thrown as
// the zone and the season have them.
//
// # Behaviours
//
// [Every] hosts a game's behaviour, told the weather and the season ([Weathering]) every tick,
// its Tick's Dt the sky's time: where it casts its effects (plugins/effects) as the weather says
// — snow lying while it snows in the frost and melting once it is warm, ice on the water, trees
// swaying in the wind — and takes them off again. The climate knows nothing of those effects;
// they are the game's, as the islands' show.
//
// The plugin is a plugin.CommandHandler: [Change] (W) goes on to the next weather now, [Set] into
// a named one — a game scripting its weather. [Plugin.Renderer] is what falls, a render.Source
// for a scene's Composer: rain streaking with the wind, snow drifting, over the world and under
// the selection (render.Air). [Plugin.Reporter] adds the weather to the telemetry. Call
// [Plugin.RunPlan] every tick, after the sky's, before the world is drawn.
package climate
