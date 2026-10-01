// Water running down its slope: ripples and flecks of foam carried by the current.

// RunningWater is water running over what it lies on at p: shine in red, lit of the sun reaching it
// as the fraction, how fast it runs in custom's x and y and, over a blended sprite, its weight and
// mark in z and w, so it shows only where the sprite does.
fn RunningWater(p: vec2<f32>, shine: f32, lit: f32, custom: vec4<f32>) -> vec4<f32> {
    var c = water(shine, lit, stream(p, custom.xy));
    if custom.w > 50000.0 {
        c *= blended(custom.z, custom.w);
    }
    return c;
}

// stream is, for water at p running at v, what sea is for still water. Its ripples and flecks are
// noise in the world carried down the current — a flow map: sampled twice, each time the current
// has carried it a fraction of flowPeriod on, the two half a period apart and crossfaded, so the
// pattern flows on for ever without stretching, and runs on without a seam from one piece of a
// river to the next. The faster it runs the rougher it is, flecked with foam and at last white.
fn stream(p: vec2<f32>, v: vec2<f32>) -> vec3<f32> {
    let t = U.Clock;
    let speed = length(v);
    let a = fract(t / flowPeriod);
    let b = fract(t / flowPeriod + 0.5);
    let wa = 1.0 - abs(2.0 * a - 1.0); // a's weight: none as it starts over, all halfway
    let pa = p - v * (a * flowPeriod);
    let pb = p - v * (b * flowPeriod) + vec2<f32>(31.7, 17.3);
    let ra = ripples(pa);
    let rb = ripples(pb);
    let rough = clamp(streamCalm + speed / fastWater, streamCalm, streamRough);
    let slope = (ra.slope * wa + rb.slope * (1.0 - wa)) * rough;
    let n = ra.height * wa + rb.height * (1.0 - wa);
    let white = smoothstep(whiteFrom, whiteFull, speed);
    let fleck = smoothstep(0.6, 0.85, n) * smoothstep(fleckFrom, fleckFull, speed) * fleckCover;
    let foam = max(white * (0.55 + 0.45 * n), fleck);
    let nrm = normalize(vec3<f32>(-slope.x, -slope.y, 1.0));
    let h = normalize(sunWay() + U.Toward);
    let fresnel = streamMirror + (1.0 - streamMirror) * pow(1.0 - max(dot(nrm, U.Toward), 0.0), 3.0);
    return vec3<f32>(U.SunStrength * pow(max(dot(nrm, h), 0.0), glintSharpness), clamp(foam, 0.0, 1.0), fresnel);
}

// Ripple is running water's ripples at a point: their height, 0 to 1, and their slope.
struct Ripple {
    height: f32,
    slope: vec2<f32>,
}

// ripples is the ripples at p: two sizes of noise, the finer fading out where it would flicker,
// their slope the noise's own gradient.
fn ripples(p: vec2<f32>) -> Ripple {
    let fine = 0.5 * seen(rippleSize / 2.0);
    let coarse = noised(p / rippleSize);
    let finer = noised(p * 2.0 / rippleSize + 7.0);
    let slope = (coarse.yz + fine * 2.0 * finer.yz) / rippleSize;
    return Ripple((coarse.x + fine * finer.x) / 1.5, slope * (rippleSlope * seen(rippleSize)));
}

// Running water: how rough it is still and at most, the speed (world units a second) it gets as
// rough as fastWater says, and the speeds it begins to foam at and is white from.
const streamCalm: f32 = 0.4;
const streamRough: f32 = 1.6;
const fastWater: f32 = 40.0;
const whiteFrom: f32 = 30.0;
const whiteFull: f32 = 55.0;

// Running water's ripples: how wide the coarser are and how steep, in world units; how long the
// current carries a pattern before it starts over, in seconds; how much of the water flecks of
// foam cover at most, and the speeds they begin to show at and show fully from; how much of the
// sky it reflects looked at straight down.
const rippleSize: f32 = 7.0;
const rippleSlope: f32 = 6.0;
const flowPeriod: f32 = 1.2;
const fleckCover: f32 = 0.35;
const fleckFrom: f32 = 3.0;
const fleckFull: f32 = 15.0;
const streamMirror: f32 = 0.08;
