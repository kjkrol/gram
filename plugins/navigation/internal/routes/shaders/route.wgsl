// The routes laid over the ground (navigation's routes): an instance a stretch of a route — from
// (x, y) to (z, w), world units — and the rectangle of the viewport it may cover, pixels. Every
// pixel of it finds the ground point drawn there from the depth the ground left (depthAt, through
// U.Unproject, the viewport U.ViewSize pixels from U.ViewAt on the target) and, as near the
// stretch as half of its width (pixels, the fourth vec4's x), draws it in its colour (the third),
// lying at the ground's depth.

struct Stretch {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) ends: vec4<f32>,
    @location(1) @interpolate(flat) color: vec4<f32>,
    @location(2) @interpolate(flat) width: f32,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) ends: vec4<f32>, @location(1) rect: vec4<f32>, @location(2) color: vec4<f32>, @location(3) line: vec4<f32>) -> Stretch {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let px = mix(rect.xy, rect.zw, vec2<f32>(across[vid], down[vid]));
    var s: Stretch;
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    s.ends = ends;
    s.color = color;
    s.width = line.x;
    return s;
}

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(s: Stretch) -> Laid {
    let depth = depthAt(s.clip.xy);
    let px = s.clip.xy - U.ViewAt;
    let w = U.Unproject * vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, depth, 1.0);
    let p = w.xy / w.w;
    let ab = s.ends.zw - s.ends.xy;
    let t = clamp(dot(p - s.ends.xy, ab) / max(dot(ab, ab), 1e-6), 0.0, 1.0);
    let d = distance(p, s.ends.xy + ab * t);
    // how far that is in pixels, as the ground runs under the screen there
    let pixels = d / max(length(vec2<f32>(dpdx(d), dpdy(d))), 1e-6);
    let cover = 1.0 - smoothstep(s.width * 0.5 - 0.5, s.width * 0.5 + 0.5, pixels);
    if depth <= 0.0 || cover < 0.002 {
        discard; // nothing drawn there, the sky; or off the line
    }
    return Laid(s.color * cover, depth);
}
