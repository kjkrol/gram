// A piece of road an instance: a strip from one point of its axis to the next, as wide at each
// as the instance says and level across, or a round plate. Drawn through U.RibbonViewProj as
// the ground's mesh is (camera.Transform, sunk by U.RibbonBend·d² off U.RibbonEye), in the
// frame's light, the ground's shade and the air's haze, its lanes marked.

struct Ribbon {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    // across, world units off the axis; half the width there; along the stretch; a plate's
    // share of its radius
    @location(1) at: vec4<f32>,
    @location(2) plate: vec2<f32>,
    @location(3) @interpolate(flat) lanes: vec4<f32>, // lanes, kind, the plate's reach
    @location(4) @interpolate(flat) surface: vec3<f32>,
    @location(5) lamps: vec3<f32>,
}

const oneWay = 1u;
const disc = 2u;
const island = 4u;
const arm = 8u;
const backward = 16u;
const bank = 32u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // the axis from and to
    @location(1) b: vec4<f32>, // half the width to the left at either
    @location(2) c: vec4<f32>, // the heights at either, how far along the stretch each lies
    @location(3) d: vec4<f32>, // lanes, kind, a plate's reach, 1 under the ground
    @location(4) e: vec4<f32>, // the surface's colour
) -> Ribbon {
    var along = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var across = array<f32, 6>(-1.0, -1.0, 1.0, -1.0, 1.0, 1.0);
    let t = along[vid];
    let s = across[vid];
    let side = mix(b.xy, b.zw, t);
    var p = vec3<f32>(mix(a.xy, a.zw, t) + side * s, mix(c.x, c.y, t));
    var out: Ribbon;
    out.plate = vec2<f32>(s, 2.0 * t - 1.0);
    if (u32(d.y) & bank) != 0u {
        // a bank or a wall: from the road's edge (a, at c.xy) down to its foot, b off it, at
        // c.zw; one face, lit by how it stands to the light
        let foot = max(s, 0.0);
        p = vec3<f32>(mix(a.xy, a.zw, t) + side * foot,
            mix(mix(c.x, c.y, t), mix(c.z, c.w, t), foot));
        let run = vec3<f32>(a.zw - a.xy, c.y - c.x);
        let fall = vec3<f32>((b.xy + b.zw) * 0.5, (c.z + c.w - c.x - c.y) * 0.5);
        var up = cross(run, fall);
        up = up / max(length(up), 1e-6);
        if up.z < 0.0 {
            up = -up;
        }
        out.plate = vec2<f32>(max(dot(up, lightWay()), 0.0), 0.0);
    }
    let from = p.xy - U.RibbonEye.xy;
    out.clip = U.RibbonViewProj * vec4<f32>(p.xy, p.z - U.RibbonBend * dot(from, from), 1.0);
    out.world = p;
    let half = length(side);
    out.at = vec4<f32>(s * half, half, mix(c.z, c.w, t), 0.0);
    out.lanes = d;
    out.surface = e.rgb;
    out.lamps = lampLightAt(p);
    return out;
}

// hazeDepth is how much air lies between the eye and the point p, world units, as the ground's
// shader has it.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.RibbonHaze.y <= 0.0 {
        return distance(U.RibbonEye, p);
    }
    return max(U.RibbonHaze.y - p.z, 0.0) / max(-U.RibbonHazeWay.z, 0.05);
}

// how much of a pixel a line half wide either side of where off is 0 covers, px a pixel's
// width: a line thinner than a pixel fades instead of flickering
fn line(off: f32, half: f32, px: f32) -> f32 {
    let edge = clamp((half - abs(off)) / max(px, 1e-5) + 0.5, 0.0, 1.0);
    return edge * min(2.0 * half / max(px, 1e-5), 1.0);
}

// metres: a line, a dash and the gap after it, an arrow and how far apart arrows lie
const lineHalf = 0.07;
const dashLong = 3.0;
const dashEvery = 9.0;
const arrowLong = 3.0;
const arrowEvery = 40.0;

