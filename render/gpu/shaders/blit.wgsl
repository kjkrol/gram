// An image drawn as a textured quad: src in the source texture's pixels, sampled nearest or
// blended between texels (U.filter.x over a half), times the vertex colour.

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let size = vec2<f32>(textureDimensions(image0));
    if U.filter.x > 0.5 {
        return textureSampleLevel(image0, linear, src / size, 0.0) * color;
    }
    return textureSampleLevel(image0, nearest, src / size, 0.0) * color;
}
