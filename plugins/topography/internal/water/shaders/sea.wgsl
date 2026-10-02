// The sea: waves running with the wind, a swell coming ashore and breaking into surf.

// SeaGlint is a glint over a tile: water rippled by small waves the wind runs across it, at p,
// shine in red, lit of the sun reaching it — the clouds' shadow taken off — as the fraction and its
// shore in custom: the water's own colour under it shaded wave by wave, the sky it reflects and the
// surf's foam laid over it, the sun it throws back added.
fn SeaGlint(p: vec2<f32>, shine: f32, lit: f32, shore: vec4<f32>) -> vec4<f32> {
    return SeaGlintAt(p, shine, lit, shore, U.Pixel, U.Toward);
}

// SeaGlintAt is SeaGlint where a pixel spans pixel world units, the eye lying toward: waves finer
// than a pixel or two there are left out, lest they flicker.
fn SeaGlintAt(p: vec2<f32>, shine: f32, lit: f32, shore: vec4<f32>, pixel: f32, toward: vec3<f32>) -> vec4<f32> {
    let w = sea(p, shore, pixel);
    let n = w.normal;
    let sun = sunWay();
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
    let sky = overcastSky() * mix(skyAtHorizon, skyOverhead, clamp(r.z, 0.0, 1.0));
    let mirror = shine * (mirrorHead + (1.0 - mirrorHead) * pow(1.0 - max(dot(m, toward), 0.0), 5.0));
    // the sun thrown back: a broad sheen and the glints on the steepest ripples
    let h = normalize(sun + toward);
    let nh = max(dot(n, h), 0.0);
    let glint = U.SunStrength * lit * shine * (sheen * pow(nh, sheenSharpness) + seaGlint * pow(nh, seaGlintSharpness));
    let light = U.Ambience + U.SunColor * (U.SunStrength * lit * max(sun.z, 0.0));
    let foam = w.foam;
    let rgb = sky * mirror * (1.0 - foam) + light * foam + U.SunColor * glint + light * catch * (1.0 - foam);
    return vec4<f32>(rgb, 1.0 - clamp(shade, 0.0, 1.0) * (1.0 - mirror) * (1.0 - foam));
}

// Sea is the sea's surface at a point: the way it faces and how much foam covers it.
struct Sea {
    normal: vec3<f32>,
    foam: f32,
}

// sea is the sea's surface at p, a pixel spanning pixel world units there. Waves run across it as
// time goes by; near a shore (the way to it, how far and how near) a swell comes in facing it and
// breaks into foam the last stretch before it, whatever the sun.
fn sea(p: vec2<f32>, shore: vec4<f32>, pixel: f32) -> Sea {
    let t = U.Clock;
    // the waves run along x, the steeper the harder the wind blows: turned with the wind, every
    // wander of it would swing the whole sea about the world's corner, to and fro
    let rough = clamp(calmSea + length(U.Wind) / fullWind, calmSea, stormSea);
    var slope = chop(p, 47.0, 9.0, 0.35, vec2<f32>(0.0, 0.0), t, pixel);
    slope += chop(p, 27.0, 7.0, -0.45, vec2<f32>(17.3, 5.1), t, pixel);
    slope += chop(p, 15.0, 5.5, 0.6, vec2<f32>(3.7, 23.9), t, pixel);
    slope += chop(p, 8.5, 4.0, -0.2, vec2<f32>(41.2, 11.6), t, pixel);
    slope += chop(p, 4.5, 3.0, 0.9, vec2<f32>(29.5, 37.1), t, pixel);
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
        let crest = smoothstep(0.2, 1.0, sin(phase));
        let breaking = 1.0 - smoothstep(0.0, surfWidth, shore.z);
        let torn = 0.6 + 0.4 * sin(p.x * 0.37 + p.y * 0.23 + t) * sin(p.y * 0.41 - p.x * 0.19 - t * 0.7);
        foam = foamCover * crest * breaking * torn * clamp(rough, 0.3, 1.2);
    }
    return Sea(normalize(vec3<f32>(-slope.x, -slope.y, 1.0)), clamp(foam, 0.0, 1.0));
}

// chop is the slope at q of waves about span world units across, noise carried along x at speed
// world units a second, turned by turn radians off it and stretched across it so the crests run long — each size of them another noise, apart by at:
// never the same twice, as regular waves would be; none where a pixel spans too much of them.
fn chop(q: vec2<f32>, span: f32, speed: f32, turn: f32, at: vec2<f32>, t: f32, pixel: f32) -> vec2<f32> {
    let c = cos(turn);
    let s = sin(turn);
    let m = q - vec2<f32>(speed * t, 0.0);
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
// the wind they run along their crests.
const chopHeight: f32 = 0.12;
const chopStretch: f32 = 0.45;

// The swell coming ashore: how steep, how long from crest to crest and how fast, in world units.
const swellSlope: f32 = 0.3;
const swellLength: f32 = 40.0;
const swellSpeed: f32 = 12.0;

// surfWidth is how far from the shore, in world units, a crest breaks; foamCover how much of the
// water its foam covers at most.
const surfWidth: f32 = 32.0;
const foamCover: f32 = 0.9;

// mirrorHead is how much of the sky the sea reflects looked at straight down; flatter it reflects
// more, all of it along the surface.
const mirrorHead: f32 = 0.02;

// mirrorWave is how much of the waves' slope tilts the sky the sea reflects.
const mirrorWave: f32 = 0.5;

// How much paler than the sky's colour the sky reflected is at the horizon and how much deeper
// overhead; how much darker a wave's side turned from the light is at most and how much of the
// light the side turned to it catches, as steep as waveTilt says; how broad the sun's sheen on the
// sea is, and how bright.
const skyAtHorizon: f32 = 1.15;
const skyOverhead: f32 = 0.7;
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
// second), and at most stormSea times that.
const calmSea: f32 = 0.35;
const fullWind: f32 = 40.0;
const stormSea: f32 = 1.8;
