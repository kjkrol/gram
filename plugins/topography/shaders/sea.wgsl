// The sea: waves running with the wind, a swell coming ashore and breaking into surf.

// SeaGlint is a glint over a tile: water rippled by small waves the wind runs across it, at p,
// shine in red, lit of the sun reaching it — the clouds' shadow taken off — as the fraction and its
// shore in custom — the sky it reflects and the surf's foam laid over the tile, the sun it throws
// back added.
fn SeaGlint(p: vec2<f32>, shine: f32, lit: f32, shore: vec4<f32>) -> vec4<f32> {
    return water(shine, lit, sea(p, shore));
}

// sea is, for water at p, the sun thrown back at the eye, how much foam covers it and how much of
// the sky it reflects. Waves run across it as time goes by; near a shore (the way to it, how far
// and how near) a swell comes in facing it and breaks into foam the last stretch before it,
// whatever the sun. The flatter the eye looks at a wave, the more of the sky it reflects.
fn sea(p: vec2<f32>, shore: vec4<f32>) -> vec3<f32> {
    let t = U.Clock;
    // the waves run with the wind, the steeper the harder it blows
    let blow = length(U.Wind);
    let rough = clamp(calmSea + blow / fullWind, calmSea, stormSea);
    var way = vec2<f32>(1.0, 0.0);
    if blow > 0.01 {
        way = U.Wind / blow;
    }
    let q = vec2<f32>(way.x * p.x + way.y * p.y, way.x * p.y - way.y * p.x); // p along the wind and across it
    var slope = wave(q, vec2<f32>(1.0, 0.1), 53.0, 10.0, t);
    slope += wave(q, vec2<f32>(0.3, 1.0), 41.0, 9.0, t);
    slope += wave(q, vec2<f32>(-0.8, 0.6), 31.0, 8.0, t);
    slope += wave(q, vec2<f32>(-0.6, -0.8), 23.0, 7.0, t);
    slope += wave(q, vec2<f32>(0.7, -0.7), 17.0, 6.0, t);
    slope += wave(q, vec2<f32>(1.0, 0.8), 13.0, 5.0, t);
    slope += wave(q, vec2<f32>(-0.2, 1.0), 9.0, 4.0, t);
    slope = rough * vec2<f32>(way.x * slope.x - way.y * slope.y, way.y * slope.x + way.x * slope.y);
    var foam = 0.0;
    let towards = length(shore.xy);
    if towards > 0.01 {
        // the crests follow the shore: their phase is the distance to it, falling as they come in,
        // shifted slowly along the coast so they do not reach it everywhere at once
        let d = shore.xy / towards;
        let drift = 2.0 * pi * (sin(p.x / 173.0 + p.y / 241.0) + 0.5 * sin(p.y / 97.0 - p.x / 131.0));
        let phase = 2.0 * pi / swellLength * (shore.z + swellSpeed * t) + drift;
        var swell = d * (rough * swellSlope * cos(phase) * seen(swellLength));
        swell += d * (rough * swellSlope * 0.5 * cos(2.2 * phase + 1.0) * seen(swellLength / 2.2));
        let near = smoothstep(0.0, 1.0, clamp(shore.w, 0.0, 1.0));
        slope = mix(slope, swell + slope * 0.3, near);
        // a crest breaks the last surfWidth before the shore, torn into patches
        let crest = smoothstep(0.2, 1.0, sin(phase));
        let breaking = 1.0 - smoothstep(0.0, surfWidth, shore.z);
        let torn = 0.6 + 0.4 * sin(p.x * 0.37 + p.y * 0.23 + t) * sin(p.y * 0.41 - p.x * 0.19 - t * 0.7);
        foam = foamCover * crest * breaking * torn * clamp(rough, 0.3, 1.2);
    }
    let n = normalize(vec3<f32>(-slope.x, -slope.y, 1.0));
    let h = normalize(sunWay() + U.Toward);
    // the sky seen in the water follows only the swell of the waves, not every ripple: fine waves
    // flattened far off would stripe it
    let calm = normalize(vec3<f32>(-slope.x * mirrorSwell, -slope.y * mirrorSwell, 1.0));
    let fresnel = mirrorHead + (1.0 - mirrorHead) * pow(1.0 - max(dot(calm, U.Toward), 0.0), 5.0);
    return vec3<f32>(U.SunStrength * pow(max(dot(n, h), 0.0), glintSharpness), clamp(foam, 0.0, 1.0), fresnel);
}

// wave is the slope at p of a wave running along way, span world units from crest to crest, at
// speed world units a second.
fn wave(p: vec2<f32>, way: vec2<f32>, span: f32, speed: f32, t: f32) -> vec2<f32> {
    let d = normalize(way);
    let k = 2.0 * pi / span;
    return d * (waveSlope * cos(k * (dot(p, d) - speed * t)) * seen(span));
}

// waveSlope is how steep a wave's side gets.
const waveSlope: f32 = 0.12;

// The swell coming ashore: how steep, how long from crest to crest and how fast, in world units.
const swellSlope: f32 = 0.3;
const swellLength: f32 = 40.0;
const swellSpeed: f32 = 12.0;

// surfWidth is how far from the shore, in world units, a crest breaks; foamCover how much of the
// water its foam covers at most.
const surfWidth: f32 = 32.0;
const foamCover: f32 = 0.9;

// mirrorHead is how much of the sky water reflects looked at straight down; flatter it reflects
// more, all of it along the surface.
const mirrorHead: f32 = 0.02;

// mirrorSwell is how much of the waves' slope tilts the sky water reflects.
const mirrorSwell: f32 = 0.3;

// How rough the sea is: calm with no wind, as the waves were made at fullWind (world units a
// second), and at most stormSea times that.
const calmSea: f32 = 0.35;
const fullWind: f32 = 40.0;
const stormSea: f32 = 1.8;
