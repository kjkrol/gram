// What lies on the ground: the board painted flat, the second image, U.Px pixels a cell and
// U.AlbedoSize in all; its water, the third, U.WaterPx pixels a cell in quadrants of the cells'
// size (terrain.WaterLayers), the flows over U.FlowSpan, none where U.WaterPx is 0; the way to
// the shore from the lattice's bottom-right quadrant, the distance over U.ShoreReach.

// albedo is the board's colour at the world point p: blended between the four pixels of the
// board painted flat round it, one picture over the whole board, so a way runs on across the
// cells' borders.
fn albedo(p: vec2<f32>) -> vec3<f32> {
    return imageSrc1Linear(imageSrc1Origin() + p / U.Cell * U.Px).rgb;
}

// shoreAt is the way to the shore from p, blended between the corners of its cell as the tiles'
// glint blends it: its x and y, how far and how near, as SeaGlint takes it.
fn shoreAt(p: vec2<f32>) -> vec4<f32> {
    let g = clamp(p / U.Cell, vec2<f32>(0.0), U.Corners - 1.0);
    let c = imageSrc0Linear(imageSrc0Origin() + U.Corners + g + 0.5);
    let s = vec3<f32>(c.r * 2.0 - 1.0, c.g * 2.0 - 1.0, c.b * U.ShoreReach);
    return vec4<f32>(s.x, s.y, s.z, 1.0 - s.z / U.ShoreReach);
}

// wetAt reports whether water may lie at p: its cell's flag, from the lattice's bottom-left quadrant.
fn wetAt(p: vec2<f32>) -> bool {
    let i = clamp(floor(p / U.Cell), vec2<f32>(0.0), U.Corners - 2.0);
    return imageSrc0At(imageSrc0Origin() + vec2<f32>(0.0, U.Corners.y) + i + 0.5).r > 0.5;
}

// waterAt is the water's layer q (its quadrant, 0 or 1 across and down) at p, blended between the
// four pixels round it, held within the quadrant.
fn waterAt(p: vec2<f32>, q: vec2<f32>) -> vec3<f32> {
    let size = (U.Corners - 1.0) * U.WaterPx;
    let t = clamp(p / U.Cell * U.WaterPx, vec2<f32>(0.5), size - 0.5);
    return imageSrc2Linear(imageSrc2Origin() + q * size + t).rgb;
}

// over is c laid over what is under it: a premultiplied colour, as the composer lays an overlay.
fn over(c: vec4<f32>, under: vec3<f32>) -> vec3<f32> {
    return c.rgb + under * (1.0 - c.a);
}
