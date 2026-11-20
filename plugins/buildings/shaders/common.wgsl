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
// a pane, 0 between them. A pane a bay a storey, none cut by the wall's corners, as much of a bay
// and of a storey as U.SolidPane says, but in the first storey by the wall's door (doorIn).
fn paneOf(along: f32, up: f32, wide: f32, bay: f32, storey: f32, eaves: f32,
    door: vec2<f32>) -> vec3<f32> {
    if bay <= 0.0 || up <= 0.0 || up >= eaves {
        return vec3<f32>(0.0);
    }
    let bays = max(floor(2.0 * wide / bay), 1.0);
    let step = 2.0 * wide / bays;
    let x = (along + wide) / step;
    let y = up / storey;
    let inX = abs(fract(x) - 0.5) < U.SolidPane.x / 2.0;
    let inY = fract(y) > U.SolidPane.y && fract(y) < U.SolidPane.z;
    let mid = -wide + (floor(x) + 0.5) * step;
    if inX && inY && floor(y) * storey + storey <= eaves + 1e-3 &&
        !(y < 1.0 && byDoor(mid, U.SolidPane.x * step / 2.0, step, door)) {
        return vec3<f32>(floor(x), floor(y), 1.0);
    }
    return vec3<f32>(floor(x), floor(y), 0.0);
}

// doorIn is the door door as paneOf takes it on the wall facing way out and running run whose
// point p lies along off its middle, deep in from its outer face: the door's middle along the
// wall and half its width, zero where it stands in another wall
fn doorIn(door: vec4<f32>, p: vec2<f32>, along: f32, way: vec2<f32>, run: vec2<f32>,
    deep: f32) -> vec2<f32> {
    let off = door.xy - p;
    if door.z <= 0.0 || abs(dot(off, way) - deep) > 0.5 {
        return vec2<f32>(0.0);
    }
    return vec2<f32>(along + dot(off, run), door.z);
}

// byDoor reports whether a pane mid along a wall, half wide either side in a bay step long, comes
// nearer the door (doorIn) than half the pier the panes keep apart: such a window is left out
fn byDoor(mid: f32, half: f32, step: f32, door: vec2<f32>) -> bool {
    return door.y > 0.0 && abs(mid - door.x) - half - door.y < (1.0 - U.SolidPane.x) * step / 2.0;
}

// doorOf reports whether the point p of a wall facing way out, deep in from its outer face,
// lies in the door door says: its middle on the outer face's foot, half its width and its height
// over the floor at floor
fn doorOf(p: vec3<f32>, way: vec2<f32>, door: vec4<f32>, floor: f32, deep: f32) -> bool {
    if door.z <= 0.0 {
        return false;
    }
    let off = p.xy - door.xy;
    let across = abs(dot(off, way));
    let along = abs(dot(off, vec2<f32>(-way.y, way.x)));
    return across < deep + 0.5 && along < door.z && p.z >= floor - 0.01 &&
        p.z < floor + door.w;
}

// eyeIn reports whether the eye stands in a room: within the walls of the rectangle half wide
// either way off mid along along and across it, between its floor and its ceiling
fn eyeIn(mid: vec2<f32>, half: vec2<f32>, along: vec2<f32>, floor: f32, ceiling: f32) -> bool {
    let q = U.SolidEye.xy - mid;
    let local = vec2<f32>(dot(q, along), dot(q, vec2<f32>(-along.y, along.x)));
    return abs(local.x) < half.x && abs(local.y) < half.y && U.SolidEye.z > floor &&
        U.SolidEye.z < ceiling;
}

// a wall of a house with a room: the way out of it, the way it runs, half its length and how
// far out of the middle its outer face stands
struct Wall {
    out: vec2<f32>,
    run: vec2<f32>,
    wide: f32,
    deep: f32,
}

// wallOf is the wall j of a house its length along, half as long and wide as half: its long
// sides first, by its side and against it, then its ends, by along and against it; each runs
// the way facing takes it, its run across the upright the way into the house
fn wallOf(j: u32, along: vec2<f32>, half: vec2<f32>) -> Wall {
    let side = vec2<f32>(-along.y, along.x);
    switch j {
        case 0u: {
            return Wall(side, along, half.x, half.y);
        }
        case 1u: {
            return Wall(-side, -along, half.x, half.y);
        }
        case 2u: {
            return Wall(along, -side, half.y, half.x);
        }
        default: {
            return Wall(-along, side, half.y, half.x);
        }
    }
}

// wallTo is which wall of a house its length along faces the way way, along it or across it
fn wallTo(way: vec2<f32>, along: vec2<f32>) -> u32 {
    let d = dot(way, along);
    if d > 0.5 {
        return 2u;
    }
    if d < -0.5 {
        return 3u;
    }
    if dot(way, vec2<f32>(-along.y, along.x)) > 0.0 {
        return 0u;
    }
    return 1u;
}

// shows reports whether the wall w of a house standing off mid from foot to top shows its outer
// face to the eye
fn shows(w: Wall, mid: vec2<f32>, foot: f32, top: f32) -> bool {
    return facing(mid, w.out * w.deep, w.run * w.wide, foot, top) > 0.0;
}

