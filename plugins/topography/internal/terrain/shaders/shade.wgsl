// The sun on the ground, baked: every pixel of an image U.ShadePx pixels a cell holds how much of
// the sun reaches the ground at its point past the relief (sunlit) and the blocks standing on it
// (blocked), for the mesh to read between them.

// Fragment is how much of the sun reaches the ground at the world point custom.xy.
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let p = custom.xy;
    let at = vec3<f32>(p, height(p));
    let lit = min(sunlit(at, sunWay()), blocked(at, sunWay()));
    return vec4<f32>(lit, lit, lit, 1.0);
}

// blocked is how much of the sun, the way sun, reaches p past the blocks (U.Blocks, two vec4s a
// block: its middle and the way it runs, then half its length and width, its bottom and its top):
// none where the line to the sun passes through one, fading in over the penumbra as the relief's
// shadows do — how far the line passes by against how far along it the block lies.
fn blocked(p: vec3<f32>, sun: vec3<f32>) -> f32 {
    if U.Shadows < 0.5 || sun.z <= 0.0 {
        return 1.0;
    }
    var lit = 1.0;
    for (var k = 0; k < i32(U.BlockCount); k++) {
        let at = U.Blocks[2 * k];
        let size = U.Blocks[2 * k + 1];
        if p.z >= size.w {
            continue;
        }
        let near = max(size.z - p.z, 0.0) / sun.z;
        let far = (size.w - p.z) / sun.z;
        if distance(p.xy, at.xy) > size.x + size.y + far * (length(sun.xy) + shadowSoft) {
            continue; // the line to the sun ends short of it, the penumbra too
        }
        let a = p.xy + sun.xy * near - at.xy;
        let b = p.xy + sun.xy * far - at.xy;
        let across = vec2<f32>(-at.w, at.z);
        let by = passesBy(
            vec2<f32>(dot(a, at.zw), dot(a, across)),
            vec2<f32>(dot(b, at.zw), dot(b, across)),
            size.xy,
        );
        lit = min(lit, clamp(by / max(0.5 * (near + far) * shadowSoft, 1e-3), 0.0, 1.0));
    }
    return lit;
}

// passesBy is how far the segment from a to b passes by the rectangle half h wide round the
// origin: 0 where it crosses it.
fn passesBy(a: vec2<f32>, b: vec2<f32>, h: vec2<f32>) -> f32 {
    let d = b - a;
    let within = clipped(clipped(vec2<f32>(0.0, 1.0), a.x, d.x, h.x), a.y, d.y, h.y);
    if within.x <= within.y {
        return 0.0;
    }
    let off = max(abs(a) - h, vec2<f32>(0.0));
    let offB = max(abs(b) - h, vec2<f32>(0.0));
    var by = min(length(off), length(offB));
    by = min(by, fromSegment(vec2<f32>(h.x, h.y), a, d));
    by = min(by, fromSegment(vec2<f32>(-h.x, h.y), a, d));
    by = min(by, fromSegment(vec2<f32>(h.x, -h.y), a, d));
    by = min(by, fromSegment(vec2<f32>(-h.x, -h.y), a, d));
    return by;
}

// clipped narrows the span t of a segment, from a running d along one axis, to where it lies within
// h of the origin on that axis: empty where none does.
fn clipped(t: vec2<f32>, a: f32, d: f32, h: f32) -> vec2<f32> {
    if abs(d) < 1e-6 {
        return select(t, vec2<f32>(1.0, 0.0), abs(a) > h);
    }
    let ta = (-h - a) / d;
    let tb = (h - a) / d;
    return vec2<f32>(max(t.x, min(ta, tb)), min(t.y, max(ta, tb)));
}

// fromSegment is how far q lies from the segment from a running d.
fn fromSegment(q: vec2<f32>, a: vec2<f32>, d: vec2<f32>) -> f32 {
    let t = clamp(dot(q - a, d) / max(dot(d, d), 1e-12), 0.0, 1.0);
    return length(q - a - d * t);
}
