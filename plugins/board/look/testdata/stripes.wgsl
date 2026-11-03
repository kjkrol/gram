// Stripes is a test's material: white and black stripes 8 world units across, running down.
fn Stripes(p: vec2<f32>, red: f32, fraction: f32, custom: vec4<f32>) -> vec4<f32> {
    if fract(p.x / 16.0) < 0.5 {
        return vec4<f32>(1.0, 1.0, 1.0, 1.0);
    }
    return vec4<f32>(0.0, 0.0, 0.0, 1.0);
}
