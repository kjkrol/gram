// A depth buffer drawn into another (gpu.CopyDepth): a triangle over the whole target, every
// pixel's depth the one read there, no colour.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

struct Copied {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(@builtin(position) p: vec4<f32>) -> Copied {
    return Copied(vec4<f32>(0.0), depthAt(p.xy));
}
