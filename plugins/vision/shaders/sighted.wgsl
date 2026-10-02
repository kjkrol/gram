// The views baked: every observer a block of U.Rings rows of U.Spokes texels in the image drawn
// into, across its cone angle by angle and out along it distance by distance; each texel how much
// of the ground there the observer sees — 1 all, 0 none — walking the line from its eye to the
// ground there every U.March world units: hidden where the ground, sunk U.SightBend·d² under the
// eye's level as far off as it lies, rises over the line, dimmed through cover as the scan dims it
// — the reach its budget, a stretch through cover of τ costing 1/τ a unit — and out of reach past
// the Radius. The quad of an observer hands its texel in src, its eye and reach in custom (x, y,
// the eye's level, the Radius) and its facing and half its angle, radians, in colour.

fn Fragment(dst: vec4<f32>, src: vec2<f32>, color: vec4<f32>, custom: vec4<f32>) -> vec4<f32> {
    let eye = custom.xyz;
    let radius = custom.w;
    let angle = color.x - color.y + src.x / U.Spokes * 2.0 * color.y;
    let dir = vec2<f32>(cos(angle), sin(angle));
    let dist = src.y / U.Rings * radius;
    if dist < 1e-3 {
        return vec4<f32>(1.0);
    }
    let aim = groundAt(eye.xy + dir * dist) - U.SightBend * dist * dist;
    let steps = min(i32(ceil(dist / U.March)), 96);
    var clearance = 1e9;
    var cost = 0.0;
    var last = 0.0;
    for (var k = 1; k < steps; k++) {
        let s = dist * f32(k) / f32(steps);
        let q = eye.xy + dir * s;
        let sunk = U.SightBend * s * s;
        let ray = eye.z + (aim - eye.z) * s / dist;
        clearance = min(clearance, ray - (groundAt(q) - sunk));
        let cover = coverAt(q);
        if cover.x < 0.999 && ray >= cover.y - sunk && ray <= cover.z - sunk {
            if cover.x <= 0.0 {
                return vec4<f32>(0.0, 0.0, 0.0, 1.0);
            }
            cost += (s - last) * (1.0 / cover.x - 1.0);
        }
        last = s;
    }
    let seen = smoothstep(-sightSoft, 0.0, clearance) * (1.0 - smoothstep(radius - sightSoft, radius, dist + cost));
    return vec4<f32>(seen, seen, seen, 1.0);
}

// sightSoft is over how many world units sight fades out where the ground rises over the line, or
// its reach runs out.
const sightSoft: f32 = 1.0;