@fragment
fn fs_main(in: Ribbon) -> @location(0) vec4<f32> {
    let kind = u32(in.lanes.y);
    var rgb = in.surface;
    if (kind & disc) != 0u {
        let far = length(in.plate);
        if far > 1.0 {
            discard;
        }
        // metres in from the rim
        let off = (1.0 - far) * in.at.y / U.RibbonMetre;
        if (kind & island) != 0u {
            if off < 0.4 {
                rgb = vec3<f32>(0.8, 0.8, 0.78); // the kerb
            }
        } else if in.lanes.x > 0.0 {
            // a roundabout's roadway: a line along its island; the one along its rim is laid
            // in pieces, broken where roads run in
            let px = fwidth(off);
            let wide = in.lanes.z / U.RibbonMetre;
            rgb = mix(rgb, vec3<f32>(0.92, 0.92, 0.9), line(off - (wide - 0.4), lineHalf, px));
        }
    } else {
        let m = U.RibbonMetre;
        let x = in.at.x / m;
        let half = in.at.y / m;
        let s = in.at.z / m;
        let px = fwidth(x);
        let lanes = in.lanes.x;
        var mark = 0.0;
        if lanes > 0.0 {
            let road = half - 0.5;
            mark = line(abs(x) - (road + 0.1), lineHalf, px);
            let dashed = step(fract(s / dashEvery), dashLong / dashEvery);
            let one = (kind & oneWay) != 0u;
            if !one && lanes >= 4.0 {
                mark = max(mark, line(abs(x) - 0.15, lineHalf, px));
            } else if !one && lanes >= 2.0 {
                mark = max(mark, line(x, lineHalf, px) * dashed);
            }
            var each = lanes;
            if !one {
                each = floor(lanes / 2.0);
            }
            if each >= 2.0 {
                let wide = 2.0 * road / lanes;
                let k = round(x / wide);
                var between = abs(k) >= 1.0 && abs(k) < each;
                if one {
                    let j = round((x + road) / wide);
                    between = j >= 1.0 && j < lanes;
                    mark = max(mark, line(x + road - j * wide, lineHalf, px) * dashed
                        * f32(between));
                } else {
                    mark = max(mark, line(x - k * wide, lineHalf, px) * dashed * f32(between));
                }
            }
            if one && lanes == 1.0 {
                // an arrow the way it is driven
                var way = s;
                if (kind & backward) != 0u {
                    way = -s;
                }
                // midway between two junctions' reach, never in a mouth
                let q = fract(way / arrowEvery + 0.5) * arrowEvery;
                if q < arrowLong {
                    let wide = 0.45 * (1.0 - q / arrowLong);
                    mark = max(mark, clamp((wide - abs(x)) / max(px, 1e-5) + 0.5, 0.0, 1.0)
                        * min(1.2 / max(fwidth(s), 1e-5), 1.0));
                }
            }
            if (kind & arm) != 0u && in.at.z < in.lanes.z {
                mark = 0.0;
            }
        }
        rgb = mix(rgb, vec3<f32>(0.92, 0.92, 0.9), mark);
    }
    if in.lanes.w > 0.5 {
        // under the ground: none of the sun's, a share of the air's, a tunnel's lamps
        let under = U.Ambience * 0.15 + vec3<f32>(1.0, 0.86, 0.62) * 0.55 + in.lamps;
        return vec4<f32>(rgb * under, 1.0);
    }
    let sun = lightWay();
    var reached = groundShadeAt(in.world.xy, 0.0, 0.0);
    if U.Cover > 0.0 {
        reached *= 1.0 - cloudShade(cloudsShadeAt(in.world, 4.0));
    }
    var facing = max(sun.z, 0.0);
    if (kind & bank) != 0u {
        facing = in.plate.x;
    }
    let light = U.Ambience + U.LightColor * facing * U.LightStrength * reached
        + in.lamps;
    rgb = rgb * light;
    if U.RibbonHaze.x > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(in.world) / U.RibbonHaze.x));
    }
    if U.RibbonSight.w > 0.0 {
        let far = distance(in.world.xy, U.RibbonSight.xy);
        rgb = mix(rgb, U.Fog, smoothstep(U.RibbonSight.z, U.RibbonSight.w, far));
    }
    return vec4<f32>(rgb, 1.0);
}
