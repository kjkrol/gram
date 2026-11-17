// The wind's gusts: bands lying across the wind and carried along by it (U.Blown), two thrown in
// every square U.GustCell a side, each its own length, strength and place, and how what sways
// leans in them (air.Weather's Gust and Sway, the same).

const gustLeast: f32 = 3.75;
const gustMost: f32 = 6.25;
const gustCell: f32 = 7.5;
const gustStray: f32 = 0.3;
const gustBow: f32 = 0.6;
const gustFront: f32 = 0.6;
const gustBack: f32 = 1.25;
const gustTaper: f32 = 0.55;
const gustFaint: f32 = 0.65;
const gustPace: f32 = 1.6;
const blownSpan: f32 = 262144.0;

const windFull: f32 = 40.0;
const windLeaning: f32 = 0.35;
const windRest: f32 = 0.25;
const windRock: f32 = 0.25;
const windRate: f32 = 2.2;
const windBeat: vec2<f32> = vec2<f32>(0.071, 0.113);

// gustAt is how hard the gusts blow over p now, 0 in the steady wind between them to 1 under a
// band's middle, every band as much wider as span, the world units p stands for.
fn gustAt(p: vec2<f32>, span: f32) -> f32 {
    let blow = length(U.Wind);
    let side = U.GustCell;
    if blow <= 0.0 || side <= 0.0 {
        return 0.0;
    }
    let w = U.Wind / blow;
    let wide = side / gustCell;
    let cells = round(gustPace * blownSpan / side);
    let q = p - U.Blown * gustPace;
    let first = floor(q / side - 0.5);
    var best = 0.0;
    for (var j = 0; j < 4; j++) {
        let c = first + vec2<f32>(f32(j % 2), f32(j / 2));
        let i = c - cells * floor(c / cells);
        for (var k = 0; k < 2; k++) {
            let h = gustThrow(u32(i.x), u32(i.y), u32(k));
            let stray = vec2<f32>(f32(h & 1023u), f32((h >> 10u) & 1023u)) / 1023.0 - 0.5;
            let long = f32((h >> 20u) & 1023u) / 1023.0;
            let strong = f32(h >> 30u) / 3.0;
            // half a square apart along the wind, each strayed its own way
            let off = (f32(k) - 0.5) * 0.5 * side;
            let d = q - ((c + 0.5) * side + w * off + stray * (gustStray * side));
            let half = 0.5 * wide * (gustLeast + (gustMost - gustLeast) * long);
            let u = (d.y * w.x - d.x * w.y) / half;
            if u <= -1.0 || u >= 1.0 {
                continue;
            }
            let a = dot(d, w) + gustBow * wide * u * u;
            var reach = gustBack * wide + span / 2.0;
            if a > 0.0 {
                reach = gustFront * wide + span / 2.0;
            }
            let v = a / reach;
            if v <= -1.0 || v >= 1.0 {
                continue;
            }
            let gust = (1.0 - v * v) * (1.0 - v * v) * (1.0 - smoothstep(gustTaper, 1.0, abs(u)))
                * (gustFaint + (1.0 - gustFaint) * strong);
            best = max(best, gust);
        }
    }
    return best;
}

// gustThrow is the throw of the k-th band of the square (x, y): whole numbers, so every machine
// throws the same.
fn gustThrow(x: u32, y: u32, k: u32) -> u32 {
    var h = (x * 0x9E3779B1u) ^ (y * 0x85EBCA77u) ^ (k * 0xC2B2AE3Du);
    h ^= h >> 15u;
    h *= 0x2C1B3C6Du;
    h ^= h >> 12u;
    h *= 0x297A2D39u;
    h ^= h >> 15u;
    return h;
}

// windStrength is the wind's strength for what sways: 1 at windFull, 1.5 at most.
fn windStrength() -> f32 {
    return min(length(U.Wind) / windFull, 1.5);
}

// windLeanUnder is how far what sways amount leans under a gust blowing as hard as gust, as a
// slope along x and y: a little in the steady wind, its whole lean under a band's middle.
fn windLeanUnder(amount: f32, gust: f32) -> vec2<f32> {
    let blow = length(U.Wind);
    if blow <= 0.0 || amount <= 0.0 {
        return vec2<f32>(0.0);
    }
    let lean = amount * windLeaning * windStrength() * (windRest + (1.0 - windRest) * gust);
    return U.Wind / blow * lean;
}

// windLean is how far what sways amount leans at p now, rocking as it leans, neighbours apart.
fn windLean(amount: f32, p: vec2<f32>) -> vec2<f32> {
    if amount <= 0.0 {
        return vec2<f32>(0.0);
    }
    let rock = 1.0 + windRock * sin(U.Clock * windRate + dot(p, windBeat));
    return windLeanUnder(amount, gustAt(p, 0.0)) * rock;
}
