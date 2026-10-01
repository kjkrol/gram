// The shadows of what stands laid over whatever ground the frame drew (topography's shades), an
// instance a patch (sky.Patch): its middle (x, y) and the way away from the sun (z, w); how far it
// reaches along and across that way, how far in it fades and how dark it is; the rectangle of the
// viewport it may cover, pixels. Every pixel of it finds the ground point drawn there from the
// frame's depth (depthAt, through U.Unproject, the viewport U.ViewSize pixels from U.ViewAt) and
// darkens it as the terrain's shadows do (terrain's shadow.wgsl).

struct Shade {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) at: vec4<f32>,
    @location(1) @interpolate(flat) size: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>, @location(1) size: vec4<f32>, @location(2) rect: vec4<f32>) -> Shade {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var s: Shade;
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    s.at = at;
    s.size = size;
    return s;
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>, s: Shade) -> @location(0) vec4<f32> {
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
    let a = s.size.w * f.x * f.y * f.z * f.w;
    if a < 0.002 {
        discard;
    }
    return vec4<f32>(0.0, 0.0, 0.0, a);
}
