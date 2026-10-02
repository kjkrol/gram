// The clouds' noise over the world, carried by the wind: the CPU works out the very same
// (air.Weather.Cloud), so the clouds and their shadows agree wherever either is drawn.

// cloudField is the clouds' noise over the world point p, carried by the Drift, 0 to 1: torn
// shreds about cloudSize wide gathered into big heaps far apart as much as U.Billow says
// (cloudMix); the same again every heapTile heaps' cells.
fn cloudField(p: vec2<f32>) -> f32 {
    let q = p - U.Drift;
    let torn = shreds(q / cloudSize);
    if U.Billow <= 0.0 {
        return torn;
    }
    return cloudMix(torn, heaps(heapCores(q / heapSize), torn));
}

// cloudFromTile is the clouds' noise of shreds torn and heaps' cores core as their tile holds them
// (air.BakeTile: the cores from heapCoreLow over heapCoreSpan).
fn cloudFromTile(torn: f32, core: f32) -> f32 {
    if U.Billow <= 0.0 {
        return torn;
    }
    return cloudMix(torn, heaps(heapCoreLow + core * heapCoreSpan, torn));
}

// cloudMix is the clouds' noise of shreds torn and heaps heap mixed as U.Billow says, spread
// apart as either alone.
fn cloudMix(torn: f32, heap: f32) -> f32 {
    let b = clamp(U.Billow, 0.0, 1.0);
    if b <= 0.0 {
        return torn;
    }
    let n = (1.0 - b) * torn + b * heap;
    return 0.5 + (n - 0.5) / sqrt((1.0 - b) * (1.0 - b) + b * b + heapTorn * b * (1.0 - b));
}

// shreds is torn clouds at q, in clouds' widths: four octaves of value noise, 0 to 1.
fn shreds(q: vec2<f32>) -> f32 {
    let n = cloudNoise(q, cloudTile) + 0.5 * cloudNoise(2.0 * q + vec2<f32>(17.37, 17.61), 2.0 * cloudTile) +
        0.25 * cloudNoise(4.0 * q + vec2<f32>(31.29, 31.83), 4.0 * cloudTile) + 0.125 * cloudNoise(8.0 * q + vec2<f32>(53.51, 53.17), 8.0 * cloudTile);
    return n / 1.875;
}

// heapCores is how high the heaps' cores stand at q, in heaps' cells, as air.heapCores has it: in
// each cell one heap kept well in from its edges, its size and the cover it forms under thrown by
// the cell, its top flat and whole, the highest over the point, round lobes bulging from its edge;
// the same again every heapTile cells.
fn heapCores(q: vec2<f32>) -> f32 {
    let c = floor(q);
    var top = -10.0;
    var inside = 0.0;
    for (var j = -1; j <= 1; j++) {
        for (var i = -1; i <= 1; i++) {
            let at = c + vec2<f32>(f32(i), f32(j));
            let k = cloudThrow(modulo2(at, heapTile)); // one throw a cell, turned four ways
            let d = at + 0.5 + heapStray * (vec2<f32>(k, modulo(37.0 * k, 289.0)) / 289.0 - 0.5) - q;
            let r = heapLeast + heapMore * modulo(53.0 * k, 289.0) / 289.0;
            let into = min((1.0 - length(d) / r) / heapFlat, 1.0);
            let t = 1.0 - heapReady * modulo(71.0 * k, 289.0) / 289.0 + heapSlope * (into - 1.0);
            if t > top {
                top = t;
                inside = into;
            }
        }
    }
    let lobes = cloudNoise(4.0 * q + vec2<f32>(0.37, 0.61), 4.0 * heapTile) + 0.5 * cloudNoise(8.0 * q + vec2<f32>(0.53, 0.29), 8.0 * heapTile) - 0.75;
    return top + heapLobes * lobes * clamp(1.0 - inside, 0.0, 1.0);
}

// heaps is billowing clouds of cores core (heapCores) over shreds torn there, 0 to 1: the shreds
// fraying their edges, and under a heavy cover the shreds between them.
fn heaps(core: f32, torn: f32) -> f32 {
    return max(0.5 + (core + heapBumps * (torn - 0.5) - 0.5) / cloudContrast, torn - heapFloor);
}

