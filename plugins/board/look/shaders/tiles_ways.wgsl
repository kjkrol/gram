// The ways across a flat board's cells (look's tiles.go), an instance a cell a way runs across
// (its column and row): a band from the cell's middle out towards each neighbour its links name,
// as wide as the way, then the hub, an octagon of the band's width round the middle, each cut out
// of the way's sprite laid over the cell's box and as opaque as the way has not faded; laid over
// the tiles, U.TileShift along, through U.TileViewProj; none across a cell of a sector seen but
// coarsely.

struct Band {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec2<f32>,
    @location(1) uv: vec2<f32>,
    @location(2) @interpolate(flat) look: vec4<f32>,
    @location(3) @interpolate(flat) keep: f32,
}

// a band's six vertices (two triangles of its corners as they are fanned: middle and out on one
// side, then out and middle on the other), the eight bands, then the hub's six triangles fanned
const bandVertices: u32 = 48u;

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) at: vec4<f32>) -> Band {
    var out: Band;
    out.clip = away;
    let b = at.xy;
    if levelOf(b) != 1u {
        return out;
    }
    let t1 = texelOf(b, 1.0, 0.0);
    let t2 = texelOf(b, 0.0, 1.0);
    let links = byteIn(t1, 2);
    let half = f32(byteIn(t2, 0) | (byteIn(t2, 1) << 8u)) / 64.0 * 0.5;
    let mid = centreOf(b) + U.TileShift;
    var w = mid;
    if vid < bandVertices {
        let piece = vid / 6u;
        if ((links >> piece) & 1u) == 0u {
            return out;
        }
        let st = stepOf(i32(piece));
        let u = st / length(st);
        let side = vec2<f32>(-u.y, u.x) * half;
        let m = mid + st * 0.5;
        var k = vid % 6u; // the fan's triangles: corners 0, 1, 2, then 0, 2, 3
        if k >= 3u {
            k = select(k - 2u, 0u, k == 3u);
        }
        if k == 0u {
            w = mid + side;
        } else if k == 1u {
            w = m + side;
        } else if k == 2u {
            w = m - side;
        } else {
            w = mid - side;
        }
    } else {
        let k = vid - bandVertices;
        let corner = select(0u, k / 3u + k % 3u, k % 3u != 0u);
        let a = f32(corner) * 0.7853982;
        w = mid + half * vec2<f32>(cos(a), sin(a));
    }
    out.world = w;
    let lo = mid - U.TileBox * 0.5;
    out.uv = clamp((w - lo) / U.TileBox, vec2<f32>(0.0), vec2<f32>(1.0));
    out.look = rectOf(byteIn(t1, 0) | (byteIn(t1, 1) << 8u));
    out.keep = 1.0 - f32(byteIn(t2, 2)) / 255.0;
    out.clip = U.TileViewProj * vec4<f32>(w, 0.0, 1.0);
    return out;
}

@fragment
fn fs_main(in: Band) -> @location(0) vec4<f32> {
    return litAt(sampled(in.look, in.uv) * in.keep, in.world);
}
