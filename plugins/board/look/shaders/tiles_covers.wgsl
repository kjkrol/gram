// The covers on a square board's cells the plain pass lays (look's tiles.go), an instance a cell
// and a cover lying on it or a neighbour (its column, its row and which cover): the cover over
// each quarter of the cell's tile, weighed at the quarter's corners (weightAt), shown where the
// weight comes over a half, fading in over coverSoft of it (draw.Quarters); laid over the tiles,
// U.TileShift along, through U.TileViewProj; none on a cell of a sector seen but coarsely.

struct Part {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) weight: f32,
    @location(3) @interpolate(flat) look: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>) -> Part {
    var out: Part;
    out.clip = away;
    let b = at.xy;
    let i = u32(at.z + 0.5);
    let flags = byteOf(b, 0.0, 0.0, 3);
    if levelOf(b) != 1u || (U.TileSways > 0.5 && !plainly(flags)) {
        return out; // out of sight, or the full pass's
    }
    // the quarter, and its corner: two triangles of top-left, top-right, bottom-left, then
    // top-right, bottom-left, bottom-right
    let q = vid / 6u;
    var k = vid % 6u;
    if k >= 3u {
        k -= 2u;
    }
    let point = vec2<f32>(f32(q % 2u + k % 2u), f32(q / 2u + k / 2u)); // 0, 1, 2 across, down
    out.weight = weightAt(b, i, point);
    let base = centreOf(b);
    let lo = base + U.TileShift - U.TileBox * 0.5;
    out.uv = point * 0.5;
    out.world = lo + out.uv * U.TileBox;
    out.look = rectOf(coverOf(i));
    out.clip = U.TileViewProj * vec4<f32>(out.world, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(in: Part) -> @location(0) vec4<f32> {
    let a = smoothstep(0.5 - coverSoft, 0.5 + coverSoft, in.weight);
    return litAt(sampled(in.look, in.uv) * a, in.world);
}
