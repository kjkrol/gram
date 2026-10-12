// How the world's entities in relief meet the eye, for the billboards and what lies.

// project is where the world point p goes in clip space, sunk under the eye's level as far off as
// it lies, as the ground is.
fn project(p: vec3<f32>) -> vec4<f32> {
    let off = p.xy - U.Eye.xy;
    return U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
}

// nudge moves the world point p towards the eye, so the ground p stands on passes behind it.
fn nudge(p: vec3<f32>) -> vec3<f32> {
    var toward = -U.Look;
    if U.Perspective > 0.5 {
        toward = U.Eye - p;
    }
    return normalize(toward) * spriteLift;
}

// spriteLift is how far towards the eye the depth of what stands or lies is taken, world units.
const spriteLift: f32 = 2.0;
