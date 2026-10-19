// The light on the board's still seen from above (board's groundShade), one triangle over the
// viewport scaling what is drawn there (render.DrawMeshOptions.Multiply): the pixel (x, y) of it
// looks from U.RayAt + x·U.RayDX + y·U.RayDY along U.RayDir + x·U.RayDDX + y·U.RayDDY
// (camera.RayField) down to the ground at 0. The still is drawn in the sun's light on level
// ground; here the shadows the frame holds (render.GroundShade) take the sun's share of it away
// and the lamps add theirs, as the cell takes them: its bits (render.Taking) in the red of the
// first image, a pixel a cell of U.Cells cells U.CellSize across, every bit off the board or
// without the image (U.Cells.x of 0).

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
    let sun = lightWay();
    let sunlit = U.LightColor * max(sun.z, 0.0) * U.LightStrength;
    let light = U.Ambience + sunlit; // the still's light
    var k = vec3<f32>(1.0);
    if takes(taken, takesRelief) || takes(taken, takesBlocks) {
        let lit = groundShadeOf(p).r;
        k *= 1.0 - sunlit / max(light, vec3<f32>(1e-4)) * (1.0 - lit);
    }
    if takes(taken, takesLamps) {
        k *= 1.0 + lampLightAt(vec3<f32>(p, 0.0)) / max(light, vec3<f32>(1e-4));
    }
    return vec4<f32>(k, 1.0);
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
