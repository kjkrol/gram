// A part of a building an instance: walls on a rectangle turned any way, from its foot to its
// eaves, and a roof over them, two slopes along it and one at each end, meeting at a ridge as
// long as the instance says: none of it a level roof, all of the part a gable's, a point a
// pyramid's. Drawn through U.SolidViewProj as the ground's mesh is (camera.Transform, sunk by
// U.SolidBend·d² off U.SolidEye), in the frame's light, the relief's shade and the air's haze. A
// building with a room has its door and the windows of its room's storey open, where the ground
// outside lies under their sills, and its body is left out where the eye stands in its room;
// near, what lies over something uncovered gives way (U.SolidUncovering).

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
    // a building with a room: its door's middle, half its width and its height; 0 wide, none
    @location(7) @interpolate(flat) door: vec4<f32>,
    // the stone its plinth, its corners and its openings' frames are dressed in, 1 where it is
    @location(8) @interpolate(flat) stone: vec4<f32>,
}

const wallVertices = 12u;
const slopeVertices = 12u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // its middle, its foot and its eaves
    @location(1) b: vec4<f32>, // half its length and its width, the way its length runs
    @location(2) c: vec4<f32>, // its roof's rise and half its ridge, a storey, a bay
    @location(3) d: vec4<f32>, // its walls' colour, its seed
    @location(4) e: vec4<f32>, // its roof's colour, the ground its storeys are counted from
    @location(5) f: vec4<f32>, // its door, as Solid.door
    @location(6) g: vec4<f32>, // its stone, as Solid.stone
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
    // open round its room (U.SolidOpen) a house's body is its four walls, those showing the eye
    // their outer faces cut low, the other two as high as the room, and no roof
    let open = U.SolidOpen.x > 0.5;
    var stands = !open || (f.z > 0.0 && vid < 2u * wallVertices);
    if open && vid < 2u * wallVertices {
        let w = wallOf(vid / 6u, along, half);
        let u = across[vid % 6u] * 2.0 - 1.0;
        let v = up[vid % 6u];
        var wallTop = e.w + c.z;
        if shows(w, a.xy, a.z, top) {
            wallTop = min(e.w + c.z * U.SolidOpen.y, wallTop);
        }
        p = vec3<f32>(a.xy + w.out * w.deep + w.run * u * w.wide, mix(a.z, wallTop, v));
        n = vec3<f32>(w.out, 0.0);
        wide = w.wide;
        at = vec2<f32>(u * w.wide, p.z - e.w);
    } else if vid < wallVertices {
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
        if corner == 0u {
            at.x = half.y;
        } else if corner == 1u {
            at.x = -half.y;
        }
        wide = half.y;
    }
    var s: Solid;
    let from = p.xy - U.SolidEye.xy;
    s.clip = U.SolidViewProj * vec4<f32>(p.xy, p.z - U.SolidBend * dot(from, from), 1.0);
    // a house's body seen from its room: its walls lie on the room's, the rest over its ceiling
    if !stands || (f.z > 0.0 && eyeIn(a.xy, half, along, e.w, e.w + c.z)) {
        s.clip = vec4<f32>(2.0, 2.0, 2.0, 1.0); // off the picture
    }
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
    s.door = f;
    s.stone = g;
    return s;
}

// givenWay reports whether what lies at depth at the target's pixel px gives way, within the
// rectangle of the target the uncovered lie in (U.SolidUncoveredIn)
fn givenWay(px: vec2<f32>, depth: f32) -> bool {
    if U.SolidUncovering < 0.5 || px.x < U.SolidUncoveredIn.x || px.y < U.SolidUncoveredIn.y ||
        px.x > U.SolidUncoveredIn.z || px.y > U.SolidUncoveredIn.w {
        return false;
    }
    return givesWayAt(px, depth);
}

// opened reports whether a wall's point p of a building with a room lies in an opening: its door,
// or a pane of its room's storey (pane) where the ground outside lies under its sill
fn opened(p: vec3<f32>, way: vec2<f32>, door: vec4<f32>, floor: f32, pane: vec3<f32>,
    storey: f32) -> bool {
    if door.z <= 0.0 {
        return false;
    }
    if doorOf(p, way, door, floor, 0.0) {
        return true;
    }
    return pane.z > 0.5 && pane.y < 0.5 &&
        groundHeightAt(p.xy + way * 0.5) < floor + U.SolidPane.y * storey;
}

@fragment
fn fs_main(s: Solid) -> @location(0) vec4<f32> {
    // how many storeys a pixel spans over the face, taken before anything is left out
    let span = length(fwidth(s.at)) / max(s.face.y, 1e-3);
    if givenWay(s.clip.xy, s.clip.z) {
        discard;
    }
    let sun = lightWay();
    var day = 0.0;
    if sun.z > 0.0 {
        day = 1.0;
    }
    var rgb = s.walls.rgb;
    var glow = vec3<f32>(0.0);
    let storey = s.face.y;
    let bay = s.face.z;
    if s.face.x > 0.5 {
        rgb = s.tiles.rgb * (0.92 + 0.16 * chance(vec3<f32>(s.walls.w, 3.0, 1.0)));
        if bay > 0.0 && s.normal.z < 0.999 {
            // the tiles' courses down the slope
            rgb *= 0.93 + 0.07 * step(0.5, fract(s.at.y / (0.12 * storey)));
        }
    } else {
        let floor = s.world.z - s.at.y;
        let way = s.normal.xy;
        let door = doorIn(s.door, s.world.xy, s.at.x, way, vec2<f32>(way.y, -way.x), 0.0);
        let pane = paneOf(s.at.x, s.at.y, s.face.w, bay, storey, s.tiles.w, door);
        if opened(s.world, s.normal.xy, s.door, floor, pane, storey) {
            discard;
        }
        // in storeys, one building and one wall apart from the next
        let q = s.at / storey;
        let apart = vec2<f32>(s.walls.w * 37.0 + dot(way, vec2<f32>(5.0, 11.0)), 0.0);
        rgb = plaster(rgb, q + apart, span);
        if s.stone.w > 0.5 {
            if q.y < 0.0 {
                rgb = ashlar(s.stone.rgb, q + apart, span);
            } else if quoined(s.at.x, s.at.y, s.face.w, storey, s.tiles.w) ||
                framed(s.at.x, s.at.y, s.face.w, bay, storey, s.tiles.w, door, s.door.w) {
                // dressed in courses as tall as the corners' stones, faded where a pixel spans them
                let dressed = ashlar(s.stone.rgb, q * 1.09 + apart, span);
                rgb = mix(rgb, dressed, detailOf(0.2, span));
            }
        }
        if pane.z > 0.5 {
            rgb = vec3<f32>(0.16, 0.2, 0.25);
            let side = dot(s.normal.xy, vec2<f32>(3.0, 5.0));
            let which = vec3<f32>(pane.x + 7.0 * s.walls.w, pane.y, side);
            // as the sky's light goes: the moon's is as strong as a low sun's
            let dusk = 1.0 - smoothstep(0.08, 0.2, dot(U.Ambience, vec3<f32>(0.3, 0.59, 0.11)));
            if chance(which) < U.SolidPane.w {
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
