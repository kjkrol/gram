// What a part of the sky is handed, the contract a game's own part (backdrop.Part) is written to:
// its entry is fn SkyVault(s: SkyAt) -> vec3<f32>, fn SkyStars(s: SkyAt) -> vec3<f32>,
// fn SkyMoon(s: SkyAt, d: SkyDisc) -> SkyShine or fn SkySun(s: SkyAt, d: SkyDisc) -> SkyShine.
// gogpu's compiler (naga v0.20.3 too) leaves a var without an initialiser undefined: start from
// SkyShine().

// SkyAt is the sky at a pixel: the pixel px, the unit line of sight way (z up), the radians a
// pixel spans there, the way among the stars (equatorial, x towards 0h, z the pole), the sky's
// colours at the horizon and overhead, the way to the sun (below the horizon too) and its own
// colour, how much the stars show (the night while they are shown) and the sky's colour here: the
// air's for the vault, the vault's for the rest.
struct SkyAt {
    px: vec2<f32>,
    way: vec3<f32>,
    pixel: f32,
    starWay: vec3<f32>,
    horizon: vec3<f32>,
    overhead: vec3<f32>,
    sun: vec3<f32>,
    sunColor: vec3<f32>,
    stars: f32,
    air: vec3<f32>,
}

// SkyDisc is a disc on the sky: its middle on the screen and its radius in pixels, its colour,
// how much of it shows (0 to 1, the clouds and the day taking it), the colour of its glow, the way
// to the light on it (the moon's face: x right, y down, z towards the eye) and the way north on the
// screen.
struct SkyDisc {
    at: vec2<f32>,
    radius: f32,
    color: vec3<f32>,
    shown: f32,
    glow: vec3<f32>,
    lit: vec3<f32>,
    north: vec2<f32>,
}

// SkyShine is what a disc gives the sky: the light its glow adds under whatever lies over it, and
// what it lays over what came before, premultiplied.
struct SkyShine {
    glow: vec3<f32>,
    front: vec4<f32>,
}
