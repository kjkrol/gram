// The ways' pieces painted into the albedo (paint_ways.wgsl), as WayPiece.draw lays them: the
// way's sprite, blended over its weight where it fades, then turned into its mix's sprite as far
// as it mixes; nothing outside the cells painted anew.

@fragment
fn fs_main(in: Lane) -> @location(0) vec4<f32> {
    if masked(in.clip.xy) {
        discard;
    }
    let bits = u32(in.params.w + 0.5);
    var a = 1.0;
    if (bits & laneFaded) != 0u {
        a = shows(in.weight, in.params.z);
    }
    var c = sampled(in.look.x, in.uv) * a;
    if (bits & laneMixes) != 0u {
        c = over(sampled(in.look.y, in.uv) * (a * in.mix), c);
    }
    return c;
}