// hash2 is a number in [0, 1) of the point p
fn hash2(p: vec2<f32>) -> f32 {
    return fract(sin(dot(p, vec2<f32>(127.1, 311.7))) * 43758.5453);
}

// valueNoise is smooth noise in [0, 1) at p, a cell a unit
fn valueNoise(p: vec2<f32>) -> f32 {
    let i = floor(p);
    let f = p - i;
    let u = f * f * (3.0 - 2.0 * f);
    let a = hash2(i);
    let b = hash2(i + vec2<f32>(1.0, 0.0));
    let c = hash2(i + vec2<f32>(0.0, 1.0));
    let d = hash2(i + vec2<f32>(1.0, 1.0));
    return a + (b - a) * u.x + (c - a) * u.y + (a - b - c + d) * u.x * u.y;
}

// fbm is valueNoise at p over four octaves
fn fbm(p0: vec2<f32>) -> f32 {
    var p = p0;
    var sum = 0.0;
    var amp = 0.5;
    var all = 0.0;
    for (var i = 0; i < 4; i++) {
        sum += amp * valueNoise(p);
        all += amp;
        p = p * 2.03 + vec2<f32>(17.0, 31.0);
        amp *= 0.5;
    }
    return sum / all;
}

// a storey's height in metres, the measure the walls' textures are made to
const storeyMetres = 3.0;

// detailOf is how much of a texture's detail size metres across a pixel spanning span storeys
// shows: whole while the pixel is smaller, none once it is three times as large
fn detailOf(size: f32, span: f32) -> f32 {
    return 1.0 - smoothstep(size, 3.0 * size, span * storeyMetres);
}

// plaster is a wall of the colour base rendered in lime at q, storeys along it and up from its
// storeys' ground, a pixel spanning span storeys: blotches a metre or two across, a grain, and
// damp darker over its foot
fn plaster(base: vec3<f32>, q: vec2<f32>, span: f32) -> vec3<f32> {
    let m = q * storeyMetres;
    var k = 1.0 + 0.07 * (fbm(m * 0.7) - 0.5);
    k += 0.05 * (valueNoise(m * 14.0) - 0.5) * detailOf(0.07, span);
    if m.y > 0.0 {
        k -= 0.1 * exp(-m.y / 0.35) * (0.6 + 0.4 * fbm(vec2<f32>(m.x * 1.5, 3.0)));
    }
    return base * k;
}

// ashlar is a wall of the stone stone dressed in courses at q, storeys along and up, a pixel
// spanning span storeys: blocks a course tall and their joints of a lighter mortar
fn ashlar(stone: vec3<f32>, q: vec2<f32>, span: f32) -> vec3<f32> {
    let m = q * storeyMetres;
    let course = 0.36;
    let row = floor(m.y / course);
    let off = hash2(vec2<f32>(row, 7.0)) * 0.9;
    let long = 0.55 + 0.35 * hash2(vec2<f32>(row, 3.0));
    let col = floor((m.x + off) / long);
    var k = 0.88 + 0.2 * hash2(vec2<f32>(col, row)) + 0.08 * (valueNoise(m * 9.0) - 0.5);
    let joint = min(fract((m.x + off) / long) * long, fract(m.y / course) * course);
    let jointed = (1.0 - step(0.025, joint)) * detailOf(0.025, span);
    return mix(stone * k, mix(stone, vec3<f32>(1.0), 0.4), jointed);
}

// quoined reports whether the point along a wall wide either side of its middle and up over its
// storeys' ground lies in a stone of its corners, a storey storey tall: long and short in turn up
// them, to its eaves
fn quoined(along: f32, up: f32, wide: f32, storey: f32, eaves: f32) -> bool {
    if up < 0.0 || up >= eaves {
        return false;
    }
    var reach = 0.14 * storey;
    if fract(up / (0.22 * storey)) >= 0.5 {
        reach = 0.085 * storey;
    }
    return wide - abs(along) < reach;
}

// framed reports whether the point along a wall wide either side of its middle and up over its
// storeys' ground lies in the stone framing an opening: a pane of its bays under the eaves as
// paneOf lays them, a storey storey tall, or its door (doorIn) tall high
fn framed(along: f32, up: f32, wide: f32, bay: f32, storey: f32, eaves: f32, door: vec2<f32>,
    tall: f32) -> bool {
    let frame = 0.06 * storey;
    if door.y > 0.0 && abs(along - door.x) < door.y + frame && up < tall + frame {
        return true;
    }
    if bay <= 0.0 || up <= 0.0 || up >= eaves {
        return false;
    }
    let bays = max(floor(2.0 * wide / bay), 1.0);
    let step = 2.0 * wide / bays;
    let x = (along + wide) / step;
    let y = up / storey;
    if floor(y) * storey + storey > eaves + 1e-3 {
        return false;
    }
    let mid = -wide + (floor(x) + 0.5) * step;
    let half = U.SolidPane.x * step / 2.0;
    if y < 1.0 && byDoor(mid, half, step, door) {
        return false;
    }
    return abs(along - mid) < half + frame && fract(y) > U.SolidPane.y - 0.06 &&
        fract(y) < U.SolidPane.z + 0.06;
}
