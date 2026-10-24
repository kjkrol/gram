// Pixels put in place: RGBA, a byte each, packed four to a u32 in U.Pixels, row by row U.At.x
// wide; src is the pixel's place in the piece.

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let i = u32(floor(src.y)) * u32(U.At.x) + u32(floor(src.x));
    return unpack4x8unorm(U.Pixels[i / 4u][i % 4u]);
}
