// The ground and its cover as the views read them (vision's views): the first image holds, a texel
// every U.GroundStep world units from the world's corner, U.GroundCount in all, the ground's height
// in red and green, 16 bits from U.Low over U.Span, how see-through the cover there is in blue, 1
// for none, and in alpha none where the cover stands without end, a flat world's; the second the
// cover's top in red and green and its bottom in blue and alpha, the same way. Along an axis
// U.GroundWrap says wraps, the texels run on round the world.

// decode is the height 16 bits hold, high byte and low.
fn decode(hi: f32, lo: f32) -> f32 {
    return U.Low + (hi * 255.0 * 256.0 + lo * 255.0) / 65535.0 * U.Span;
}

// groundIndex is the texel i stands for: held within the ground, or round it where it wraps.
fn groundIndex(i: vec2<f32>) -> vec2<f32> {
    var k = clamp(i, vec2<f32>(0.0), U.GroundCount - 1.0);
    if U.GroundWrap.x > 0.5 {
        k.x = i.x - U.GroundCount.x * floor(i.x / U.GroundCount.x);
    }
    if U.GroundWrap.y > 0.5 {
        k.y = i.y - U.GroundCount.y * floor(i.y / U.GroundCount.y);
    }
    return k;
}

// groundTexel is the texel of the ground at i.
fn groundTexel(i: vec2<f32>) -> vec4<f32> {
    return imageSrc0At(imageSrc0Origin() + groundIndex(i) + 0.5);
}

// groundAt is the ground's height at the world point p, blended between the four texels round it.
fn groundAt(p: vec2<f32>) -> f32 {
    let free = p / U.GroundStep;
    var g = clamp(free, vec2<f32>(0.0), U.GroundCount - 1.0001);
    if U.GroundWrap.x > 0.5 {
        g.x = free.x;
    }
    if U.GroundWrap.y > 0.5 {
        g.y = free.y;
    }
    let i = floor(g);
    let f = g - i;
    let a = groundTexel(i);
    let b = groundTexel(i + vec2<f32>(1.0, 0.0));
    let c = groundTexel(i + vec2<f32>(0.0, 1.0));
    let d = groundTexel(i + vec2<f32>(1.0, 1.0));
    let top = mix(decode(a.r, a.g), decode(b.r, b.g), f.x);
    let bottom = mix(decode(c.r, c.g), decode(d.r, d.g), f.x);
    return mix(top, bottom, f.y);
}

// coverAt is the cover at the world point p, the nearest texel's: how see-through it is, its
// bottom and its top; see-through 1 where there is none.
fn coverAt(p: vec2<f32>) -> vec3<f32> {
    let k = groundIndex(floor(p / U.GroundStep + 0.5));
    let texel = imageSrc0At(imageSrc0Origin() + k + 0.5);
    let tau = texel.b;
    if tau > 0.999 {
        return vec3<f32>(1.0, 0.0, 0.0);
    }
    if texel.a < 0.5 {
        return vec3<f32>(tau, -1e9, 1e9); // without end
    }
    let band = imageSrc1At(imageSrc1Origin() + k + 0.5);
    return vec3<f32>(tau, decode(band.b, band.a), decode(band.r, band.g));
}
