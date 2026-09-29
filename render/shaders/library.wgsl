// The library of the composer's shader and of every shader built on it (ShaderSourceWith): the
// composer's own uniforms — U.Toward the way towards the eye, U.Clock the time in seconds, U.Pixel
// how many world units a pixel spans, U.Fog the colour what lies far off in the air turns to —
// and the helpers the materials call. Every other uniform is a material's, declared with its
// source (render.RegisterMaterials) and set by a source with render.Frame.Uniform.

// faded is how much of a sprite shows by the fades its custom asks for: 1 where it asks none.
fn faded(custom: vec4<f32>) -> f32 {
    let fade = smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(custom - 1.0, vec4<f32>(0.0), vec4<f32>(1.0)));
    let f = mix(vec4<f32>(1.0), fade, step(vec4<f32>(0.5), custom));
    return f.x * f.y * f.z * f.w;
}

// blended is how much of a blended sprite shows at weight, its mark 100000 plus how soft it is.
fn blended(weight: f32, mark: f32) -> f32 {
    let soft = mark - 100000.0;
    return smoothstep(0.5 - soft, 0.5 + soft, weight);
}

// seen is how much of a pattern span world units long shows: all of it over a few pixels long,
// none of it where it would flicker, finer than a pixel or two.
fn seen(span: f32) -> f32 {
    return smoothstep(finest, 2.0 * finest, span / max(U.Pixel, 1e-3));
}

// outlineDark is how much an outline darkens what it is drawn on.
const outlineDark: f32 = 0.45;

// finest is how many pixels long the finest pattern drawn is.
const finest: f32 = 2.0;

const pi: f32 = 3.14159265;
