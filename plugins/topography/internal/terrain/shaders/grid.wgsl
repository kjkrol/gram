// The grid on the ground, shown where a cell spans U.GridFrom screen pixels or more.

// edge is how dark the grid is at p, 0 to 1: full on a cell's edge, fading out over a pixel, px
// and py how far p moves over the ground for a screen pixel across and down.
fn edge(p: vec2<f32>, px: vec3<f32>, py: vec3<f32>) -> f32 {
    let g = p / U.Cell;
    let f = min(fract(g), 1.0 - fract(g)) * U.Cell; // how far from the edges along x and y, in world units
    let rate = vec2<f32>(length(vec2<f32>(px.x, py.x)), length(vec2<f32>(px.y, py.y)));
    let pixels = f / max(rate, vec2<f32>(1e-6)); // in screen pixels
    return 1.0 - smoothstep(0.0, 0.75, min(pixels.x, pixels.y));
}
