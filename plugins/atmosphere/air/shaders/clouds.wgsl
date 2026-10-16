// The clouds on the sky, as the sky draws them: every pixel looks along its own line of sight
// (U.EyeAt, U.LookDir + U.LookDX·x + U.LookDY·y, camera.RayField) up to a layer of clouds
// (U.CloudBases) and takes its noise there; U.Visibility is how far one sees through the air, 0
// without end.

// cloudsLook is how the clouds along the line of sight of the screen pixel px look on the layer
// height up, as skyClouds reads it: how hazed the layer is there, how square their undersides face
// the eye, how near the sun it looks and how much of the layer shows, none along the horizon; all
// 0 where it looks under the layer. It changes slowly: a few pixels work it out and the ones
// between have it smoothed.
fn cloudsLook(px: vec2<f32>, height: f32) -> vec4<f32> {
    let d = U.LookDir + U.LookDX * px.x + U.LookDY * px.y;
    if d.z <= 0.0 {
        return vec4<f32>(0.0);
    }
    let t = (height - U.EyeAt.z) / d.z;
    if t <= 0.0 {
        return vec4<f32>(0.0);
    }
    let view = d / length(d);
    var haze = 0.0;
    if U.Visibility > 0.0 {
        haze = 1.0 - exp(-t * length(d) / U.Visibility);
    }
    let toward = max(dot(view, sunWay()), 0.0);
    let t2 = toward * toward;
    return vec4<f32>(haze, smoothstep(0.1, 0.6, view.z), t2 * t2 * t2, smoothstep(skyLift, 3.0 * skyLift, view.z));
}

// skyClouds is the clouds of noise field under their layer's air mass m on the sky, premultiplied,
// their edge softened soft either way, looking as look says (cloudsLook), over the sky behind them:
// lit by the sky and by the sun or the moon, as much as they shine, greyer underneath the more of
// the sky they cover, their thick middles darker seen from below and their thin edges shining
// towards the light, the more so the more heaped they are, turning to the sky behind as far off as
// the layer lies in the air; whole, hiding the stars, the moon and the sun behind them.
fn skyClouds(field: f32, soft: f32, look: vec4<f32>, behind: vec3<f32>, m: vec4<f32>) -> vec4<f32> {
    let cover = cloudCoverSoft(field, soft, m);
    if cover <= 0.0 {
        return vec4<f32>(0.0);
    }
    let lit = clamp(U.SunStrength * 1.4, 0.0, 1.0);
    let light = U.Ambience * cloudsSkyLit + U.SunColor * U.SunStrength * cloudsSunLit;
    let under = mix(1.0, 0.72, clamp(m.x, 0.0, 1.0)); // the undersides greyer under a heavier sky
    let heaped = clamp(m.y, 0.0, 1.0);
    let thick = cloudThick(field, m);
    let base = 1.0 - thick * thick * (0.1 + 0.25 * heaped) * mix(0.6, 1.0, look.y);
    let rim = look.z * (1.0 - thick) * lit * (0.3 + 0.5 * heaped);
    let rgb = mix(light * under * base + U.SunColor * rim, behind, look.x);
    let a = cover * look.w;
    return vec4<f32>(rgb * a, a);
}

// How much of the sky's light and of the sun's (or the moon's) a cloud gives back: all but white
// under the noon sun.
const cloudsSkyLit: f32 = 0.6;
const cloudsSunLit: f32 = 1.1;

// skyLift is how far up a line of sight must look to see the cloud layer at all, and three times
// as far to see all of it: along the horizon the air alone shows.
const skyLift: f32 = 0.02;

// skyAt is the sky's colour looking up as high as up says, 0 at the horizon to 1 overhead, as
// air.Weather.Frame hands the frame its colours there.
fn skyAt(up: f32) -> vec3<f32> {
    return mix(U.Fog, U.SkyOverhead, sqrt(clamp(up, 0.0, 1.0)));
}

