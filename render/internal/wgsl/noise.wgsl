// Value noise for the materials: smooth, 0 to 1, changing over a unit.

// noise is smooth value noise at p.
fn noise(p: vec2<f32>) -> f32 {
    let i = floor(p);
    let f = fract(p);
    let u = f * f * (3.0 - 2.0 * f);
    let a = hash(i);
    let b = hash(i + vec2<f32>(1.0, 0.0));
    let c = hash(i + vec2<f32>(0.0, 1.0));
    let d = hash(i + vec2<f32>(1.0, 1.0));
    return mix(mix(a, b, u.x), mix(c, d, u.x), u.y);
}

// noised is noise at p and its gradient: the value in x, how it rises along x and y in y and z —
// one noise's four hashes for what finite differences take five noises for.
fn noised(p: vec2<f32>) -> vec3<f32> {
    let i = floor(p);
    let f = fract(p);
    let u = f * f * (3.0 - 2.0 * f);
    let du = 6.0 * f * (1.0 - f);
    let a = hash(i);
    let b = hash(i + vec2<f32>(1.0, 0.0));
    let c = hash(i + vec2<f32>(0.0, 1.0));
    let d = hash(i + vec2<f32>(1.0, 1.0));
    let k = a - b - c + d;
    return vec3<f32>(a + (b - a) * u.x + (c - a) * u.y + k * u.x * u.y, du * vec2<f32>(b - a + k * u.y, c - a + k * u.x));
}

// hash is a number 0 to 1 fixed for the whole point i.
fn hash(i: vec2<f32>) -> f32 {
    return fract(sin(dot(i, vec2<f32>(127.1, 311.7))) * 43758.5453);
}
