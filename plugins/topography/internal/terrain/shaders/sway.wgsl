// The wind over the ground: what stands on a cell leans with it as air.Weather.Sway leans what
// stands (the same wind, rate and gust, so the trees lean in step), the gusts going over the
// ground in fronts of the wind's own noise, as a wave runs over a field. The wave and how much
// stands are worked out at the lattice's corners (mesh.wgsl's vs_main) and blended between them.

const swayWind = 40.0;
const swayLeaning = 0.35;
const swayRate = 2.2;
const swayGust = 60.0;
const swayFront = 90.0;
const swayFrontWide = 120.0;
const swayFrontPace = 1.3;

// swayStrength is the wind's strength for the sway, 1 at swayWind, 1.5 at most.
fn swayStrength() -> f32 {
    return min(length(U.Wind) / swayWind, 1.5);
}

// swayAmountAt is how much what stands round the lattice corner i sways: the mean of its four
// cells' (the flags' blue).
fn swayAmountAt(i: vec2<f32>) -> f32 {
    let o = imageSrc0Origin() + vec2<f32>(0.0, U.Corners.y) + 0.5;
    let last = U.Corners - 2.0;
    var sum = 0.0;
    for (var dy = -1.0; dy <= 0.0; dy += 1.0) {
        for (var dx = -1.0; dx <= 0.0; dx += 1.0) {
            let c = clamp(i + vec2<f32>(dx, dy), vec2<f32>(0.0), last);
            sum += imageSrc0At(o + c).b;
        }
    }
    return sum / 4.0;
}

// swayWave is the wave going over the ground at p now, -1 to 1: the gust's rock along the wind
// and the fronts it comes in.
fn swayWave(p: vec2<f32>) -> f32 {
    let blow = length(U.Wind);
    if blow <= 0.0 {
        return 0.0;
    }
    let w = U.Wind / blow;
    let along = dot(p, w);
    let aside = dot(p, vec2<f32>(-w.y, w.x));
    let rock = sin(U.Clock * swayRate * (0.7 + 0.3 * swayStrength()) - along / swayGust);
    let front = noise(vec2<f32>(along / swayFront - U.Clock * swayFrontPace,
        aside / swayFrontWide));
    return rock * (0.4 + 0.6 * front);
}

// swayLean is how far what sways amount leans under the wave, as a slope along x and y.
fn swayLean(amount: f32, wave: f32) -> vec2<f32> {
    let blow = length(U.Wind);
    if blow <= 0.0 || amount <= 0.0 {
        return vec2<f32>(0.0);
    }
    return U.Wind / blow * (amount * swayLeaning * swayStrength() * (0.5 + 0.5 * wave));
}
