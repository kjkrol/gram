// What the buildings' programs share: where a point is drawn, which of two walls shows, the air
// over a point, a building's own chance and its windows.

fn clipOf(p: vec3<f32>) -> vec4<f32> {
    let from = p.xy - U.SolidEye.xy;
    return U.SolidViewProj * vec4<f32>(p.xy, p.z - U.SolidBend * dot(from, from), 1.0);
}

// facing is 1 where the wall standing out off the part's middle shows its outer face, -1 where
// the one across the part does: of two walls across each other one at most is seen
fn facing(mid: vec2<f32>, out: vec2<f32>, run: vec2<f32>, foot: f32, top: f32) -> f32 {
    let p0 = clipOf(vec3<f32>(mid + out - run, foot)).xyw;
    let p1 = clipOf(vec3<f32>(mid + out + run, foot)).xyw;
    let p2 = clipOf(vec3<f32>(mid + out - run, top)).xyw;
    if dot(p0, cross(p1, p2)) > 0.0 {
        return 1.0;
    }
    return -1.0;
}

fn chance(p: vec3<f32>) -> f32 {
    return fract(sin(dot(p, vec3<f32>(12.9898, 78.233, 37.719))) * 43758.5453);
}

// hazeDepth is how much air lies between the eye and the point p, world units, as the ground's
// shader has it: its distance from the eye, or seen from above (U.SolidHaze.y) the air over p up
// to that height along the lines of sight U.SolidHazeWay.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.SolidHaze.y <= 0.0 {
        return distance(U.SolidEye, p);
    }
    return max(U.SolidHaze.y - p.z, 0.0) / max(-U.SolidHazeWay.z, 0.05);
}

// paneOf is the window the point along a wall wide either side of its middle and up over the
// ground the storeys are counted from lies in under the eaves: its bay and its storey, and 1 in
// a pane, 0 between them. A pane a bay a storey, the bays counted from the wall's middle, none cut
// by its corners, as much of a bay and of a storey as U.SolidPane says.
fn paneOf(along: f32, up: f32, wide: f32, bay: f32, storey: f32, eaves: f32) -> vec3<f32> {
    if bay <= 0.0 || up <= 0.0 || up >= eaves {
        return vec3<f32>(0.0);
    }
    let bays = max(floor(2.0 * wide / bay), 1.0);
    let x = (along + wide) / (2.0 * wide / bays);
    let y = up / storey;
    let inX = abs(fract(x) - 0.5) < U.SolidPane.x / 2.0;
    let inY = fract(y) > U.SolidPane.y && fract(y) < U.SolidPane.z;
    if inX && inY && floor(y) * storey + storey <= eaves + 1e-3 {
        return vec3<f32>(floor(x), floor(y), 1.0);
    }
    return vec3<f32>(floor(x), floor(y), 0.0);
}

// doorOf reports whether the point p of a wall facing way out lies in the door door says: its
// middle on the wall's foot, half its width and its height over the floor at floor
fn doorOf(p: vec3<f32>, way: vec2<f32>, door: vec4<f32>, floor: f32) -> bool {
    if door.z <= 0.0 {
        return false;
    }
    let off = p.xy - door.xy;
    let across = abs(dot(off, way));
    let along = abs(dot(off, vec2<f32>(-way.y, way.x)));
    return across < 0.5 && along < door.z && p.z >= floor - 0.01 && p.z < floor + door.w;
}