// cloudHeap is the low clouds as bodies at the world point p, z world units up: how much of a heap
// is there (x, 0 to 1) and how much of the sun reaches it through the heap over it (y), in the band
// band of them there (cloudLowBand), under the low clouds' air mass m, their noise read as a pixel
// pixel world units wide sees it. A heap's base lies flat on the band's base, its top domed as high
// as it is thick, up past the band's deck, and bulging in round lumps of two sizes creased between,
// never flat.
fn cloudHeap(p: vec2<f32>, z: f32, band: vec2<f32>, pixel: f32, m: vec4<f32>) -> vec2<f32> {
    let h = (z - band.x) / max(band.y - band.x, 1e-3);
    if h < 0.0 {
        return vec2<f32>(0.0, 1.0);
    }
    let c = cloudLowAt(p, pixel, m.y);
    let cover = cloudCoverSoft(c.x, c.y, m);
    if cover <= 0.0 {
        return vec2<f32>(0.0, 1.0);
    }
    let thick = cloudThick(c.x, m);
    let lumps = 0.6 * bodyBillow(p, bodyLump) + 0.4 * bodyBillow(p, 0.35 * bodyLump);
    // rising gently from the edge: steeper, the edges stood up as walls
    let top = pow(thick, bodyRise) * (1.0 - bodyLumps + 2.0 * bodyLumps * lumps) + 0.1 * cover;
    let there = cover * (1.0 - smoothstep(top - bodySoft, top, h));
    return vec2<f32>(there, exp(-max(top - h, 0.0) * bodyShade));
}

// CloudRay is a line of sight through the low clouds: where it enters them and leaves (enter,
// leave), and their band (cloudLowBand) where it was looked at, at from and at to, read between.
struct CloudRay {
    enter: f32,
    leave: f32,
    from: f32,
    to: f32,
    near: vec2<f32>,
    far: vec2<f32>,
}

// cloudLowRay is the line from o along d through the low clouds between t0 and t1, their whole
// span, narrowed down to the bands at its two ends, when they lie near enough together for the
// bands between to be like theirs: a thin deck takes all the steps.
fn cloudLowRay(o: vec3<f32>, d: vec3<f32>, t0: f32, t1: f32) -> CloudRay {
    let near = cloudLowBand(o.xy + d.xy * t0);
    var r = CloudRay(t0, t1, t0, t1, near, cloudLowBand(o.xy + d.xy * t1));
    if length(d.xy) * abs(t1 - t0) > 0.5 * cloudCeilingWide || abs(d.z) < 1e-6 {
        return r;
    }
    let ta = (min(r.near.x, r.far.x) - o.z) / d.z;
    let tb = (max(cloudLumpsTop(r.near), cloudLumpsTop(r.far)) - o.z) / d.z;
    r.enter = max(t0, min(ta, tb));
    r.leave = min(t1, max(ta, tb));
    return r;
}

// cloudRayBand is the low clouds' band t along the line of sight r.
fn cloudRayBand(r: CloudRay, t: f32) -> vec2<f32> {
    return mix(r.near, r.far, clamp((t - r.from) / max(r.to - r.from, 1e-6), 0.0, 1.0));
}

// cloudLumpsTop is how high the heaps' lumps rise over a band of them (cloudLowBand).
fn cloudLumpsTop(band: vec2<f32>) -> f32 {
    return band.x + bodyLowRise * (band.y - band.x);
}

// bodyBillow is round bulges creased between at p, wide clouds' widths apart, 0 to 1: the noise
// folded at its middle, the folds the creases (the other way round, they stood up as peaks).
fn bodyBillow(p: vec2<f32>, wide: f32) -> f32 {
    return abs(2.0 * cloudNoise(p / (wide * cloudSize), 4096.0) - 1.0);
}

// cloudBodySpan is how high a layer of clouds reaches, world units, from (x) up to (y): the
// heaps' bodies if low (cloudLowSpan), else the band of the shreds' sheets.
fn cloudBodySpan(low: bool) -> vec2<f32> {
    if low {
        return cloudLowSpan();
    }
    return vec2<f32>(cloudHighFloor(), U.CloudBases.y);
}

// How deep the heaps' deck is without a relief, in clouds' widths, and how much higher than their
// deck their lumps rise; how wide a lump on a top is, in clouds' widths, and how far it bulges
// either way; over how much of the deck a top rounds off; how quickly the cloud over a point takes
// the sun; how a top rises with how thick the cloud is.
const bodyLowDeep: f32 = 0.45;
const bodyLowRise: f32 = 1.6;
const bodyLump: f32 = 0.25;
const bodyLumps: f32 = 0.4;
const bodySoft: f32 = 0.07;
const bodyShade: f32 = 1.2;
const bodyRise: f32 = 0.8;
