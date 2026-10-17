// The shadows of the blocks standing on the ground, baked over the relief's in a min blend that
// keeps the darkest: an instance a block — its middle and the way it runs, then half its length
// and width, its bottom and its top — drawn over the part of the strip U.Strip (its world origin
// and size) its shadow may reach. Red and alpha are how much of the sun reaches the ground past
// it, blue one less the root of the share of U.ShadeSpan its shadow reaches over the ground.

struct Cast {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) @interpolate(flat) at: vec4<f32>,
    @location(2) @interpolate(flat) size: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) at: vec4<f32>,
    @location(1) size: vec4<f32>,
) -> Cast {
    let sun = sunWay();
    // the lowest ground the block's shadow falls on lies this far from it, away from the sun
    let far = (size.w - U.Low) / max(sun.z, 1e-3);
    let reach = -sun.xy * far;
    let across = vec2<f32>(-at.w, at.z);
    var lo = vec2<f32>(1e30, 1e30);
    var hi = vec2<f32>(-1e30, -1e30);
    for (var k = 0u; k < 4u; k++) {
        let along = select(-1.0, 1.0, (k & 1u) == 1u) * size.x;
        let wide = select(-1.0, 1.0, (k & 2u) == 2u) * size.y;
        let c = at.xy + at.zw * along + across * wide;
        lo = min(lo, min(c, c + reach));
        hi = max(hi, max(c, c + reach));
    }
    let spare = far * shadowSoft + U.Cell;
    lo -= vec2<f32>(spare, spare);
    hi += vec2<f32>(spare, spare);
    var xs = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var ys = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let w = mix(lo, hi, vec2<f32>(xs[vid], ys[vid]));
    let s = (w - U.Strip.xy) / U.Strip.zw;
    var c: Cast;
    c.clip = vec4<f32>(s.x * 2.0 - 1.0, 1.0 - s.y * 2.0, 0.0, 1.0);
    c.world = w;
    c.at = at;
    c.size = size;
    return c;
}

@fragment
fn fs_main(c: Cast) -> @location(0) vec4<f32> {
    let sun = sunWay();
    let p = vec3<f32>(c.world, height(c.world));
    let lit = blockedBy(p, sun, c.at, c.size);
    let top = columnTop(p, sun, c.at, c.size);
    return vec4<f32>(lit, 1.0, 1.0 - sqrt(clamp(top / U.ShadeSpan, 0.0, 1.0)), lit);
}

// blockedBy is how much of the sun, the way sun, reaches p past the block at, size: none where the
// line to the sun passes through it, fading in over the penumbra as the relief's shadows do — how
// far the line passes by against how far along it the block lies.
fn blockedBy(p: vec3<f32>, sun: vec3<f32>, at: vec4<f32>, size: vec4<f32>) -> f32 {
    if sun.z <= 0.0 || p.z >= size.w {
        return 1.0;
    }
    let near = max(size.z - p.z, 0.0) / sun.z;
    let far = (size.w - p.z) / sun.z;
    let a = p.xy + sun.xy * near - at.xy;
    let b = p.xy + sun.xy * far - at.xy;
    let across = vec2<f32>(-at.w, at.z);
    let by = passesBy(
        vec2<f32>(dot(a, at.zw), dot(a, across)),
        vec2<f32>(dot(b, at.zw), dot(b, across)),
        size.xy,
    );
    return clamp(by / max(0.5 * (near + far) * shadowSoft, 1e-3), 0.0, 1.0);
}

// columnTop is how high over p the block's shadow reaches: where the line to the sun from p's
// ground track first comes over the block, down by as far as the sun rises meanwhile; 0 where the
// track misses it.
fn columnTop(p: vec3<f32>, sun: vec3<f32>, at: vec4<f32>, size: vec4<f32>) -> f32 {
    if sun.z <= 0.0 {
        return 0.0;
    }
    let across = vec2<f32>(-at.w, at.z);
    let a = p.xy - at.xy;
    let o = vec2<f32>(dot(a, at.zw), dot(a, across));
    let d = vec2<f32>(dot(sun.xy, at.zw), dot(sun.xy, across));
    let within = clipped(clipped(vec2<f32>(0.0, 1e9), o.x, d.x, size.x), o.y, d.y, size.y);
    if within.x > within.y {
        return 0.0;
    }
    return max(size.w - sun.z * within.x - p.z, 0.0);
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
