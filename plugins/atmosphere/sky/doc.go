// Package sky is the light of the day over a world: the sun and the moon of the calendar's hour,
// and the sky behind the world.
//
// [New] takes the world, the calendar, the [Config] — which [Way] the sun stands at noon, in how
// many steps a day it moves, whether the light begins frozen and at what hour — and the latitude
// the sun goes at, the climate's zone's in an atmosphere. Once a tick ([Sky.Update], in the
// interface part of the plan) the sky sets the world's light to [Config.LightAt] the hour: the
// sun — with noon in the south rising in the east and setting in the west, the whole path turned
// round to NoonWay, higher at midsummer and lower at midwinter, up all day or none at all past
// the polar circle — and once it is well below the horizon the moon, going its way behind it, as
// bright as it is full, in a paler light; the sun's strength rising and falling with it, and the
// colours of the sky and of the sun's light going through the day: blue by day, orange at sunrise
// and sunset, deep blue at night. The board and the world light and shade the ground and the
// sprites by that sun, so mornings and evenings cast long shadows and nights are dark; the light
// moves in steps so the terrain's shadows, worked out anew whenever the sun moves, are not worked
// out every tick.
//
// The light can be frozen: [Freeze] (P) stops it at the hour it stands, or lets it go with the
// calendar again; [Later] and [Earlier] (Shift+] and Shift+[) move a frozen light half an hour on
// or back. Only the light freezes — the calendar, the weather and the schedule go on — and it is
// a look at the world, like the camera's turn: it changes at once, in the tactical pause too, and
// is not saved with the game. Let go, the light is the hour's again at once.
//
// [Sky.Reporter] adds the light to a scene's render.TelemetryRenderer. [Backdrop] is the sky
// behind the world, a render.Source for a scene's Composer: the viewport in the sky's colour,
// grey under clouds, under everything, drawn only when the ground does not cover the whole screen
// — beyond the world's edge, above a low view — and, through a camera with vanishing points
// (camera.Vanisher, a perspective's), the sun's disc and glow where the way towards it vanishes,
// over the horizon, dimmed by the clouds, the hills over it.
package sky
