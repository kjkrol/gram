// The tiles' full pass (tiles_screen.wgsl): every pixel of a hex board, and of a square board
// those of the cells something sways on or beside, where the tiles sway; the plain pass has the
// rest. Every cell whose box covers the point is laid there in the order the cells are, its sprite
// stretched over its box leant with the wind (U.Wind) as air.Weather.Sway leans what stands on it,
// faded into the fog as the plain pass fades it.

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> Laid {
    let g = groundAt(frag);
    if g.z < 0.5 {
        discard; // it never meets the ground
    }
    let p = g.xy;
    let here = cellAt(p);
    let hex = U.TileHex > 0.5;
    if !hex {
        let own = folded(here);
        if own.z < 0.5 || plainly(byteOf(own.xy, 0.0, 0.0, 3)) {
            discard; // the plain pass's
        }
    }
    var laid = vec4<f32>(0.0);
    for (var k = 0; k < 9; k++) {
        let n = near(here, k);
        if n.z > 0.5 {
            laid = over(tileAt(n.xy, p), laid);
        }
    }
    laid = litAt(laid, p);
    if laid.a <= 0.0 {
        discard;
    }
    return laidAt(laid, p);
}

// tileAt is what the tile of the cell c lays at p, nothing where its box, leant with the wind,
// does not reach, where its material draws it or where it lies out of sight.
fn tileAt(c: vec2<f32>, p: vec2<f32>) -> vec4<f32> {
    let b = folded(c);
    if b.z < 0.5 {
        return vec4<f32>(0.0);
    }
    let t0 = texelOf(b.xy, 0.0, 0.0);
    let flags = byteIn(t0, 3);
    if (flags & flagShaded) != 0u || levelOf(b.xy) == 0u {
        return vec4<f32>(0.0);
    }
    let base = centreOf(c);
    var mid = base;
    if (flags & flagSways) != 0u {
        mid += leanOf(b.xy, base);
    }
    let lo = mid - U.TileBox * 0.5;
    let hi = mid + U.TileBox * 0.5;
    if p.x < lo.x || p.y < lo.y || p.x >= hi.x || p.y >= hi.y {
        return vec4<f32>(0.0);
    }
    var laid = spriteAt(byteIn(t0, 0) | (byteIn(t0, 1) << 8u), lo, hi, p);
    if U.TileSeen > 0.5 {
        laid = vec4<f32>(mix(laid.rgb, U.Fog * laid.a, fogOf(b.xy)), laid.a);
    }
    return laid;
}
