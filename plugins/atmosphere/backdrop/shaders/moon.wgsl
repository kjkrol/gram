// The Earth's moon: its glow round it, and its face a half ball towards the eye, lit as the sun
// stands to it, a little light on its dark side, which shows the sky's colour otherwise.
fn SkyMoon(s: SkyAt, d: SkyDisc) -> SkyShine {
    var o = SkyShine();
    let q = (s.px - d.at) / d.radius;
    let r = length(q);
    o.glow = d.glow * d.shown * moonGlow * exp(-max(r - 1.0, 0.0) * moonGlowFall);
    if r < 1.0 {
        let n = vec3<f32>(q, sqrt(1.0 - r * r));
        let lit = smoothstep(-0.03, 0.08, dot(n, d.lit));
        let face = d.color * moonAlbedo(q, d) * (earthshine + (1.0 - earthshine) * lit);
        let a = (1.0 - smoothstep(1.0 - 1.5 / d.radius, 1.0, r)) * d.shown;
        o.front = vec4<f32>(mix(s.air, face, max(lit, earthshine * s.stars)) * a, a);
    }
    return o;
}

// The moon: its glow round it, how bright and how fast it falls off, out in radii; how much light
// its dark side takes; how bright its face is to its picture's grey.
const moonGlow: f32 = 0.08;
const moonGlowFall: f32 = 1.2;
const earthshine: f32 = 0.08;
const moonBright: f32 = 1.7;

// moonAlbedo is how bright the moon's face is at q, in its radii from the middle of its disc on
// the screen (y down): its picture in image1, north turned to d.north, four of its pixels blended
// where the disc is drawn smaller than the picture.
fn moonAlbedo(q: vec2<f32>, d: SkyDisc) -> f32 {
    let north = d.north;
    let east = vec2<f32>(-north.y, north.x);
    let uv = vec2<f32>(0.5 + 0.5 * dot(q, east), 0.5 - 0.5 * dot(q, north));
    let size = f32(textureDimensions(image1).x);
    let o = max(0.5 * size / d.radius - 1.0, 0.0) * 0.25 / size; // how far apart, when smaller
    var a = textureSampleLevel(image1, linear, uv, 0.0).r;
    if o > 0.0 {
        a = 0.25 * (textureSampleLevel(image1, linear, uv + vec2<f32>(o, o), 0.0).r +
            textureSampleLevel(image1, linear, uv + vec2<f32>(-o, o), 0.0).r +
            textureSampleLevel(image1, linear, uv + vec2<f32>(o, -o), 0.0).r +
            textureSampleLevel(image1, linear, uv + vec2<f32>(-o, -o), 0.0).r);
    }
    return a * moonBright;
}
