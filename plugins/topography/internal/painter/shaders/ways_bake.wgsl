// The ways' pieces painted into the layer U.PaintLayer says (paint_ways.wgsl), nothing outside
// the cells painted anew: the albedo as WayPiece.draw lays them, the way's sprite blended over
// its weight where it fades, then turned into its mix's sprite; the kinds' the way's detail
// edged hard; the water's as layWater lays them, the water running and glinting on the way where
// it does, dark where it does not.

@fragment
fn fs_main(in: Lane) -> @location(0) vec4<f32> {
    if masked(in.clip.xy) {
        discard;
    }
    let bits = u32(in.params.w + 0.5);
    let faded = (bits & laneFaded) != 0u;
    let l = layer();
    if l == layerKinds {
        var a = 1.0;
        if faded {
            a = shows(in.weight, 0.0);
        }
        return vec4<f32>(f32(in.look.z) / 255.0 * a, 0.0, 0.0, a);
    }
    var a = 1.0;
    if faded {
        a = shows(in.weight, in.params.z);
    }
    if l == layerAlbedo {
        var c = sampled(in.look.x, in.uv) * a;
        if (bits & laneMixes) != 0u {
            c = over(sampled(in.look.y, in.uv) * (a * in.mix), c);
        }
        return c;
    }
    var c = vec3<f32>(0.0);
    let runs = (bits & laneRuns) != 0u;
    if l == layerFlow && runs {
        c = vec3<f32>(flowOf(in.flow.x), flowOf(in.flow.y), 1.0);
    } else if l == layerShine && runs {
        c = vec3<f32>(in.params.x * (1.0 - in.mix), 0.0, 0.0);
    } else if l == layerMouth && (bits & laneGlints) != 0u {
        c = vec3<f32>(in.params.y * in.mix, 1.0, 0.0);
    }
    return vec4<f32>(c * a, a);
}
