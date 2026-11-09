// The tops the corners of the sheets' lattices (terrain's photo.go) stand at, at LEVEL, L4 or L5:
// an instance a sector, the texels of its corners among the tops (x0, y0, x1, y1), U.SheetQuads + 1
// a side, and its box (x0, y0, x1, y1). A corner's texel holds the highest top the relief (the
// third image, U.SheetPx texels a world unit, a float16 in red and green) holds within
// U.SheetReach texels of it, a float16 in red and green and 1 in blue, or 0 in blue where nothing
// stands so near, sought at L4 first in the albedo at L5 (the fourth image); its alpha 1, so it
// lays over what was there.

struct Corner {
    @builtin(position) clip: vec4<f32>,
    @location(0) @interpolate(flat) first: vec2<f32>,
    @location(1) @interpolate(flat) box: vec4<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) rect: vec4<f32>,
    @location(1) box: vec4<f32>,
) -> Corner {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vid % 6u;
    let t = mix(rect.xy, rect.zw, vec2<f32>(across[k], down[k]));
    return Corner(vec4<f32>(t.x / D.target.x * 2.0 - 1.0, 1.0 - t.y / D.target.y * 2.0, 0.0, 1.0),
        rect.xy, box);
}

@fragment
fn fs_main(g: Corner) -> @location(0) vec4<f32> {
    let c = (floor(g.clip.xy) - g.first) / f32(u32(U.SheetQuads));
    let top = topNear(mix(g.box.xy, g.box.zw, c));
    if top < -1.0e38 {
        return vec4<f32>(0.0, 0.0, 0.0, 1.0);
    }
    let bits = pack2x16float(vec2<f32>(top, 0.0));
    return vec4<f32>(f32(bits & 0xffu) / 255.0, f32((bits >> 8u) & 0xffu) / 255.0, 1.0, 1.0);
}

// topNear is the highest top the relief holds within U.SheetReach texels of p, below -1e38 where
// nothing stands so near: a quad over a thing narrower than it stands at its top at every corner.
fn topNear(p: vec2<f32>) -> f32 {
    let at = floor(p * U.SheetPx);
    let reach = i32(U.SheetReach);
    var top = -3.0e38;
    if LEVEL == 4u && !coarselyNear(at, reach) {
        return top;
    }
    for (var dy = -reach; dy <= reach; dy++) {
        for (var dx = -reach; dx <= reach; dx++) {
            let t = at + vec2<f32>(f32(dx), f32(dy)) + 0.5;
            if imageSrc1At(imageSrc1Origin() + t).a > 0.0 {
                let r = imageSrc2At(imageSrc2Origin() + t);
                let bits = u32(r.r * 255.0 + 0.5) | (u32(r.g * 255.0 + 0.5) << 8u);
                top = max(top, unpack2x16float(bits).x);
            }
        }
    }
    return top;
}

// coarselyNear reports whether something stands within reach texels at L4 of the texel at, as
// the albedo at L5, half as fine, has it: most corners have nothing near, sought so at a quarter
// of the reads.
fn coarselyNear(at: vec2<f32>, reach: i32) -> bool {
    let c = floor(at * 0.5);
    let r = reach / 2 + 1;
    for (var dy = -r; dy <= r; dy++) {
        for (var dx = -r; dx <= r; dx++) {
            let t = c + vec2<f32>(f32(dx), f32(dy)) + 0.5;
            if imageSrc3At(imageSrc3Origin() + t).a > 0.0 {
                return true;
            }
        }
    }
    return false;
}
