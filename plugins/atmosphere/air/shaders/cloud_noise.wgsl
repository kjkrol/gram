// The clouds' noise over the world, carried by the wind, baked into a tile the shaders look up
// (air.Tile): the CPU works out the same (air.Weather.Cloud), so the clouds and their shadows agree
// wherever either is drawn.

// cloudLowMass is the low layer's share of the air mass m (airAt), the heaps, the more of them
// the more the sky is covered and the more heaped; cloudHighMass the high's, the shreds, there
// with a few clouds already and the less heaped the more, in patches (cloudHighHere); as
// air.Weather.Share has it; with U.CloudsOneLayer all of it low.
fn cloudLowMass(m: vec4<f32>) -> vec4<f32> {
    if U.CloudsOneLayer > 0.5 {
        return m;
    }
    let heaped = cloudLowLeast + (1.0 - cloudLowLeast) * clamp(m.y, 0.0, 1.0);
    let cover = clamp(m.x, 0.0, 1.0) * heaped;
    return vec4<f32>(cover, 1.0, m.z, m.w);
}

fn cloudHighMass(m: vec4<f32>) -> vec4<f32> {
    if U.CloudsOneLayer > 0.5 {
        return vec4<f32>(0.0);
    }
    let cover = min(cloudHighEarly * clamp(m.x, 0.0, 1.0), 1.0) *
        (1.0 - cloudHighHeaped * clamp(m.y, 0.0, 1.0));
    return vec4<f32>(cover, 0.0, m.z, m.w);
}

// cloudHighHere is the high clouds' mass m (cloudHighMass) at p: there in patches, none between,
// so the high and the low come together or apart.
fn cloudHighHere(m: vec4<f32>, p: vec2<f32>) -> vec4<f32> {
    let q = (p - U.Drift) / cloudPatchWide + 17.0;
    let n = cloudNoise(q, cloudTile * cloudSize / cloudPatchWide);
    return vec4<f32>(m.x * smoothstep(0.3, 0.7, n), m.yzw);
}

// cloudHighHeight is how high the high clouds hang over p, world units: each sheet of shreds at
// its own height through their band, from cloudHighFloor up to U.CloudBases.y.
fn cloudHighHeight(p: vec2<f32>) -> f32 {
    let n = cloudNoise((p - U.Drift) / cloudBandWide, cloudTile * cloudSize / cloudBandWide);
    return mix(cloudHighFloor(), U.CloudBases.y, n);
}

// cloudHighCross is how far along the line from o along d it meets the high clouds' sheet
// (cloudHighHeight) between t0 and t1, found step by step, then halved down to it; -1 if it does
// not.
fn cloudHighCross(o: vec3<f32>, d: vec3<f32>, t0: f32, t1: f32) -> f32 {
    var near = t0;
    var above = o.z + d.z * t0 - cloudHighHeight(o.xy + d.xy * t0);
    let step = (t1 - t0) / f32(cloudCrossSteps);
    for (var i = 1; i <= cloudCrossSteps; i++) {
        let t = t0 + f32(i) * step;
        let now = o.z + d.z * t - cloudHighHeight(o.xy + d.xy * t);
        if (now < 0.0) != (above < 0.0) {
            var a = near;
            var b = t;
            for (var k = 0; k < cloudCrossHalvings; k++) {
                let mid = 0.5 * (a + b);
                let there = o.z + d.z * mid - cloudHighHeight(o.xy + d.xy * mid);
                if (there < 0.0) == (above < 0.0) {
                    a = mid;
                } else {
                    b = mid;
                }
            }
            return 0.5 * (a + b);
        }
        near = t;
        above = now;
    }
    return -1.0;
}

// cloudHighFloor is where the high clouds' band begins: into the low clouds' bodies, so the two
// leave no gap between them.
fn cloudHighFloor() -> f32 {
    return U.CloudBases.x + cloudOverlap * bodyLowDeep * cloudSize;
}

// How much of the cover the low clouds hold, unheaped, all of it heaped; how soon the high ones
// come, a few clouds already holding all of them, and how many fewer the more heaped; how wide
// their patches and the sheets of one height are, world units, a tile holding a whole number of
// each; how far into the low clouds' bodies their band begins.
const cloudLowLeast: f32 = 0.35;
const cloudHighEarly: f32 = 2.0;
const cloudHighHeaped: f32 = 0.5;
const cloudPatchWide: f32 = 2520.0;
const cloudBandWide: f32 = 1260.0;
const cloudOverlap: f32 = 0.6;

