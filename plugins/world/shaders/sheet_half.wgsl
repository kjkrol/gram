// The sheets developed (world's sheet_gpu.go): an instance a rectangle of the target's texels
// (x0, y0, x1, y1), U.DevelopTarget texels large, and how far its texels lie from those read for
// them in the first image (x, y). U.DevelopPass 0 copies the first image; 1 and 2 halve the
// sheets at L4, a texel at L5 the four under it read from the albedo, the first image, and the
// relief, the second: 1 the mean of their albedo, 2 the mean height and way of those something
// stands on, a height of zero facing up where nothing does; 3 fills with U.DevelopFill.

struct Half {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) shift: vec2<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) rect: vec4<f32>,
    @location(1) shift: vec4<f32>,
) -> Half {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vid % 6u;
    let t = mix(rect.xy, rect.zw, vec2<f32>(across[k], down[k]));
    return Half(vec4<f32>(t.x / U.DevelopTarget.x * 2.0 - 1.0,
        1.0 - t.y / U.DevelopTarget.y * 2.0, 0.0, 1.0), shift.xy);
}

@fragment
fn fs_main(h: Half) -> @location(0) vec4<f32> {
    if U.DevelopPass > 2.5 {
        return vec4<f32>(U.DevelopFill);
    }
    if U.DevelopPass < 0.5 {
        return imageSrc0At(imageSrc0Origin() + h.clip.xy + h.shift);
    }
    let from = 2.0 * floor(h.clip.xy);
    var albedo = vec4<f32>(0.0);
    var height = 0.0;
    var way = vec3<f32>(0.0);
    var n = 0.0;
    for (var j = 0u; j < 4u; j++) {
        let t = from + vec2<f32>(f32(j % 2u), f32(j / 2u)) + 0.5;
        let a = imageSrc0At(imageSrc0Origin() + t);
        albedo += a;
        if a.a > 0.0 {
            let r = imageSrc1At(imageSrc1Origin() + t);
            let bits = u32(r.r * 255.0 + 0.5) | (u32(r.g * 255.0 + 0.5) << 8u);
            height += unpack2x16float(bits).x;
            let xy = r.ba * 2.0 - 1.0;
            way += vec3<f32>(xy, sqrt(max(1.0 - dot(xy, xy), 0.0)));
            n += 1.0;
        }
    }
    if U.DevelopPass < 1.5 {
        return albedo * 0.25;
    }
    if n == 0.0 {
        return vec4<f32>(0.0, 0.0, 128.0 / 255.0, 128.0 / 255.0);
    }
    let bits = pack2x16float(vec2<f32>(height / n, 0.0));
    let up = normalize(way);
    return vec4<f32>(f32(bits & 0xffu) / 255.0, f32((bits >> 8u) & 0xffu) / 255.0,
        up.xy * 0.5 + 0.5);
}
