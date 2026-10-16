// The clouds' shadows over a flat board's ground: a mesh over the viewport of pieces
// cloudPiece pixels across, U.Pieces of them across and down, the clouds' layers over the ground
// worked out at every corner (cloudLowOver, cloudHighOver) — the pixel (x, y) of the viewport,
// U.ViewSize large, looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir + x·U.RayDDX +
// y·U.RayDDY (camera.RayField) down to the ground at 0 — and each pixel between them darkened as
// much as the clouds over it take of the sun.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) field: vec4<f32>, // the low layer's noise and edge, then the high's
    @location(1) ground: f32, // 1 where the corner's line of sight meets the ground
    @location(2) low: vec4<f32>, // the layers' air masses
    @location(3) high: vec4<f32>,
}

const cloudPiece: f32 = 8.0;

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Piece {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let n = vid / 6u;
    let pieces = u32(U.Pieces.x);
    let corner = vec2<f32>(f32(n % pieces) + across[vid % 6u], f32(n / pieces) + down[vid % 6u]);
    let px = min(corner * cloudPiece, U.ViewSize);
    var s = Piece();
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    let at = groundAt(px);
    if at.z > 0.5 {
        // the noise as fine as a piece shows it
        let next = groundAt(px + vec2<f32>(cloudPiece, 0.0));
        let pixel = distance(at.xy, next.xy);
        let low = cloudLowOver(vec3<f32>(at.xy, 0.0), pixel);
        let high = cloudHighOver(vec3<f32>(at.xy, 0.0), pixel);
        s.field = vec4<f32>(low.field, high.field);
        s.low = low.mass;
        s.high = high.mass;
        s.ground = 1.0;
    }
    return s;
}

// groundAt is where the line of sight of the viewport's pixel px meets the ground (xy), z 1 where
// it does.
fn groundAt(px: vec2<f32>) -> vec3<f32> {
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    if way.z >= 0.0 {
        return vec3<f32>(0.0);
    }
    return vec3<f32>((from + way * (-from.z / way.z)).xy, 1.0);
}

@fragment
fn fs_main(s: Piece) -> @location(0) vec4<f32> {
    if s.ground < 0.999 {
        discard; // a line of sight never meeting the ground
    }
    let low = cloudOverCover(CloudOver(s.field.xy, s.low));
    let high = cloudOverCover(CloudOver(s.field.zw, s.high));
    return vec4<f32>(0.0, 0.0, 0.0, cloudShade(1.0 - (1.0 - low) * (1.0 - high)));
}
