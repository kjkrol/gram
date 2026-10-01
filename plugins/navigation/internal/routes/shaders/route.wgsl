// The routes laid over the ground (navigation's routes): an instance a stretch of a route — from
// (x, y) to (z, w), world units — and the rectangle of the viewport it may cover, pixels. Every
// pixel of it finds the ground point drawn there from the depth the ground left (depthAt, through
// U.Unproject, the viewport U.ViewSize pixels from U.ViewAt on the target) and, as near the
// stretch as half of U.LineWidth pixels, draws it in U.LineColor.

struct Stretch {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) ends: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) ends: vec4<f32>, @location(1) rect: vec4<f32>) -> Stretch {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var s: Stretch;
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    s.ends = ends;
    return s;
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>, s: Stretch) -> @location(0) vec4<f32> {
    let depth = depthAt(frag.xy);
    let px = frag.xy - U.ViewAt;
    let w = U.Unproject * vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, depth, 1.0);
    let p = w.xy / w.w;
    let ab = s.ends.zw - s.ends.xy;
    let t = clamp(dot(p - s.ends.xy, ab) / max(dot(ab, ab), 1e-6), 0.0, 1.0);
    let d = distance(p, s.ends.xy + ab * t);
    // how far that is in pixels, as the ground runs under the screen there
    let pixels = d / max(length(vec2<f32>(dpdx(d), dpdy(d))), 1e-6);
    let cover = 1.0 - smoothstep(U.LineWidth * 0.5 - 0.5, U.LineWidth * 0.5 + 0.5, pixels);
    if depth <= 0.0 || cover < 0.002 {
        discard; // nothing drawn there, the sky; or off the line
    }
    return U.LineColor * cover;
}
