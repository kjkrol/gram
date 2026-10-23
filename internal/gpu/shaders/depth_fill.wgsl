// A window of a depth buffer filled with one depth (gpu.FillDepth): a triangle over the whole
// target, scissored to the window, every pixel's depth D.place.x, no colour.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

struct Filled {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main() -> Filled {
    return Filled(vec4<f32>(0.0), D.place.x);
}
