// The clouds worked out coarser than the screen (aloft.Renderer, image0) laid over a view
// U.LayView pixels wide, U.LayCoarse pixels of it a pixel of theirs, a coarse pixel's worth
// blurred: their steps put off at random a pixel showed as grain.

struct Out {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.LayView;
    return o;
}

@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    let at = imageSrc0Origin() + o.px / U.LayCoarse;
    return 0.25 * (imageSrc0Linear(at + vec2<f32>(0.5, 0.5)) +
        imageSrc0Linear(at + vec2<f32>(-0.5, 0.5)) + imageSrc0Linear(at + vec2<f32>(0.5, -0.5)) +
        imageSrc0Linear(at + vec2<f32>(-0.5, -0.5)));
}
