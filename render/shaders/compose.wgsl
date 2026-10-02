// The composer's Fragment, after the library and the materials.

// Fragment is the sheet's texel times the vertex colour, faded or outlined at the edges an item
// asks for. Per edge, custom holds 1 plus the distance to it in units of its fade (the piece fades
// out there), minus 1 minus the distance in pixels (the piece is outlined there), or 0 (nothing).
// A blended sprite (the last custom over 50000, 100000 plus how soft; a fade's stays under 10000)
// holds its weight in alpha: it shows where the weight is over a half, glazed as far as 1 plus its
// opacity in the third custom says. An overlay (alpha over 1.5, 2 plus twice its material's number
// plus a fraction 0 to 1 it reads) is its material's to work out: where it lies in the world in
// green and blue, what the material reads in red, the fraction and custom — see material.
fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    if color.a > 1.5 {
        let k = floor((color.a - 2.0) / 2.0);
        return material(k, color.gb, color.r, color.a - 2.0 - 2.0 * k, custom);
    }
    if custom.w > 50000.0 {
        // a blended sprite: shown where its weight is over a half, fading in round it, as far as
        // its opacity (1 plus it in the third custom) where it glazes
        var a = blended(color.a, custom.w);
        if custom.z > 0.5 {
            a *= custom.z - 1.0;
        }
        let c = imageSrc0At(src);
        return vec4<f32>(c.rgb * color.rgb * a, c.a * a);
    }
    let f = faded(custom);
    let outlined = step(custom, vec4<f32>(-0.5));
    let near = outlined * (1.0 - smoothstep(vec4<f32>(0.0), vec4<f32>(1.0), clamp(-custom - 1.0, vec4<f32>(0.0), vec4<f32>(1.0))));
    let edge = max(max(near.x, near.y), max(near.z, near.w));

    // a plain sprite far off in the air (alpha 1 plus a fogSpan of its fog) turns to the Fog
    let fog = clamp((color.a - 1.0) / fogSpan, 0.0, 1.0);
    let c = imageSrc0At(src) * vec4<f32>(color.rgb, min(color.a, 1.0)) * f;
    let rgb = c.rgb * (1.0 - outlineDark * edge);
    return vec4<f32>(mix(rgb, U.Fog * c.a, fog), c.a);
}

// fogSpan is how far over 1 a plain sprite's alpha goes for all the fog there is (Frame.Fog).
const fogSpan: f32 = 0.49;
