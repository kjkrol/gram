// The tiles' plain pass (tiles_screen.wgsl): each cell of a square board nothing sways on or
// beside and no material lies over, drawn by its sprite over its own box, faded into U.Fog as far into the fog before the
// sight's end as its middle lies (U.TileFogging: from where, where it begins and ends); nothing of
// a cell whose middle lies in a sector hidden.

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> Laid {
    let g = groundAt(frag);
    if g.z < 0.5 {
        discard; // it never meets the ground
    }
    let p = g.xy;
    let here = floor(p / U.TileBox);
    let b = folded(here);
    if b.z < 0.5 {
        discard;
    }
    let t0 = texelOf(b.xy, 0.0, 0.0);
    if !plainly(byteIn(t0, 3)) || levelOf(b.xy) == 0u {
        discard; // the full pass's or the material's, or out of sight
    }
    let lo = here * U.TileBox;
    let hi = lo + U.TileBox;
    var laid = spriteAt(byteIn(t0, 0) | (byteIn(t0, 1) << 8u), lo, hi, p);
    if U.TileSeen > 0.5 {
        laid = vec4<f32>(mix(laid.rgb, U.Fog * laid.a, fogOf(b.xy)), laid.a);
    }
    laid = vec4<f32>(laid.rgb * U.TileTint * lightAt(f32(byteIn(t0, 2)), p), laid.a);
    if laid.a <= 0.0 {
        discard;
    }
    return laidAt(laid, p);
}
