// The grid on the ground, shown where a cell spans U.GridFrom screen pixels or more, in the line
// U.GridLine, premultiplied, U.GridWidth pixels wide.

// edge is how much of p the grid's line covers, 0 to 1: full along a cell's edge, fading out
// over a pixel either side, px and py how far p moves over the ground for a screen pixel across
// and down.
fn edge(p: vec2<f32>, px: vec3<f32>, py: vec3<f32>) -> f32 {
    let g = p / U.Cell;
    let f = min(fract(g), 1.0 - fract(g)) * U.Cell; // how far from the edges along x and y, in world units
    let rate = vec2<f32>(length(vec2<f32>(px.x, py.x)), length(vec2<f32>(px.y, py.y)));
    let pixels = f / max(rate, vec2<f32>(1e-6)); // in screen pixels
    let half = U.GridWidth * 0.5;
    return 1.0 - smoothstep(half - 0.5, half + 0.5, min(pixels.x, pixels.y));
}
