// The ways' pieces over a hex board's tops (paint_ways.wgsl): the way's sprite, blended where it
// fades, turned into its mix's sprite, water running over it (U.LaneStream) as far as it has not
// turned and glinting (U.LaneGlint) as far as it has, where it does not fade.

@fragment
fn fs_main(in: Lane) -> @location(0) vec4<f32> {
    let bits = u32(in.params.w + 0.5);
    let mark = 100000.0 + clamp(in.params.z, 0.01, 0.5);
    var a = 1.0;
    if (bits & laneFaded) != 0u {
        a = shows(in.weight, in.params.z);
    }
    var c = sampled(in.look.x, in.uv) * a;
    if (bits & laneMixes) != 0u {
        c = over(sampled(in.look.y, in.uv) * (a * in.mix), c);
    }
    if (bits & laneRuns) != 0u {
        var custom = vec4<f32>(in.flow, 0.0, 0.0);
        if (bits & laneFaded) != 0u {
            custom = vec4<f32>(in.flow, in.weight, mark);
        }
        c = over(material(U.LaneStream, in.world, in.params.x * (1.0 - in.mix), 1.0, custom), c);
    }
    if (bits & laneGlints) != 0u {
        c = over(material(U.LaneGlint, in.world, in.params.y * in.mix, 1.0, vec4<f32>(0.0)), c);
    }
    return c;
}
