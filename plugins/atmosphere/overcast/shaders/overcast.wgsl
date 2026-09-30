// The clouds' shadows over a flat world (atmosphere's clouds): a mesh over the viewport of pieces
// cloudPiece pixels across, U.Pieces of them across and down, the clouds' noise worked out at every
// corner — the pixel (x, y) of the viewport, U.ViewSize large, looks from U.RayAt + x·U.RayDX +
// y·U.RayDY along U.RayDir + x·U.RayDDX + y·U.RayDDY (camera.RayField) down to the ground at 0 —
// and each pixel between them darkened as much as the clouds over it take of the sun.

struct Piece {
    @builtin(position) clip: vec4<f32>,
    @location(0) field: f32,
    @location(1) ground: f32, // 1 where the corner's line of sight meets the ground
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
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    var s: Piece;
    s.clip = vec4<f32>(px.x / U.ViewSize.x * 2.0 - 1.0, 1.0 - px.y / U.ViewSize.y * 2.0, 0.0, 1.0);
    if way.z < 0.0 {
        s.field = cloudField((from + way * (-from.z / way.z)).xy);
        s.ground = 1.0;
    }
    return s;
}

@fragment
fn fs_main(s: Piece) -> @location(0) vec4<f32> {
    if s.ground < 0.999 {
        discard; // a line of sight never meeting the ground
    }
    return vec4<f32>(0.0, 0.0, 0.0, cloudShade(cloudCover(s.field)));
}