// How many steps a line takes looking for the high clouds' sheet, and how many times the one that
// crosses it is halved.
const cloudCrossSteps: i32 = 8;
const cloudCrossHalvings: i32 = 4;

// cloudLowAt is the low clouds' noise at p (x) and how much their edge softens there (y), as a
// pixel pixel world units wide sees them, heaped as billow says (their mass's: 1 but in one
// layer): looked up in the frame's tile of it (CloudTile, air.Tile), evened out as far as the
// pixel sees it; cloudHighAt the high clouds'.
fn cloudLowAt(p: vec2<f32>, pixel: f32, billow: f32) -> vec2<f32> {
    let lod = cloudTileLevel(pixel, cloudTile * cloudSize);
    let torn = cloudTileAt(p, lod, cloudTile * cloudSize).r;
    if billow <= 0.0 {
        return vec2<f32>(torn, cloudTileSoft * lod);
    }
    let core = cloudTileAt(p, cloudTileLevel(pixel, heapTile * heapSize), heapTile * heapSize).g;
    let heap = heaps(heapCoreLow + core * heapCoreSpan, torn);
    return vec2<f32>(cloudMix(torn, heap, billow), cloudTileSoft * lod);
}

fn cloudHighAt(p: vec2<f32>, pixel: f32) -> vec2<f32> {
    let lod = cloudTileLevel(pixel, cloudTile * cloudSize);
    return vec2<f32>(cloudTileAt(p, lod, cloudTile * cloudSize).r, cloudTileSoft * lod);
}

