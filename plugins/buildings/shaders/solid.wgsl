// A part of a building an instance: walls on a rectangle turned any way, from its foot to its
// eaves, and a roof over them, two slopes along it and one at each end, meeting at a ridge as
// long as the instance says: none of it a level roof, all of the part a gable's, a point a
// pyramid's. Drawn through U.SolidViewProj as the ground's mesh is (camera.Transform, sunk by
// U.SolidBend·d² off U.SolidEye), in the frame's light, the relief's shade and the air's haze.

struct Solid {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) @interpolate(flat) normal: vec3<f32>,
    // along the face, world units from its middle, and the height over the building's ground
    @location(2) at: vec2<f32>,
    @location(3) @interpolate(flat) walls: vec4<f32>, // the walls' colour, the building's seed
    @location(4) @interpolate(flat) tiles: vec4<f32>, // the roof's colour, the eaves' height
    // 0 a wall, 1 a roof's slope; how high a storey is, how wide a bay, how wide the face
    @location(5) @interpolate(flat) face: vec4<f32>,
    // the lamps' light, and how much of the sun the relief and the clouds let through
    @location(6) lit: vec4<f32>,
}

const wallVertices = 12u;
const slopeVertices = 12u;

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

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // its middle, its foot and its eaves
    @location(1) b: vec4<f32>, // half its length and its width, the way its length runs
    @location(2) c: vec4<f32>, // its roof's rise and half its ridge, a storey, a bay
    @location(3) d: vec4<f32>, // its walls' colour, its seed
    @location(4) e: vec4<f32>, // its roof's colour, the ground its storeys are counted from
) -> Solid {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var up = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let half = b.xy;
    let rise = c.x;
    let ridge = c.y;
    var p = vec3<f32>(0.0);
    var n = vec3<f32>(0.0, 0.0, 1.0);
    var at = vec2<f32>(0.0);
    var kind = 0.0;
    var wide = 0.0;
    // far off a part is its walls under a level lid, half its roof's rise up
    let far = U.SolidFar > 0.5;
    var top = a.w;
    if far {
        top += 0.5 * rise;
    }
    if vid < wallVertices {
        // the walls seen: one of its long sides, then one of its ends
        let u = across[vid % 6u] * 2.0 - 1.0;
        let v = up[vid % 6u];
        var off = vec2<f32>(0.0);
        if vid < 6u {
            let sign = facing(a.xy, side * half.y, along * half.x, a.z, top);
            off = along * u * half.x * sign + side * half.y * sign;
            n = vec3<f32>(side * sign, 0.0);
            wide = half.x;
        } else {
            let sign = facing(a.xy, along * half.x, -side * half.y, a.z, top);
            off = along * half.x * sign - side * u * half.y * sign;
            n = vec3<f32>(along * sign, 0.0);
            wide = half.y;
        }
        p = vec3<f32>(a.xy + off, mix(a.z, top, v));
        at = vec2<f32>(u * wide, p.z - e.w);
    } else if far {
        let k = vid - wallVertices;
        let u = across[k % 6u] * 2.0 - 1.0;
        let v = up[k % 6u] * 2.0 - 1.0;
        p = vec3<f32>(a.xy + along * u * half.x + side * v * half.y, top);
        kind = 1.0;
        wide = half.x;
    } else if vid < wallVertices + slopeVertices {
        // the slopes along it: the eaves below, the ridge above
        let k = vid - wallVertices;
        let sign = 1.0 - 2.0 * f32(k / 6u);
        let u = across[k % 6u] * 2.0 - 1.0;
        let v = up[k % 6u];
        let reach = mix(half.x, ridge, v);
        let off = along * u * reach * sign + side * half.y * (1.0 - v) * sign;
        p = vec3<f32>(a.xy + off, a.w + rise * v);
        n = normalize(vec3<f32>(side * sign * rise, half.y));
        at = vec2<f32>(u * reach, length(vec2<f32>(half.y, rise)) * v);
        kind = 1.0;
        wide = half.x;
    } else {
        // the ends: a slope of a hipped roof, the wall's gable where the ridge runs through
        let k = vid - wallVertices - slopeVertices;
        let sign = 1.0 - 2.0 * f32(k / 3u);
        let corner = k % 3u;
        var off = along * half.x * sign;
        var z = a.w;
        if corner == 0u {
            off += side * half.y;
        } else if corner == 1u {
            off -= side * half.y;
        } else {
            off = along * ridge * sign;
            z += rise;
        }
        p = vec3<f32>(a.xy + off, z);
        let run = half.x - ridge;
        n = normalize(vec3<f32>(along * sign * max(rise, 1e-4), run));
        if run < 1e-3 * half.x {
            n = vec3<f32>(along * sign, 0.0);
        } else {
            kind = 1.0;
        }
        at = vec2<f32>(0.0, z - e.w);
        wide = half.y;
    }
    var s: Solid;
    let from = p.xy - U.SolidEye.xy;
    s.clip = U.SolidViewProj * vec4<f32>(p.xy, p.z - U.SolidBend * dot(from, from), 1.0);
    s.world = p;
    s.normal = n;
    s.at = at;
    s.walls = d;
    s.tiles = vec4<f32>(e.rgb, a.w - e.w);
    s.face = vec4<f32>(kind, c.z, c.w, wide);
    // a vertex's share of the sun serves its face: the shade changes slowly over a building;
    // one far off, drawn without its windows, takes the relief's shade alone
    var reached = groundShadeAt(p.xy, max(p.z - e.w, 0.0), 0.25 * c.z);
    var lamps = vec3<f32>(0.0);
    if c.w > 0.0 {
        if U.Cover > 0.0 && U.CloudsShadeFigures > 0.5 {
            reached *= 1.0 - cloudShade(cloudsShadeAt(p, 2.0 * wide));
        }
        lamps = lampLightAt(p);
    }
    s.lit = vec4<f32>(lamps, reached);
    return s;
}

