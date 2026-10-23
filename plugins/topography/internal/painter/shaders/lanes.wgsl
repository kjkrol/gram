// The ways across a board's tiles drawn from above (painter's lanes.go), an instance a piece of a
// way: its corners in the world (top-left, top-right, bottom-left, bottom-right), how much of the
// way shows at each (weight) and how far its look has turned into its mix sprite there (mix), how
// fast its water runs there (flow), the sheet's rectangles of its sprite and its mix sprite, half
// a texel in, and its shine, its mix's shine, how soft it blends and its bits. A world unit lies
// U.LaneTo.xy apart in clip space from U.LaneTo.zw. Each piece is drawn as the composer draws
// DrawWay's: the sprite, blended where faded; glazed into the mix sprite; water running over it
// (U.LaneStream) as far as it has not turned, and glinting (U.LaneGlint) as far as it has, unfaded.

struct Lane {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) weight: f32,
    @location(3) mix: f32,
    @location(4) flow: vec2<f32>,
    @location(5) @interpolate(flat) look: vec4<f32>,
    @location(6) @interpolate(flat) glaze: vec4<f32>,
    @location(7) @interpolate(flat) params: vec4<f32>,
}

// a piece's bits
const laneFaded: u32 = 1u;
const laneMixes: u32 = 2u;
const laneRuns: u32 = 4u;
const laneGlints: u32 = 8u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) top: vec4<f32>,
    @location(1) bottom: vec4<f32>,
    @location(2) weight: vec4<f32>,
    @location(3) mix: vec4<f32>,
    @location(4) topFlow: vec4<f32>,
    @location(5) bottomFlow: vec4<f32>,
    @location(6) look: vec4<f32>,
    @location(7) glaze: vec4<f32>,
    @location(8) params: vec4<f32>,
) -> Lane {
    // two triangles: top-left, top-right, bottom-left, then top-right, bottom-left, bottom-right
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    var at = top.xy;
    var flow = topFlow.xy;
    if k == 1u {
        at = top.zw;
        flow = topFlow.zw;
    } else if k == 2u {
        at = bottom.xy;
        flow = bottomFlow.xy;
    } else if k == 3u {
        at = bottom.zw;
        flow = bottomFlow.zw;
    }
    var out: Lane;
    out.clip = vec4<f32>(at * U.LaneTo.xy + U.LaneTo.zw, 0.0, 1.0);
    out.world = at;
    out.uv = vec2<f32>(f32(k % 2u), f32(k / 2u));
    out.weight = weight[k];
    out.mix = mix[k];
    out.flow = flow;
    out.look = look;
    out.glaze = glaze;
    out.params = params;
    return out;
}

@fragment
fn fs_main(in: Lane) -> @location(0) vec4<f32> {
    let bits = u32(in.params.w + 0.5);
    let mark = 100000.0 + in.params.z;
    var a = 1.0;
    if (bits & laneFaded) != 0u {
        a = blended(in.weight, mark);
    }
    var c = imageSrc0At(mix(in.look.xy, in.look.zw, in.uv)) * a;
    if (bits & laneMixes) != 0u {
        c = atop(imageSrc0At(mix(in.glaze.xy, in.glaze.zw, in.uv)) * (a * in.mix), c);
    }
    if (bits & laneRuns) != 0u {
        var custom = vec4<f32>(in.flow, 0.0, 0.0);
        if (bits & laneFaded) != 0u {
            custom = vec4<f32>(in.flow, in.weight, mark);
        }
        c = atop(material(U.LaneStream, in.world, in.params.x * (1.0 - in.mix), 1.0, custom), c);
    }
    if (bits & laneGlints) != 0u {
        c = atop(material(U.LaneGlint, in.world, in.params.y * in.mix, 1.0, vec4<f32>(0.0)), c);
    }
    return c;
}

// atop is a laid over b, both premultiplied.
fn atop(a: vec4<f32>, b: vec4<f32>) -> vec4<f32> {
    return a + b * (1.0 - a.a);
}
