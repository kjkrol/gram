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

// outlineDark is how much an outline darkens what it is drawn on.
const outlineDark: f32 = 0.45;

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
