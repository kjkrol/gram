// The sun on the ground: the way it faces, blended across a cell from the lattice's corners.

// normal is the way the ground faces at p: the corners' ways, from the lattice's top-right
// quadrant, blended across the cell, so the light runs on smoothly from cell to cell.
fn normal(p: vec2<f32>) -> vec3<f32> {
    let g = clamp(p / U.Cell, vec2<f32>(0.0), U.Corners - 1.0);
    return normalize(imageSrc0Linear(imageSrc0Origin() + vec2<f32>(U.Corners.x, 0.0) + g + 0.5).rgb * 2.0 - 1.0);
}
