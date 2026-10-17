// The library of the composer's shader and of every shader built on it (ShaderSourceWith): the
// composer's own uniforms — U.Toward the way towards the eye, U.Clock the time in seconds, U.Pixel
// how many world units a pixel spans, U.Fog the colour what lies far off in the air turns to, the
// ground's heights (U.GroundLattice, U.GroundSpan: render.GroundHeights) — and the helpers the
// materials call. Every other uniform is a material's, declared with its
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
    return seenAt(span, U.Pixel);
}

// seenAt is seen where a pixel spans pixel world units: for what knows it pixel by pixel, as a
// perspective draws the far off in larger ones.
fn seenAt(span: f32, pixel: f32) -> f32 {
    return smoothstep(finest, 2.0 * finest, span / max(pixel, 1e-3));
}

// finest is how many pixels long the finest pattern drawn is.
const finest: f32 = 2.0;

const pi: f32 = 3.14159265;

// groundHeightAt is the ground's height at the world point p as the frame's GroundHeights hold it
// (render.GroundHeights), blended between the lattice's corners, the edge's past it; groundNone
// in a frame without them.
fn groundHeightAt(p: vec2<f32>) -> f32 {
    if U.GroundLattice.w < 0.5 {
        return groundNone;
    }
    let g = clamp(p / U.GroundLattice.z, vec2<f32>(0.0), U.GroundLattice.xy - 1.0001);
    let i = floor(g);
    let f = g - i;
    let top = mix(groundCorner(i), groundCorner(i + vec2<f32>(1.0, 0.0)), f.x);
    let bottom = mix(groundCorner(i + vec2<f32>(0.0, 1.0)), groundCorner(i + 1.0), f.x);
    return mix(top, bottom, f.y);
}

// groundHighest is the highest the frame's GroundHeights reach; groundNone without them.
fn groundHighest() -> f32 {
    if U.GroundLattice.w < 0.5 {
        return groundNone;
    }
    return U.GroundSpan.z + U.GroundSpan.w;
}

fn groundCorner(i: vec2<f32>) -> f32 {
    let c = textureLoad(GroundHeights, vec2<i32>(U.GroundSpan.xy + i), 0);
    return U.GroundSpan.z + (c.r * 255.0 * 256.0 + c.g * 255.0) / 65535.0 * U.GroundSpan.w;
}

const groundNone: f32 = -3.0e38;

// groundShadeOf is the frame's GroundShade at the world point p (render.GroundShade), each band
// faded from the shade before as far as it has come: the sun everywhere, (1, 0, 1, 1), in a frame
// without it.
fn groundShadeOf(p: vec2<f32>) -> vec4<f32> {
    if U.GroundShadeArea.z <= 0.0 {
        return vec4<f32>(1.0, 0.0, 1.0, 1.0);
    }
    let at = (p - U.GroundShadeArea.xy) / U.GroundShadeArea.zw;
    let uv = clamp(at, vec2<f32>(0.0), vec2<f32>(1.0));
    let now = textureSampleLevel(GroundShade, linear, uv, 0.0);
    let was = textureSampleLevel(GroundShadeWas, linear, uv, 0.0);
    let band = min(i32(uv.y / max(U.GroundShadeSpan.y, 1e-6)), 15);
    var mixes = U.GroundShadeMix0;
    if band >= 12 {
        mixes = U.GroundShadeMix3;
    } else if band >= 8 {
        mixes = U.GroundShadeMix2;
    } else if band >= 4 {
        mixes = U.GroundShadeMix1;
    }
    let k = band % 4;
    let pick = vec4<f32>(f32(k == 0), f32(k == 1), f32(k == 2), f32(k == 3));
    return mix(was, now, dot(mixes, pick));
}

// groundShadeAt is how much of the sun reaches the point over world units over the ground at p
// past the relief: none below how high its shadow reaches there, fading in over soft under it.
fn groundShadeAt(p: vec2<f32>, over: f32, soft: f32) -> f32 {
    let g = groundShadeOf(p).g;
    let reach = g * g * U.GroundShadeSpan.x;
    return smoothstep(reach - max(soft, 1e-3), reach, over);
}

// standingShadeAt is how much of the sun reaches the point over world units over the ground at p
// past the blocks standing on it: as much as at the ground below how high their shadows reach
// there, all of it over, fading in over soft under it.
fn standingShadeAt(p: vec2<f32>, over: f32, soft: f32) -> f32 {
    let s = groundShadeOf(p);
    let reach = (1.0 - s.b) * (1.0 - s.b) * U.GroundShadeSpan.x;
    return mix(s.a, 1.0, smoothstep(reach - max(soft, 1e-3), reach, over));
}
