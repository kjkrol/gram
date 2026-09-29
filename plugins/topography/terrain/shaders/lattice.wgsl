// The ground's mesh on the GPU, after the composer's library and every material
// (render.NewMeshShaderWith): the lattice of the ground's heights. The lattice, U.Corners.x by
// U.Corners.y corners a U.Cell apart, lies in the first image's quadrants of that size: the
// heights top-left, U.Low up to U.Low+U.Span in red and green as 16 bits; the way the ground faces
// top-right as (n+1)/2; the way to the shore bottom-right.

// height is the ground's height at the world point p as a tile is drawn: two flat triangles split
// along the diagonal whose corners stand nearer in height (topography.Relief.GroundAt).
fn height(p: vec2<f32>) -> f32 {
    let g = clamp(p / U.Cell, vec2<f32>(0.0), U.Corners - 1.0001);
    let i = floor(g);
    return drawn(g - i, corner(i), corner(i + vec2<f32>(1.0, 0.0)), corner(i + vec2<f32>(0.0, 1.0)), corner(i + vec2<f32>(1.0, 1.0)));
}

// drawn is the height at f, 0 to 1 across and down a cell whose corners stand at h0 to h3, as the
// cell is drawn: two flat triangles split along the diagonal whose corners stand nearer.
fn drawn(f: vec2<f32>, h0: f32, h1: f32, h2: f32, h3: f32) -> f32 {
    if abs(h0 - h3) < abs(h1 - h2) {
        if f.x >= f.y {
            return h0 + f.x * (h1 - h0) + f.y * (h3 - h1);
        }
        return h0 + f.y * (h2 - h0) + f.x * (h3 - h2);
    }
    if f.x + f.y <= 1.0 {
        return h0 + f.x * (h1 - h0) + f.y * (h2 - h0);
    }
    return h3 + (1.0 - f.x) * (h2 - h3) + (1.0 - f.y) * (h1 - h3);
}

// corner is the height at the lattice corner i, decoded from 16 bits of the lattice's top-left
// quadrant.
fn corner(i: vec2<f32>) -> f32 {
    let c = imageSrc0At(imageSrc0Origin() + i + 0.5);
    return U.Low + (c.r * 255.0 * 256.0 + c.g * 255.0) / 65535.0 * U.Span;
}
