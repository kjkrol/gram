// The cells drawn by their material (tiles_screen.wgsl), over all the rest of the tiles, as each
// cell's material works it out over its box (render.Overlay: World the box, Red 1, Custom its
// middle and half its width).

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> @location(0) vec4<f32> {
    let g = groundAt(frag);
    if g.z < 0.5 {
        discard; // it never meets the ground
    }
    let p = g.xy;
    let here = cellAt(p);
    var laid = vec4<f32>(0.0);
    if U.TileHex > 0.5 {
        for (var k = 0; k < 7; k++) {
            laid = over(materialAt(near(here, k).xy, p), laid);
        }
    } else {
        laid = materialAt(here, p);
    }
    if laid.a <= 0.0 {
        discard;
    }
    return laid;
}

// materialAt is the material of the cell c over its box at p, nothing where c is drawn by its
// sprite, its box does not reach or it lies out of sight.
fn materialAt(c: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let b = folded(c);
    if b.z < 0.5 || (byteOf(b.xy, 0.0, 0.0, 3) & flagShaded) == 0u || levelOf(b.xy) == 0u {
        return vec4<f32>(0.0);
    }
    let mid = centreOf(c);
    let half = U.TileBox * 0.5;
    let lo = mid - half;
    let hi = mid + half;
    if p.x < lo.x || p.y < lo.y || p.x >= hi.x || p.y >= hi.y {
        return vec4<f32>(0.0);
    }
    let k = f32(byteOf(b.xy, 1.0, 1.0, 2));
    return material(k, p, 1.0, 0.0, vec4<f32>(mid, half.x, 0.0));
}
