// How the world's entities in relief meet the eye and stand in the light, for the billboards,
// the bodies and what lies.

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

// givenWay reports whether the target's pixel px of what takes as taking says (render.Taking,
// with the library's givesWay past its bits), lying at depth, gives way: something uncovered
// lies farther there, within the rectangle of the target the uncovered lie in (U.UncoveredIn).
fn givenWay(taking: f32, px: vec2<f32>, depth: f32) -> bool {
    if U.Uncovering < 0.5 || !takes(taking, givesWay) {
        return false;
    }
    if px.x < U.UncoveredIn.x || px.y < U.UncoveredIn.y || px.x > U.UncoveredIn.z
        || px.y > U.UncoveredIn.w {
        return false;
    }
    return givesWayAt(px, depth);
}

// shaded is how much of its light reaches the point a share up of what stands at feet over ground,
// sun of the light the sun's, taking the shade takes says: the shadows the ground holds and the
// clouds take some of the sun's, the clouds some of the sky's too.
fn shaded(feet: vec4<f32>, ground: f32, up: f32, sun: f32, wide: f32, takes: f32) -> f32 {
    let over = feet.z - ground + up * feet.w;
    let soft = 0.25 * feet.w;
    var reached = 1.0;
    if takes(takes, takesRelief) {
        reached = min(reached, groundShadeAt(feet.xy, over, soft));
    }
    if takes(takes, takesBlocks) {
        reached = min(reached, standingShadeAt(feet.xy, over, soft));
    }
    var clouds = 1.0;
    if U.Cover > 0.0 && U.CloudsShadeFigures > 0.5 && takes(takes, takesClouds) {
        clouds = 1.0 - cloudShade(cloudsShadeAt(vec3<f32>(feet.xy, ground + over), wide));
    }
    return (1.0 - sun * (1.0 - reached)) * clouds;
}
