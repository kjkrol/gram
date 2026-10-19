// An image drawn as a textured quad blended between texels, times the vertex colour, fading out
// towards each edge as its custom asks (faded): a DrawImage with a Fade.

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    return imageSrc0Linear(src) * color * faded(custom);
}