// cloudMix is the clouds' noise of shreds torn and heaps heap mixed as billow says, spread apart
// as either alone: heap alone at 1.
fn cloudMix(torn: f32, heap: f32, billow: f32) -> f32 {
    let b = clamp(billow, 0.0, 1.0);
    if b >= 1.0 {
        return heap;
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
// each cell one heap kept well in from its edges, an ellipse with puffs of many sizes on its rim,
// its size, its shape and the cover it forms under thrown by the cell, its top flat and whole, the
// highest over the point, lobes bulging from its edge; the same again every heapTile cells.
fn heapCores(q: vec2<f32>) -> f32 {
    let c = floor(q);
    var top = -10.0;
    var inside = 0.0;
    for (var j = -1; j <= 1; j++) {
        for (var i = -1; i <= 1; i++) {
            let at = c + vec2<f32>(f32(i), f32(j));
            let k = cloudThrow(modulo2(at, heapTile)); // one throw a cell, turned many ways
            let stray = vec2<f32>(heapRoll(k, 1.0), heapRoll(k, 37.0)) - 0.5;
            let off = q - (at + 0.5 + heapStray * stray);
            // an ellipse, long and turned as the throw says, puffs of many sizes on its rim
            let wide = heapLeast + heapMore * heapRoll(k, 53.0);
            let narrow = wide / (1.0 + heapStretch * heapRoll(k, 59.0));
            let turn = 3.14159265 * heapRoll(k, 61.0);
            let way = vec2<f32>(cos(turn), sin(turn));
            let across = vec2<f32>(-way.y, way.x);
            var into = 1.0 - length(vec2<f32>(dot(off, way) / wide, dot(off, across) / narrow));
            for (var l = 0; l < heapPuffs; l++) {
                let round = 6.2831853 * heapRoll(k, 67.0 + 7.0 * f32(l));
                let size = heapPuffLeast + heapPuffMore * heapRoll(k, 73.0 + 7.0 * f32(l));
                let puff = narrow * size;
                let rim = heapPuffOut * (cos(round) * wide * way + sin(round) * narrow * across);
                into = max(into, 1.0 - length(off - rim) / puff);
            }
            into = min(into / heapFlat, 1.0);
            top = max(top, 1.0 - heapReady * heapRoll(k, 71.0) + heapSlope * (into - 1.0));
            // the deepest in any heap: the winning one's jumped where another rose over it,
            // cutting the lobes off along a straight line
            inside = max(inside, into);
        }
    }
    let lobes = cloudNoise(4.0 * q + vec2<f32>(0.37, 0.61), 4.0 * heapTile) + 0.5 * cloudNoise(8.0 * q + vec2<f32>(0.53, 0.29), 8.0 * heapTile) - 0.75;
    return top + heapLobes * lobes * clamp(1.0 - inside, 0.0, 1.0);
}

// heapRoll is a number 0 to 1 the throw k gives, by its multiplier m: whole numbers all, so the
// CPU (air.heapRoll) works out the very same.
fn heapRoll(k: f32, m: f32) -> f32 {
    return modulo(m * k, 289.0) / 289.0;
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
const heapSize: f32 = 787.5;
const heapTile: f32 = 16.0;
const heapStray: f32 = 0.8;
const heapStretch: f32 = 1.2;
const heapPuffs: i32 = 5;
const heapPuffLeast: f32 = 0.35;
const heapPuffMore: f32 = 0.5;
const heapPuffOut: f32 = 0.8;
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

// cloudTileLevel is the level of a tile of the clouds (air.Tile) period world units wide that a
// pixel pixel world units wide sees it at: a fraction between two.
fn cloudTileLevel(pixel: f32, period: f32) -> f32 {
    return clamp(log2(max(pixel * cloudTileTexels / period, 1.0)), 0.0, cloudTileLevels - 1.0);
}

// cloudTileSpot is where in the clouds' tile (air.Tile) of period world units the world point
// p lies on level level, a whole number, in the tile's pixels from its corner: each level with a
// pixel round it of what lies past its far edge, so the GPU's filter reads across the wrap. The
// shreds' period is cloudTile clouds' widths (red), the heaps' cores' heapTile heaps' cells
// (green).
fn cloudTileSpot(p: vec2<f32>, level: f32, period: f32) -> vec2<f32> {
    let size = cloudTileTexels / exp2(level);
    var origin = vec2<f32>(0.0);
    if level > 0.0 {
        let down = cloudTileTexels - 2.0 * size + 2.0 * (level - 1.0);
        origin = vec2<f32>(cloudTileTexels + 2.0, down);
    }
    let q = fract((p - U.Drift) / period);
    return origin + 1.0 + q * size;
}

// cloudTileAt is the clouds' tile (the frame's CloudTile) at p on level lod, its period period:
// the two levels round it blended.
fn cloudTileAt(p: vec2<f32>, lod: f32, period: f32) -> vec2<f32> {
    let low = floor(lod);
    let dims = vec2<f32>(textureDimensions(CloudTile));
    var n = textureSampleLevel(CloudTile, linear, cloudTileSpot(p, low, period) / dims, 0.0).rg;
    if lod > low {
        let at = cloudTileSpot(p, low + 1.0, period) / dims;
        let next = textureSampleLevel(CloudTile, linear, at, 0.0).rg;
        n = mix(n, next, lod - low);
    }
    return n;
}

// The clouds' tile: how many pixels across its first level, how many levels, and how much the
// clouds' edge softens for each level their noise is seen at, its detail evened out.
const cloudTileTexels: f32 = 1024.0;
const cloudTileLevels: f32 = 6.0;
const cloudTileSoft: f32 = 0.03;

// cloudCover is how thick the clouds of a layer are over a point whose noise is field under the
// layer's air mass m (cloudLowMass, cloudHighMass): 0 in the clear, 1 under the thickest, the noise
// spread out and cut at the mass's cover, heaps' edges sharper than shreds'.
fn cloudCover(field: f32, m: vec4<f32>) -> f32 {
    return cloudCoverSoft(field, 0.0, m);
}

// cloudCoverSoft is cloudCover with the edge softened soft either way, for noise seen from afar,
// its detail evened out.
fn cloudCoverSoft(field: f32, soft: f32, m: vec4<f32>) -> f32 {
    let cover = m.x;
    if cover <= 0.0 {
        return 0.0;
    }
    let n = clamp((field - 0.5) * cloudContrast + 0.5, 0.0, 1.0); // the noise bunches round a half: spread it out
    let edge = mix(cloudEdge, heapEdge, clamp(m.y, 0.0, 1.0));
    return smoothstep(1.0 - cover - soft, 1.0 - cover + edge + soft, n);
}

// cloudThick is how deep in a cloud a point whose noise is field lies under the air mass m: 0 at
// its edge, 1 as deep as cloudDepth and more, where the sun gets least through.
fn cloudThick(field: f32, m: vec4<f32>) -> f32 {
    let n = clamp((field - 0.5) * cloudContrast + 0.5, 0.0, 1.0);
    return clamp((n - (1.0 - m.x)) / cloudDepth, 0.0, 1.0);
}

const cloudDepth: f32 = 0.6;

// How far the noise is spread to make clouds and clear sky between them; over how much of the
// cover a cloud's edge softens, a shred's and a heap's.
const cloudContrast: f32 = 1.8;
const cloudEdge: f32 = 0.15;
const heapEdge: f32 = 0.07;
