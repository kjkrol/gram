// The sun on the ground: the way it faces, blended across a cell from the lattice's corners, and
// the terrain's shadows, cast when U.Shadows is over a half — baked (shade.wgsl).

// normal is the way the ground faces at p: the corners' ways, from the lattice's top-right
// quadrant, blended across the cell, so the light runs on smoothly from cell to cell.
fn normal(p: vec2<f32>) -> vec3<f32> {
    let g = clamp(p / U.Cell, vec2<f32>(0.0), U.Corners - 1.0);
    return normalize(imageSrc0Linear(imageSrc0Origin() + vec2<f32>(U.Corners.x, 0.0) + g + 0.5).rgb * 2.0 - 1.0);
}

// sunlit is how much of the sun, the way sun, reaches p: none where the ground towards the sun
// rises over the line to it within shadowSteps steps, fading in over the penumbra — the clearance
// over the ground against how far along the line it is, so level ground stays lit under a low
// sun.
fn sunlit(p: vec3<f32>, sun: vec3<f32>) -> f32 {
    if U.Shadows < 0.5 || sun.z <= 0.0 {
        return 1.0;
    }
    let stride = U.Cell * 0.75;
    let top = U.Low + U.Span;
    var lit = 1.0;
    for (var k = 1; k <= shadowSteps; k++) {
        let t = stride * f32(k);
        let q = p + sun * t;
        if q.z > top {
            break;
        }
        let clearance = q.z - height(q.xy);
        if clearance < 0.0 {
            return 0.0;
        }
        lit = min(lit, clearance / (t * shadowSoft));
    }
    return lit;
}

// How many steps of three quarters of a cell a line towards the sun takes, and how wide the
// shadows' penumbra is per unit towards the sun: a cone of about 4.6°.
const shadowSteps: i32 = 16;
const shadowSoft: f32 = 0.08;
