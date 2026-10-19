// The light on the board's still seen from above (board's groundShade), one triangle over the
// viewport scaling what is drawn there (render.DrawMeshOptions.Multiply): the pixel (x, y) of it
// looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir + x·U.RayDDX + y·U.RayDDY
// (camera.RayField) down to the ground at 0. The still is drawn as it is; here it is lit by the
// ambient light, the distant light on level ground less the shadows the frame holds
// (render.GroundShade) and the lamps', as the cell takes them: its bits (render.Taking) in the
// red of the first image, a pixel a cell of U.Cells cells U.CellSize across, every bit off the
// board or without the image (U.Cells.x of 0); a cell taking none stays as drawn.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> @location(0) vec4<f32> {
    let px = frag.xy - U.ViewAt;
    let from = U.RayAt + px.x * U.RayDX + px.y * U.RayDY;
    let way = U.RayDir + px.x * U.RayDDX + px.y * U.RayDDY;
    if way.z >= 0.0 {
        discard; // it never meets the ground
    }
    let p = (from + way * (-from.z / min(way.z, -1e-6))).xy;
    let taken = cellTakesAt(p);
    if u32(taken + 0.5) == 0u {
        return vec4<f32>(1.0); // as drawn
    }
    let way = lightWay();
    var lit = 1.0;
    if takes(taken, takesRelief) || takes(taken, takesBlocks) {
        lit = groundShadeOf(p).r;
    }
    var light = U.Ambience + U.LightColor * max(way.z, 0.0) * U.LightStrength * lit;
    if takes(taken, takesLamps) {
        light += lampLightAt(vec3<f32>(p, 0.0));
    }
    return vec4<f32>(light, 1.0);
}

// cellTakesAt is the shade the cell at p takes, every bit off the board or without the cells'
// image.
fn cellTakesAt(p: vec2<f32>) -> f32 {
    let all = f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    if U.Cells.x < 0.5 {
        return all;
    }
    let i = floor(p / U.CellSize);
    if i.x < 0.0 || i.y < 0.0 || i.x >= U.Cells.x || i.y >= U.Cells.y {
        return all;
    }
    return imageSrc0At(imageSrc0Origin() + i + 0.5).r * 255.0;
}
