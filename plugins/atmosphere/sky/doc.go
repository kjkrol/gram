// Package sky is the light of the day over a world: the [Sun] and the moon of the calendar's hour.
//
// A [Sun] is a direction towards it, a strength, how much of the sky's light every surface gets
// anyway, and the colours of its light and of the sky (white when zero); [Sun.Light] is the light
// — a render.Light — it casts on a surface of a given normal, [Sun.Shaded] the same with only part
// of the sun reaching it, a [Lamp] the sun ready to light many surfaces; [Sun.Frame] hands a
// render.Frame its shader's uniforms (shaders/sun.wgsl: Sun, SunStrength, SunColor, SkyColor, Ambience,
// sunWay()) for what glints and reflects the sky; [Sun.ShadowOf] is where the shadow an entity
// casts lies on the ground away from the sun ([Patch]), as wide as it, stretched by its height,
// pushed off by how far above the ground it stands — a hawk's falls where it flies over — for the
// ground drawn on the GPU to lay. [DefaultSun] stands high over
// the south-east, the sun of a relief under no sky.
//
// [New] takes the calendar, the [Config] — which [celestial.Way] the sun stands at noon, in how many steps
// a day it moves (none: as it goes), whether the light begins frozen and at what hour on the
// clock, which stars the night shows — and the latitude the sun goes at, the climate's zone's in
// an atmosphere. Once a tick ([Sky.Update], in the interface part of
// the plan) the sky sets its light ([Sky.Sun]) to [Config.LightAt] the hour: the
// sun — with noon in the south rising in the east and setting in the west, the whole path turned
// round to NoonWay, higher at midsummer and lower at midwinter, up all day or none at all past
// the polar circle — and once it is well below the horizon the moon, as bright as it is full, in
// the light its knob says ([Moon]: the colour and strength of its light and the tint of its face,
// carried by the atmosphere's own entity, where an effect's Alter turns it — a blood moon; none
// with [Sky.SetMoon] off). [Moonrise] is the moment the moon comes up, for rules the atmosphere
// plays the role of ([Moonrise.Full] a full one); the sun's strength rising and falling with it, and the
// colours of the sky and of the sun's light going through the day: blue by day, orange at sunrise
// and sunset, deep blue at night. The atmosphere hands that sun to whoever draws — a topography
// lights and shades its relief and the sprites on it by it, so mornings and evenings cast long
// shadows and nights are dark. The light goes on tick by tick, or, where the Config asks for steps,
// moves a step at a time — for whoever works out much anew whenever the sun moves.
//
// The light can be frozen: [Freeze] (P) stops it at the hour it stands, or lets it go with the
// calendar again; [Later] and [Earlier] (Shift+] and Shift+[) move a frozen light half an hour on
// or back. Only the light freezes — the calendar, the weather and the schedule go on — and it is
// a look at the world, like the camera's turn: it changes at once, in the tactical pause too, and
// is not saved with the game. Let go, the light is the hour's again at once.
//
// Where the sun and the moon stand is plugins/atmosphere/celestial's, from the sky's place — the
// latitude and the NoonWay; [Sky.Heavens] is its celestial.Heavens at the hour of the light, the
// sphere of the stars with them, for whoever draws the sky, and [SunColorAt] the colour of the
// sun's own light at a height, for its disc.
//
// [Sky.Reporter] adds the light to a scene's render.TelemetryRenderer. The sky behind the world
// is plugins/atmosphere's Backdrop, which needs the weather too.
package sky
