// Package sky is the light of the day over a world: the [Sun] and the moon of the calendar's hour.
//
// A [Sun] is a direction towards it, a strength, how much of the sky's light every surface gets
// anyway, and the colours of its light and of the sky (white when zero); [Sun.Light] is the light
// — a render.Light — it casts on a surface of a given normal, [Sun.Shaded] the same with only part
// of the sun reaching it, a [Lamp] the sun ready to light many surfaces; [Sun.Frame] hands a
// render.Frame its shader's uniforms (sun.kage: Sun, SunStrength, SunColor, SkyColor, Ambience,
// sunWay()) for what glints and reflects the sky; [Sun.Shadow] lays the shadow an entity casts on
// the ground away from the sun, as wide as it, stretched by its height, pushed off by how far
// above the ground it stands — a hawk's falls where it flies over. [DefaultSun] stands high over
// the south-east, the sun of a relief under no sky.
//
// [New] takes the calendar, the [Config] — which [Way] the sun stands at noon, in how many steps
// a day it moves, whether the light begins frozen and at what hour — and the latitude the sun goes
// at, the climate's zone's in an atmosphere. Once a tick ([Sky.Update], in the interface part of
// the plan) the sky sets its light ([Sky.Sun]) to [Config.LightAt] the hour: the
// sun — with noon in the south rising in the east and setting in the west, the whole path turned
// round to NoonWay, higher at midsummer and lower at midwinter, up all day or none at all past
// the polar circle — and once it is well below the horizon the moon, going its way behind it, as
// bright as it is full, in a paler light; the sun's strength rising and falling with it, and the
// colours of the sky and of the sun's light going through the day: blue by day, orange at sunrise
// and sunset, deep blue at night. The atmosphere hands that sun to whoever draws — a topography
// lights and shades its relief and the sprites on it by it, so mornings and evenings cast long
// shadows and nights are dark; the light moves in steps so the terrain's shadows, worked out anew
// whenever the sun moves, are not worked out every tick.
//
// The light can be frozen: [Freeze] (P) stops it at the hour it stands, or lets it go with the
// calendar again; [Later] and [Earlier] (Shift+] and Shift+[) move a frozen light half an hour on
// or back. Only the light freezes — the calendar, the weather and the schedule go on — and it is
// a look at the world, like the camera's turn: it changes at once, in the tactical pause too, and
// is not saved with the game. Let go, the light is the hour's again at once.
//
// [Sky.Reporter] adds the light to a scene's render.TelemetryRenderer. The sky behind the world
// is plugins/atmosphere's Backdrop, which needs the weather too.
package sky
