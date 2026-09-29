// The clouds' noise over the world, carried by the wind: the CPU works out the very same
// (air.Weather.Cloud), so the clouds and their shadows agree wherever either is drawn.

// cloudField is the clouds' noise over the world point p, carried by the Drift: four octaves of
// value noise over cloudSize, 0 to 1.
fn cloudField(p: vec2<f32>) -> f32 {
    let q = (p - U.Drift) / cloudSize;
    let n = cloudNoise(q) + 0.5 * cloudNoise(q * 2.03 + 17.0) + 0.25 * cloudNoise(q * 4.01 + 31.0) + 0.125 * cloudNoise(q * 8.07 + 53.0);
    return n / 1.875;
}

// cloudNoise is smooth value noise at p: 0 to 1, changing over a unit.
fn cloudNoise(p: vec2<f32>) -> f32 {
    let i = floor(p);
    var f = p - i;
    f = f * f * (3.0 - 2.0 * f);
    let a = cloudHash(i);
    let b = cloudHash(i + vec2<f32>(1.0, 0.0));
    let c = cloudHash(i + vec2<f32>(0.0, 1.0));
    let d = cloudHash(i + vec2<f32>(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

// cloudHash is a number 0 to 1 fixed for the lattice point i: a permutation polynomial mod 289 in
// whole numbers under 2²⁴, exact in floats, so the CPU (air.cloudHash) works out the very same.
fn cloudHash(i: vec2<f32>) -> f32 {
    let x = modulo(i.x + 17.0, 289.0);
    let y = modulo(i.y + 53.0, 289.0);
    let p = modulo((34.0 * x + 1.0) * x, 289.0);
    return modulo((34.0 * (p + y) + 1.0) * (p + y), 289.0) / 289.0;
}

// cloudSize is how wide a cloud is, in world units — air's, the same.
const cloudSize: f32 = 420.0;

// cloudCover is how thick the clouds are over a point whose noise is n: 0 in the clear, 1 under
// the thickest, the noise spread out and cut at the cover, as air.Weather.Shade has it.
fn cloudCover(field: f32) -> f32 {
    if U.Cover <= 0.0 {
        return 0.0;
    }
    let n = clamp((field - 0.5) * cloudContrast + 0.5, 0.0, 1.0); // the noise bunches round a half: spread it out
    return smoothstep(1.0 - U.Cover, 1.0 - U.Cover + cloudEdge, n);
}

// How far the noise is spread to make clouds and clear sky between them; over how much of the
// cover a cloud's edge softens.
const cloudContrast: f32 = 1.8;
const cloudEdge: f32 = 0.15;
