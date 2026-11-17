// The sea: waves carried by the wind and steeper under its gusts, a swell coming ashore and
// breaking into surf.

// SeaGlint is a glint over a tile: water rippled by small waves the wind runs across it, at p,
// shine in red, lit of the sun reaching it — the clouds' shadow taken off — as the fraction and its
// shore in custom: the water's own colour under it shaded wave by wave, the sky it reflects and the
// surf's foam laid over it, the sun it throws back added.
fn SeaGlint(p: vec2<f32>, shine: f32, lit: f32, shore: vec4<f32>) -> vec4<f32> {
    return SeaGlintAt(p, shine, lit, shore, U.Pixel, U.Toward, gustAt(p, U.Pixel));
}

// SeaGlintAt is SeaGlint where a pixel spans pixel world units, the eye lying toward, under a
// gust blowing as hard as gust (gustAt): waves finer than a pixel or two there are left out,
// lest they flicker.
fn SeaGlintAt(p: vec2<f32>, shine: f32, lit: f32, shore: vec4<f32>, pixel: f32, toward: vec3<f32>,
    gust: f32) -> vec4<f32> {
    let w = sea(p, shore, pixel, gust);
    let n = w.normal;
    let sun = lightWay();
    // a wave's side turned from the light is darker and the side turned to it catches it, whatever
    // the sun: the light from above and towards the sun, its way across the sea
    let toLight = normalize(sun + vec3<f32>(0.0, 0.0, 1.0));
    let across = normalize(vec2<f32>(toLight.x, toLight.y) + vec2<f32>(1e-4, 0.0));
    let tilt = dot(n.xy, across) / max(n.z, 1e-3); // how steeply it faces the light
    let shade = 1.0 - waveShade * shine * clamp(-tilt * waveTilt, 0.0, 1.0);
    let catch = waveCatch * shine * clamp(tilt * waveTilt, 0.0, 1.0);
    // the sky it reflects: the deeper overhead, the paler towards the horizon; the more the flatter
    // the eye meets the wave — the waves tilting it as far as mirrorWave says, lest the sides
    // turned from the eye, met all but along them, give back nothing but the sky
    let m = normalize(vec3<f32>(n.xy / max(n.z, 1e-3) * mirrorWave, 1.0));
    let r = reflect(-toward, m);
    let sky = skyAt(r.z);
    let mirror = shine * (mirrorHead + (1.0 - mirrorHead) * pow(1.0 - max(dot(m, toward), 0.0), 5.0));
    // the sun thrown back: a broad sheen and the glints on the steepest ripples
    let h = normalize(sun + toward);
    let nh = max(dot(n, h), 0.0);
    let glint = U.LightStrength * lit * shine * (sheen * pow(nh, sheenSharpness) + seaGlint * pow(nh, seaGlintSharpness));
    let light = U.Ambience + U.LightColor * (U.LightStrength * lit * max(sun.z, 0.0));
    let foam = w.foam;
    let rgb = sky * mirror * (1.0 - foam) + light * foam + U.LightColor * glint + light * catch * (1.0 - foam);
    return vec4<f32>(rgb, 1.0 - clamp(shade, 0.0, 1.0) * (1.0 - mirror) * (1.0 - foam));
}

// Sea is the sea's surface at a point: the way it faces and how much foam covers it.
struct Sea {
    normal: vec3<f32>,
    foam: f32,
}

// sea is the sea's surface at p, a pixel spanning pixel world units there, under a gust blowing as
// hard as gust. Waves run across it with the wind; near a shore (the way to it, how far and how
// near) a swell comes in facing it and breaks into foam the last stretch before it, whatever the
// sun. What is finer than a pixel or two is left at its mean: far off the surf is an even pale
// line along the shore.
fn sea(p: vec2<f32>, shore: vec4<f32>, pixel: f32, gust: f32) -> Sea {
    let t = U.Clock;
    // the steeper the harder the wind blows, and under a gust: its dark patch going over the water
    let rough = clamp(calmSea + length(U.Wind) / fullWind, calmSea, stormSea)
        * (1.0 + gustRough * gust);
    var slope = chop(p, 47.0, 9.0, 0.35, vec2<f32>(0.0, 0.0), t, pixel);
    slope += chop(p, 27.0, 7.0, -0.45, vec2<f32>(17.3, 5.1), t, pixel);
    slope += chop(p, 15.0, 5.5, 0.6, vec2<f32>(3.7, 23.9), t, pixel);
    slope += chop(p, 8.5, 4.0, -0.2, vec2<f32>(41.2, 11.6), t, pixel);
    slope += chop(p, 4.5, 3.0, 0.9, vec2<f32>(29.5, 37.1), t, pixel);
    // far off, as the fine waves go out of sight, the long swells of the open sea come in: two
    // trains of them, wandering a little
    let far = 1.0 - seenAt(farFrom, pixel);
    if far > 0.0 {
        let wander = 2.0 * sin(p.x * 0.011 - p.y * 0.007 + t * 0.05)
            + 1.1 * sin(p.x * 0.004 + p.y * 0.013 - t * 0.03);
        let a = 2.0 * pi / longSwell * (dot(p, longWay) - 17.0 * t) + wander;
        let b = pi / longSwell * (dot(p, longWay.yx) - 24.0 * t) - 2.0 * wander;
        slope += (longWay * cos(a) * seenAt(longSwell, pixel)
            + longWay.yx * cos(b) * seenAt(2.0 * longSwell, pixel)) * (far * longSlope);
    }
    slope *= rough;
    var foam = 0.0;
    let towards = length(shore.xy);
    if towards > 0.01 {
        // the crests follow the shore: their phase is the distance to it, falling as they come in,
        // shifted slowly along the coast so they do not reach it everywhere at once
        let d = shore.xy / towards;
        let drift = 2.0 * pi * (sin(p.x / 173.0 + p.y / 241.0) + 0.5 * sin(p.y / 97.0 - p.x / 131.0));
        let phase = 2.0 * pi / swellLength * (shore.z + swellSpeed * t) + drift;
        var swell = d * (rough * swellSlope * cos(phase) * seenAt(swellLength, pixel));
        swell += d * (rough * swellSlope * 0.5 * cos(2.2 * phase + 1.0) * seenAt(swellLength / 2.2, pixel));
        let near = smoothstep(0.0, 1.0, clamp(shore.w, 0.0, 1.0));
        slope = mix(slope, swell + slope * 0.3, near);
        // a crest breaks the last surfWidth before the shore, torn into patches
        let crest = mix(crestMean, smoothstep(0.2, 1.0, sin(phase)), seenAt(swellLength, pixel));
        let breaking = 1.0 - smoothstep(0.0, surfWidth, shore.z);
        let patches = sin(p.x * 0.37 + p.y * 0.23 + t) + sin(p.y * 0.29 - p.x * 0.11 - t * 0.7)
            + sin(p.x * 0.17 - p.y * 0.41 + t * 0.4);
        let torn = 0.6 + 0.4 / 3.0 * patches * seenAt(tornSize, pixel);
        foam = foamCover * crest * breaking * torn * clamp(rough, 0.3, 1.2)
            * seenAt(surfWidth, pixel);
    }
    return Sea(normalize(vec3<f32>(-slope.x, -slope.y, 1.0)), clamp(foam, 0.0, 1.0));
}