// cloudNoise is smooth value noise at p: 0 to 1, changing over a unit, the same again every
// period units.
fn cloudNoise(p: vec2<f32>, period: f32) -> f32 {
    let i = floor(p);
    var f = p - i;
    f = f * f * (3.0 - 2.0 * f);
    let i0 = modulo2(i, period);
    let i1 = modulo2(i + 1.0, period);
    let a = cloudHash(i0);
    let b = cloudHash(vec2<f32>(i1.x, i0.y));
    let c = cloudHash(vec2<f32>(i0.x, i1.y));
    let d = cloudHash(i1);
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

// cloudHash is a number 0 to 1 fixed for the lattice point i (cloudThrow's, over 289).
fn cloudHash(i: vec2<f32>) -> f32 {
    return cloudThrow(i) / 289.0;
}

// cloudThrow is a whole number under 289 fixed for the lattice point i: a permutation polynomial
// mod 289 in whole numbers under 2²⁴, exact in floats, so the CPU (air.cloudThrow) works out the
// very same.
fn cloudThrow(i: vec2<f32>) -> f32 {
    let x = modulo(i.x + 17.0, 289.0);
    let y = modulo(i.y + 53.0, 289.0);
    let p = modulo((34.0 * x + 1.0) * x, 289.0);
    return modulo((34.0 * (p + y) + 1.0) * (p + y), 289.0);
}

// cloudSize is how wide a cloud is, in world units, cloudTile clouds' widths a tile of the shreds;
// the heaps', as air has them; the part of the heaps' cores their tile holds (heapCoreLow up over
// heapCoreSpan, what lies under it no heap).
const cloudSize: f32 = 420.0;
const cloudTile: f32 = 30.0;
const heapSize: f32 = 3150.0;
const heapTile: f32 = 16.0;
const heapStray: f32 = 0.8;
const heapLeast: f32 = 0.25;
const heapMore: f32 = 0.2;
const heapFlat: f32 = 0.25;
const heapReady: f32 = 0.6;
const heapSlope: f32 = 0.6;
const heapLobes: f32 = 1.2;
const heapBumps: f32 = 0.3;
const heapFloor: f32 = 0.2;
const heapTorn: f32 = 0.4;
const heapCoreLow: f32 = -0.6;
const heapCoreSpan: f32 = 1.6;

// cloudTileLevel is the level of a tile of the clouds (air.BakeTile) period world units wide that a
// pixel pixel world units wide sees it at: a fraction between two.
fn cloudTileLevel(pixel: f32, period: f32) -> f32 {
    return clamp(log2(max(pixel * cloudTileTexels / period, 1.0)), 0.0, cloudTileLevels - 1.0);
}

// cloudTileSpot is where in the clouds' tile (air.BakeTile) of period world units the world point
// p lies on level level, a whole number, in the tile's pixels from its corner: the tile wrapping
// round, held half a pixel in from the level's edge. The shreds' period is cloudTile clouds'
// widths (red), the heaps' cores' heapTile heaps' cells (green).
fn cloudTileSpot(p: vec2<f32>, level: f32, period: f32) -> vec2<f32> {
    let size = cloudTileTexels / exp2(level);
    var origin = vec2<f32>(0.0);
    if level > 0.0 {
        origin = vec2<f32>(cloudTileTexels, cloudTileTexels - 2.0 * size);
    }
    let q = fract((p - U.Drift) / period);
    return origin + clamp(q * size, vec2<f32>(0.5), vec2<f32>(size - 0.5));
}

// The clouds' tile: how many pixels across its first level, how many levels, and how much the
// clouds' edge softens for each level their noise is seen at, its detail evened out.
const cloudTileTexels: f32 = 1024.0;
const cloudTileLevels: f32 = 6.0;
const cloudTileSoft: f32 = 0.03;

// cloudCover is how thick the clouds are over a point whose noise is n: 0 in the clear, 1 under
// the thickest, the noise spread out and cut at the cover, heaps' edges sharper than shreds', as
// air.Weather.Shade has it.
fn cloudCover(field: f32) -> f32 {
    return cloudCoverSoft(field, 0.0);
}

// cloudCoverSoft is cloudCover with the edge softened soft either way, for noise seen from afar,
// its detail evened out.
fn cloudCoverSoft(field: f32, soft: f32) -> f32 {
    if U.Cover <= 0.0 {
        return 0.0;
    }
    let n = clamp((field - 0.5) * cloudContrast + 0.5, 0.0, 1.0); // the noise bunches round a half: spread it out
    let edge = mix(cloudEdge, heapEdge, clamp(U.Billow, 0.0, 1.0));
    return smoothstep(1.0 - U.Cover - soft, 1.0 - U.Cover + edge + soft, n);
}

// cloudThick is how deep in a cloud a point whose noise is field lies: 0 at its edge, 1 as deep as
// cloudDepth and more, where the sun gets least through.
fn cloudThick(field: f32) -> f32 {
    let n = clamp((field - 0.5) * cloudContrast + 0.5, 0.0, 1.0);
    return clamp((n - (1.0 - U.Cover)) / cloudDepth, 0.0, 1.0);
}

const cloudDepth: f32 = 0.6;

// How far the noise is spread to make clouds and clear sky between them; over how much of the
// cover a cloud's edge softens, a shred's and a heap's.
const cloudContrast: f32 = 1.8;
const cloudEdge: f32 = 0.15;
const heapEdge: f32 = 0.07;