// how much of a bay and of a storey a window takes, and how many of them are lit at night
const paneWide = 0.44;
const paneFrom = 0.32;
const paneTo = 0.8;
const litShare = 0.3;

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

@fragment
fn fs_main(s: Solid) -> @location(0) vec4<f32> {
    let sun = lightWay();
    var day = 0.0;
    if sun.z > 0.0 {
        day = 1.0;
    }
    // a building in three is washed warmer than its kind, one in three rosier
    var rgb = s.walls.rgb;
    let wash = chance(vec3<f32>(s.walls.w, 1.0, 2.0));
    if wash < 0.33 {
        rgb *= vec3<f32>(1.0, 0.95, 0.84);
    } else if wash < 0.5 {
        rgb *= vec3<f32>(1.0, 0.9, 0.88);
    }
    var glow = vec3<f32>(0.0);
    let storey = s.face.y;
    let bay = s.face.z;
    if s.face.x > 0.5 {
        rgb = s.tiles.rgb * (0.92 + 0.16 * chance(vec3<f32>(s.walls.w, 3.0, 1.0)));
        if bay > 0.0 && s.normal.z < 0.999 {
            // the tiles' courses down the slope
            rgb *= 0.93 + 0.07 * step(0.5, fract(s.at.y / (0.12 * storey)));
        }
    } else if bay > 0.0 && s.at.y > 0.0 && s.at.y < s.tiles.w {
        // a window a bay a storey, the bays counted from the wall's middle and none cut by its
        // corners
        let bays = max(floor(2.0 * s.face.w / bay), 1.0);
        let step = 2.0 * s.face.w / bays;
        let x = (s.at.x + s.face.w) / step;
        let y = s.at.y / storey;
        let inX = abs(fract(x) - 0.5) < paneWide / 2.0;
        let inY = fract(y) > paneFrom && fract(y) < paneTo;
        if inX && inY && floor(y) * storey + storey <= s.tiles.w + 1e-3 {
            rgb = vec3<f32>(0.16, 0.2, 0.25);
            let side = dot(s.normal.xy, vec2<f32>(3.0, 5.0));
            let which = vec3<f32>(floor(x) + 7.0 * s.walls.w, floor(y), side);
            // as the sky's light goes: the moon's is as strong as a low sun's
            let dusk = 1.0 - smoothstep(0.08, 0.2, dot(U.Ambience, vec3<f32>(0.3, 0.59, 0.11)));
            if chance(which) < litShare {
                glow = vec3<f32>(1.0, 0.82, 0.5) * 0.9 * dusk;
            }
        }
    }
    let reached = s.lit.w;
    let diffuse = max(dot(s.normal, sun), 0.0) * day;
    // what the lit ground throws back up a wall
    let bounce = 0.45 * (1.0 - abs(s.normal.z)) * max(sun.z, 0.0);
    let light = U.Ambience + U.LightColor * (diffuse + bounce) * U.LightStrength * reached
        + s.lit.rgb;
    rgb = rgb * light + glow;
    if U.SolidHaze.x > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(s.world) / U.SolidHaze.x));
    }
    if U.SolidSight.w > 0.0 {
        let far = distance(s.world.xy, U.SolidSight.xy);
        rgb = mix(rgb, U.Fog, smoothstep(U.SolidSight.z, U.SolidSight.w, far));
    }
    return vec4<f32>(rgb, 1.0);
}