// chop is the slope at q of waves about span world units across, noise carried the way the wind
// has blown (U.Blown), speed world units a second in a wind of seaWind, and a little along x
// besides, so a sea in still air still breathes; turned by turn radians off x and stretched across
// it so the crests run long — each size of them another noise, apart by at: never the same twice,
// as regular waves would be; none where a pixel spans too much of them. Carried, not turned with
// the wind: turned, every wander of it would swing the whole sea about the world's corner.
fn chop(q: vec2<f32>, span: f32, speed: f32, turn: f32, at: vec2<f32>, t: f32, pixel: f32) -> vec2<f32> {
    let c = cos(turn);
    let s = sin(turn);
    let m = q - U.Blown * (speed / seaWind) - vec2<f32>(speed * seaIdle * t, 0.0);
    let r = vec2<f32>(c * m.x + s * m.y, -s * m.x + c * m.y); // turned off the wind
    let seen = seenAt(span, pixel);
    if seen <= 0.0 {
        return vec2<f32>(0.0);
    }
    let k = vec2<f32>(1.0, chopStretch) / span;
    let g = noised(r * k + at).yz * k; // the slope in the turned frame, per world unit
    return vec2<f32>(c * g.x - s * g.y, s * g.x + c * g.y) * (chopHeight * span * seen);
}

// chopHeight is how high the waves stand for their span; chopStretch how much longer than across
// the wind they run along their crests; farFrom the span of the waves whose going out of sight
// brings the long swells in, longSwell world units from crest to crest and twice that, running
// longWay and across it, as steep as longSlope.
const chopHeight: f32 = 0.12;
const chopStretch: f32 = 0.45;
const farFrom: f32 = 15.0;
const longSwell: f32 = 95.0;
const longWay: vec2<f32> = vec2<f32>(0.96, 0.28);
const longSlope: f32 = 0.055;

// The swell coming ashore: how steep, how long from crest to crest and how fast, in world units.
const swellSlope: f32 = 0.3;
const swellLength: f32 = 40.0;
const swellSpeed: f32 = 12.0;

// surfWidth is how far from the shore, in world units, a crest breaks; foamCover how much of the
// water its foam covers at most; crestMean how much of a swell is its crest, tornSize how wide
// the patches its foam is torn into are.
const surfWidth: f32 = 32.0;
const foamCover: f32 = 0.9;
const crestMean: f32 = 0.26;
const tornSize: f32 = 16.0;

// mirrorHead is how much of the sky the sea reflects looked at straight down; flatter it reflects
// more, all of it along the surface.
const mirrorHead: f32 = 0.02;

// mirrorWave is how much of the waves' slope tilts the sky the sea reflects.
const mirrorWave: f32 = 0.5;

// How much darker a wave's side turned from the light is at most and how much of the light the
// side turned to it catches, as steep as waveTilt says; how broad the sun's sheen on the sea is,
// and how bright.
const waveShade: f32 = 0.4;
const waveCatch: f32 = 0.07;
const waveTilt: f32 = 8.0;
const sheenSharpness: f32 = 150.0;
const sheen: f32 = 0.04;

// How narrowly the sun's glints on the sea gather round the perfect reflection, and how bright
// they are: narrower than a stream's, the sea's wide smooth swells would throw back the sun whole.
const seaGlintSharpness: f32 = 1200.0;
const seaGlint: f32 = 0.8;

// How rough the sea is: calm with no wind, as the waves were made at fullWind (world units a
// second), and at most stormSea times that; under a gust's middle gustRough rougher again.
const calmSea: f32 = 0.35;
const fullWind: f32 = 40.0;
const stormSea: f32 = 1.8;
const gustRough: f32 = 0.7;

// seaWind is the wind, world units a second, in which the waves run at the speeds they were made
// with; seaIdle the share of those they run along x whatever the wind.
const seaWind: f32 = 20.0;
const seaIdle: f32 = 0.25;
