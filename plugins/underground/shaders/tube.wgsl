// What lies under the ground from inside: a stretch of a structure an instance, from a to b
// along its floor, half its width either side and its room up to its roof, drawn through
// U.TubeViewProj as the ground's mesh is (camera.Transform, sunk by U.TubeBend·d² off
// U.TubeEye). The faces of a stretch, six vertices each: its floor, its roof, the wall on its
// left and on its right going from a to b, and the walls closing it at a and at b; faces says
// which of the four walls stand, a bit each.

struct Tube {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) @interpolate(flat) face: u32,
    // along the stretch and across it or up its wall, in world units
    @location(2) at: vec2<f32>,
    @location(3) @interpolate(flat) room: vec2<f32>, // how wide and how high the stretch is
}

const faceFloor = 0u;
const faceRoof = 1u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // where it begins on its floor, and half its width
    @location(1) b: vec4<f32>, // where it ends on its floor, and its room up to its roof
    @location(2) c: vec4<f32>, // its walls standing (faces), how far along its line it begins
) -> Tube {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var up = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let face = vid / 6u;
    let u = across[vid % 6u];
    let v = up[vid % 6u];
    let run = b.xyz - a.xyz;
    let length = max(length(run.xy), 1e-6);
    let d = run.xy / length;
    let n = vec2<f32>(-d.y, d.x);
    let half = a.w;
    let clear = b.w;
    var p = a.xyz;
    var at = vec2<f32>(0.0);
    var stands = true;
    switch face {
        case 0u, 1u: { // the floor, the roof
            p = a.xyz + run * u + vec3<f32>(n * (v * 2.0 - 1.0) * half, 0.0);
            if face == 1u {
                p.z += clear;
            }
            at = vec2<f32>(c.y + u * length, (v * 2.0 - 1.0) * half);
        }
        case 2u, 3u: { // the walls along it
            let side = 5.0 - 2.0 * f32(face); // 1 on its left, -1 on its right
            p = a.xyz + run * u + vec3<f32>(n * side * half, v * clear);
            at = vec2<f32>(c.y + u * length, v * clear);
            stands = (u32(c.x) & (1u << (face - 2u))) != 0u;
        }
        default: { // the walls closing it
            p = a.xyz + run * f32(face - 4u) + vec3<f32>(n * (u * 2.0 - 1.0) * half, v * clear);
            at = vec2<f32>((u * 2.0 - 1.0) * half, v * clear);
            stands = (u32(c.x) & (1u << (face - 2u))) != 0u;
        }
    }
    var t: Tube;
    let off = p.xy - U.TubeEye.xy;
    t.clip = U.TubeViewProj * vec4<f32>(p.xy, p.z - U.TubeBend * dot(off, off), 1.0);
    if !stands {
        t.clip = vec4<f32>(2.0, 2.0, 2.0, 1.0); // off the picture
    }
    t.world = p;
    t.face = face;
    t.at = at;
    t.room = vec2<f32>(2.0 * half, clear);
    return t;
}

// how far apart the lamps hang under the roof, in rooms' heights, and how far each lights
const lampEvery = 6.0;

// fs_main is the stretch's face in the light under the ground: none of the sun's, a share of
// the air's, the frame's lamps, and the structure's own lamps under its roof, a strip every
// lampEvery heights of its room, warm, each lighting the floor and the walls about it.
@fragment
fn fs_main(t: Tube) -> @location(0) vec4<f32> {
    var rgb = vec3<f32>(0.42, 0.41, 0.39); // the walls' concrete
    let period = lampEvery * t.room.y;
    let along = t.at.x / period;
    let near = abs(fract(along) - 0.5) * 2.0; // 0 under a lamp, 1 halfway between two
    var glow = 0.0;
    if t.face == faceFloor {
        rgb = vec3<f32>(0.17, 0.17, 0.18);
        // a broken line down the middle of a floor wide enough for two
        let dash = step(fract(t.at.x / (t.room.y * 1.5)), 0.5);
        let line = 1.0 - smoothstep(0.02, 0.04, abs(t.at.y) / t.room.x);
        if t.room.x > 1.5 * t.room.y {
            rgb = mix(rgb, vec3<f32>(0.8, 0.8, 0.75), dash * line);
        }
    } else if t.face == faceRoof {
        rgb = vec3<f32>(0.27, 0.27, 0.27);
        // the lamp itself: a strip across the roof
        let strip = 1.0 - smoothstep(0.02, 0.05, near);
        let wide = 1.0 - smoothstep(0.25, 0.3, abs(t.at.y) / t.room.x);
        glow = strip * wide;
    }
    let own = vec3<f32>(1.0, 0.86, 0.62) * (0.25 + 0.75 * (1.0 - near) * (1.0 - near));
    let light = U.Ambience * 0.15 + own + lampLightAt(t.world);
    rgb = rgb * light + vec3<f32>(1.0, 0.92, 0.75) * glow;
    return vec4<f32>(rgb, 1.0);
}
