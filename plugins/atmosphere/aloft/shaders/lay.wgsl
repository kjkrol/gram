// The clouds worked out coarser than the screen (aloft.Renderer, image0) laid over a view
// U.LayView pixels wide, U.LayCoarse pixels of it a pixel of theirs, a coarse pixel's worth
// blurred: their steps put off at random a pixel showed as grain. Each lies as deep as the
// nearest of the coarse pixels round it left in the depth read (depthAt), so what stands nearer
// hides them.

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

struct Laid {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(o: Out) -> Laid {
    let at = imageSrc0Origin() + o.px / U.LayCoarse;
    var l: Laid;
    l.color = 0.25 * (imageSrc0Linear(at + vec2<f32>(0.5, 0.5)) +
        imageSrc0Linear(at + vec2<f32>(-0.5, 0.5)) + imageSrc0Linear(at + vec2<f32>(0.5, -0.5)) +
        imageSrc0Linear(at + vec2<f32>(-0.5, -0.5)));
    if l.color.a < 1.0 / 512.0 {
        discard;
    }
    l.depth = coarseDepth(at);
    return l;
}

// coarseDepth is the depth the clouds left at their pixel p, held within their image.
fn coarseDepth(p: vec2<f32>) -> f32 {
    return depthAt(clamp(p, imageSrc0Origin(), imageSrc0Origin() + imageSrc0Size() - 1.0));
}
