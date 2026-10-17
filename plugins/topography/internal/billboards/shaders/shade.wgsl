// The shadows of what stands laid over whatever ground the frame drew (topography's shades), an
// instance a cast: its middle (x, y) and the way away from the sun (z, w); how far it reaches along
// and across that way, how far in it fades and how dark it is (sky.Patch); the corners of the
// sprite on the first image whose shape it has, all 0 for a patch's own; the rectangle of the
// viewport it may cover, pixels. Every pixel of it finds the ground point drawn there from the
// ground's depth (depthAt, through U.Unproject, the viewport U.ViewSize pixels from U.ViewAt) and
// darkens it as the terrain's shadows do (terrain's shadow.wgsl), lying at the ground's depth.

struct Shade {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec4<f32>,
    @location(1) @interpolate(flat) size: vec4<f32>,
    @location(2) @interpolate(flat) sprite: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) at: vec4<f32>,
    @location(1) size: vec4<f32>,
    @location(2) sprite: vec4<f32>,
    @location(3) rect: vec4<f32>,
) -> Shade {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var s: Shade;
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    s.at = at;
    s.size = size;
    s.sprite = sprite;
    return s;
}

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>, s: Shade) -> Laid {
    let depth = depthAt(frag.xy);
    if depth <= 0.0 {
        discard; // nothing drawn there
    }
    let px = frag.xy - U.ViewAt;
    let w = U.Unproject * vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, depth, 1.0);
    let rel = w.xy / w.w - s.at.xy;
    let local = vec2<f32>(dot(rel, s.at.zw), dot(rel, vec2<f32>(-s.at.w, s.at.z)));
    let edges = vec4<f32>(s.size.x + local.x, s.size.x - local.x, s.size.y + local.y, s.size.y - local.y) / max(s.size.z, 1e-3);
    let f = smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(edges, vec4<f32>(0.0), vec4<f32>(1.0)));
    let a = s.size.w * f.x * f.y * f.z * f.w * cast(local, s.size, s.sprite);
    if a < 0.002 {
        discard;
    }
    return Laid(vec4<f32>(0.0, 0.0, 0.0, a), depth);
}

// cast is how much of the sprite's shape covers the point local of a cast of size: all of it for a
// patch's own; the sprite's feet lie at the end nearest the sun, its head away from it.
fn cast(local: vec2<f32>, size: vec4<f32>, sprite: vec4<f32>) -> f32 {
    let used = abs(sprite);
    if max(max(used.x, used.y), max(used.z, used.w)) <= 0.0 {
        return 1.0;
    }
    let along = clamp((local.x + size.x) / max(2.0 * size.x, 1e-3), 0.0, 1.0);
    let across = clamp((local.y + size.y) / max(2.0 * size.y, 1e-3), 0.0, 1.0);
    let lo = min(sprite.xy, sprite.zw) + 0.5;
    let hi = max(sprite.xy, sprite.zw) - 0.5;
    let at = vec2<f32>(mix(sprite.x, sprite.z, across), mix(sprite.w, sprite.y, along));
    let uv = clamp(at, lo, hi);
    return imageSrc0Linear(uv).a;
}
