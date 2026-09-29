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

// hash is a number 0 to 1 fixed for the whole point i.
fn hash(i: vec2<f32>) -> f32 {
    return fract(sin(dot(i, vec2<f32>(127.1, 311.7))) * 43758.5453);
}
