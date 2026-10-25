// The sun on the ground, baked: every pixel of an image holds how much of the light reaches the
// ground at its point past the relief (red) and how high over the ground the relief's shadow
// reaches there (green, the root of its share of U.ShadeSpan); the blocks are baked over it
// after (blocks.wgsl), blue and alpha theirs.

// Fragment is the relief's shade at the world point custom.xy.
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let p = custom.xy;
    let shade = reliefShade(vec3<f32>(p, bakedHeight(p)), lightWay());
    return vec4<f32>(shade.x, sqrt(clamp(shade.y / U.ShadeSpan, 0.0, 1.0)), 1.0, 1.0);
}

// reliefShade is how much of the light, the way sun, reaches p past the relief and how high over p
// the relief's shadow reaches: the most the ground towards the light rises over the line from p to
// it, within shadowSteps steps of three quarters of a cell, fading in over the penumbra — the
// clearance over the ground against how far along the line it is, so level ground stays lit
// under a low light. Level ground, no relief, is in the light everywhere.
fn reliefShade(p: vec3<f32>, sun: vec3<f32>) -> vec2<f32> {
    if U.GroundLattice.w < 0.5 || sun.z <= 0.0 {
        return vec2<f32>(1.0, 0.0);
    }
    let stride = U.BakeCell * 0.75;
    let top = bakedTop();
    var lit = 1.0;
    var over = 0.0;
    for (var k = 1; k <= shadowSteps; k++) {
        let t = stride * f32(k);
        let q = p + sun * t;
        if q.z > top {
            break;
        }
        let clearance = q.z - bakedHeight(q.xy);
        over = max(over, -clearance);
        if clearance < 0.0 {
            lit = 0.0;
        } else {
            lit = min(lit, clearance / (t * shadowSoft));
        }
    }
    return vec2<f32>(lit, over);
}
