// Package sky is the light of the day over a world: the [Sun] and the moon of the calendar's hour.
//
// # Sun
//
// A [Sun] is a direction towards it, a strength, the sky's ambient light and the colours of its
// light and of the sky. [Sun.Light] and [Sun.Shaded] are the light it casts on a surface, a [Lamp]
// the sun ready to light many; [Sun.Frame] hands a render.Frame its shader's uniforms
// (shaders/sun.wgsl) and [Sun.ShadowOf] is the [Patch] an entity's shadow covers on the ground.
// [DefaultSun] is the sun of a relief under no sky.
//
// # Sky
//
// [New] takes the calendar, a [Config] and the latitude. Once a tick ([Sky.Update], [Sky.System])
// the sky sets its light ([Sky.Sun]) to [Config.LightAt] the hour: the sun on its path, or once it
// is well below the horizon the [Moon] — a knob on the atmosphere's entity, which an effect may
// turn, beginning as [Config.Moon] says ([DefaultMoon], [Sky.Moonlight]); off with [Sky.SetMoon].
// [Sky.Heavens] is where the sun, the moon and the stars stand at that hour, the sun's own colour
// there among them (Heavens.SunColor).
//
// The light by the sun's height is a table of [Daylight] rows, [Config.Daylight]: the sky's colour,
// the sun's and the ambient light, blended between rows; the Earth's ([EarthDaylight]) when none.
// The sky's colours, the world's light and the sun's disc all come from it.
//
// [Sky.SetFrozen] stops the light at the hour it stands and [Sky.Shift] moves a frozen one; only
// the light freezes, a look at the world that is not saved.
//
// # Moonrise
//
// [Moonrise] is the moment the moon comes over the horizon, told to the rules of the roles the
// atmosphere plays ([Sky.Rules], [Sky.RiseSystem]); its word [FullMoon] picks a full moon.
//
// [Sky.Reporter] adds the light to the telemetry panel.
//
// # Vocabulary
//
// Moments, for a role's rules: [Moonrise] (the atmosphere's own entity, as the moon rises; its
// word [FullMoon]). Knobs, for an effect's Alter: [Moon].
package sky
