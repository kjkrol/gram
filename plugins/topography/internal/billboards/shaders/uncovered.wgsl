// What the world's entities in relief have uncovered (topography's sprites, world's
// Slot.Uncovered), drawn before the cutouts into the depth what gives way reads
// (render.Target.Uncovered), the screen's colours left as they are: an instance a disc facing
// the eye round the middle of what stands there (disc.xyz), reaching disc.w, lying half its
// reach before that middle and nudged as what stands there is, so only what stands clear
// before it gives way; at each pixel as deep as the disc's own plane there, since a depth is
// told apart along one line of sight alone; written as the library's uncoveredAt says at the
// pixels uncoveredOpen leaves open. Nothing of one whose middle the ground hides from the eye:
// what stands before that ground has nothing to show.

struct Bared {
    @builtin(position) clip: vec4<f32>,
    // where the pixel lies off the disc's middle, 1 at its rim
    @location(0) off: vec2<f32>,
}

// how many times the ground is met from a point towards the eye, half a cell on at first and
// each step a twentieth longer than the one before, and how far over a point the ground must
// stand to hide it
const hiddenSteps: i32 = 48;
const hiddenOver: f32 = 0.5;

// hidden reports whether the ground stands between the point p and the eye, toward which toward
// leads: its heights met along that way till it passes over the highest of them or the eye.
fn hidden(p: vec3<f32>, toward: vec3<f32>) -> bool {
    let level = length(toward.xy);
    if U.GroundLattice.w < 0.5 || level < 1e-3 {
        return false;
    }
    let top = groundHighest();
    var step = 0.5 * U.GroundLattice.z / level;
    var eye = 3.0e38;
    if U.Perspective > 0.5 {
        eye = length(U.Eye - p);
    }
    var far = step;
    for (var i = 0; i < hiddenSteps; i++) {
        let q = p + toward * far;
        if q.z > top || far >= eye {
            return false;
        }
        if groundHeightAt(q.xy) > q.z + hiddenOver {
            return true;
        }
        far += step;
        step *= 1.05;
    }
    return false;
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) disc: vec4<f32>) -> Bared {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let side = vec2<f32>(across[vid], down[vid]) * 2.0 - vec2<f32>(1.0);
    let mid = disc.xyz;
    var toward = -normalize(U.Look);
    if U.Perspective > 0.5 {
        toward = normalize(U.Eye - mid);
    }
    var b: Bared;
    b.off = side;
    b.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0);
    let front = mid + toward * (0.5 * disc.w);
    if project(front).w <= 0.0 || hidden(mid, toward) {
        return b;
    }
    // across the screen and up it, square to the line of sight
    let up = normalize(cross(U.Right, -toward));
    let p = front + (U.Right * side.x + up * side.y) * disc.w;
    b.clip = project(p + nudge(p));
    b.clip.z = uncoveredAt(b.clip.z / max(b.clip.w, 1e-6)) * b.clip.w;
    return b;
}

@fragment
fn fs_main(b: Bared) -> @location(0) vec4<f32> {
    if !uncoveredOpen(b.off, b.clip.xy) {
        discard;
    }
    return vec4<f32>(0.0);
}
