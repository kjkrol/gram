// A building's room an instance, seen from inside: its floor and the inner faces of its walls, a
// wall's thickness in from the building's, the two across it from the eye and, where the eye
// stands in it or the room is open round it (U.SolidOpen), the two by the eye, and its ceiling
// where the eye stands in it; the reveals of its door and of the windows of its storey through
// the walls' thickness; open, the tops of its walls, those showing the eye their outer faces cut
// low, and the ends of those standing over the low ones. Its openings left out, lit by the sun
// where it comes in by a window or the door (sunIn), by as much of the sky as gets in, by the
// room's own lamp after dusk and by the frame's lamps.

struct Inner {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) @interpolate(flat) normal: vec3<f32>,
    // along the wall, world units from its middle, and over the floor; the outer wall's half width
    @location(2) at: vec3<f32>,
    @location(3) @interpolate(flat) a: vec4<f32>,
    @location(4) @interpolate(flat) b: vec4<f32>,
    @location(5) @interpolate(flat) c: vec4<f32>,
    @location(6) @interpolate(flat) d: vec4<f32>,
    // its colour, and what it is: 0 the floor, 1 a wall's face with openings, 2 one without, 3
    // a wall's section where it is cut, lit as the outside
    @location(7) @interpolate(flat) colour: vec4<f32>,
    @location(8) @interpolate(flat) thick: f32,
}

// how much darker than the walls a room's ceiling is
const ceilingShade = 0.9;

// a room's faces, six vertices each: its floor, its four walls, its ceiling, the tops of its four
// walls, the ends of its walls at its four corners, the reveals of its door, its threshold and the
// top of its wall cut low past it, and from there on the reveals of its windows, four a bay of a
// wall
const ceilingFace = 5u;
const topFaces = 6u;
const endFaces = 10u;
const doorFaces = 14u;
const paneFaces = 19u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // its middle, its floor and its ceiling
    @location(1) b: vec4<f32>, // half its outer length and width, the way its length runs
    @location(2) c: vec4<f32>, // its door: its middle, half its width and its height
    @location(3) d: vec4<f32>, // a storey, a bay, the eaves over the floor, its lamp's strength
    @location(4) e: vec4<f32>, // the floor's colour, how thick its walls are
    @location(5) f: vec4<f32>, // its walls' colour outside, its building's seed
) -> Inner {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var up = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let half = b.xy;
    let thick = e.w;
    let u0 = across[vid % 6u];
    let v0 = up[vid % 6u];
    let u = u0 * 2.0 - 1.0;
    let face = vid / 6u;
    let open = U.SolidOpen.x > 0.5;
    // how high a wall cut low stands
    let low = a.z + d.x * U.SolidOpen.y;
    // plastered, the walls washed half to white inside
    let inner = (f.rgb + vec3<f32>(1.0)) / 2.0;
    var p = vec3<f32>(0.0);
    var n = vec3<f32>(0.0, 0.0, 1.0);
    var at = vec3<f32>(0.0);
    var colour = vec4<f32>(e.rgb, 0.0);
    var stands = true;
    if face == 0u {
        // the floor, within the walls
        p = vec3<f32>(a.xy + along * u * (half.x - thick) +
            side * (v0 * 2.0 - 1.0) * (half.y - thick), a.z);
    } else if face < ceilingFace {
        // a wall's inner face: the long one and the end across from the eye, then those by it
        let first = face < 3u;
        var j = 0u;
        if (face - 1u) % 2u == 1u {
            j = 2u;
        }
        if shows(wallOf(j, along, half), a.xy, a.z, a.w) == first {
            j += 1u;
        }
        let w = wallOf(j, along, half);
        // open, a wall showing the eye its outer face stands low inside too, and one standing by
        // such a wall reaches over its thickness
        var top = a.w;
        var reach = w.wide - thick;
        if open {
            if shows(w, a.xy, a.z, a.w) {
                top = low;
            } else if shows(wallOf(wallTo(w.run * u, along), along, half), a.xy, a.z, a.w) {
                reach = w.wide;
            }
        }
        p = vec3<f32>(a.xy + w.out * (w.deep - thick) + w.run * u * reach, mix(a.z, top, v0));
        n = vec3<f32>(-w.out, 0.0);
        at = vec3<f32>(u * reach, p.z - a.z, w.wide);
        colour = vec4<f32>(inner, 1.0);
        stands = first || open || eyeIn(a.xy, half, along, a.z, a.w);
    } else if face == ceilingFace {
        p = vec3<f32>(a.xy + along * u * (half.x - thick) +
            side * (v0 * 2.0 - 1.0) * (half.y - thick), a.w);
        n = vec3<f32>(0.0, 0.0, -1.0);
        colour = vec4<f32>(inner * ceilingShade, 2.0);
        stands = !open && eyeIn(a.xy, half, along, a.z, a.w);
    } else if face < endFaces {
        // open, the top of a wall, low where it shows the eye its outer face, up to its door there
        let w = wallOf(face - topFaces, along, half);
        var top = a.w;
        var to = w.wide;
        if shows(w, a.xy, a.z, a.w) {
            top = low;
            let door = doorIn(c, a.xy + w.out * w.deep, 0.0, w.out, w.run, 0.0);
            if door.y > 0.0 {
                to = door.x - door.y;
            }
        }
        p = vec3<f32>(a.xy + w.out * mix(w.deep - thick, w.deep, v0) +
            w.run * mix(-w.wide, to, u0), top);
        colour = vec4<f32>(f.rgb, 3.0);
        stands = open;
    } else if face < doorFaces {
        // open, at a corner where one wall stands and the other is cut low, the end of the one
        // standing over the low one
        let k = face - endFaces;
        let sideWall = wallOf(k / 2u, along, half);
        let endWall = wallOf(2u + k % 2u, along, half);
        let sideCut = shows(sideWall, a.xy, a.z, a.w);
        var tall = sideWall;
        var cut = endWall;
        if sideCut {
            tall = endWall;
            cut = sideWall;
        }
        p = vec3<f32>(a.xy + cut.out * cut.deep +
            tall.out * mix(tall.deep - thick, tall.deep, u0), mix(low, a.w, v0));
        n = vec3<f32>(cut.out, 0.0);
        colour = vec4<f32>(f.rgb, 3.0);
        stands = open && sideCut != shows(endWall, a.xy, a.z, a.w);
    } else if face < paneFaces {
        // the reveals of the door: either side of it, over it, its threshold, and past it the top
        // of its wall cut low
        let q = c.xy - a.xy;
        let local = vec2<f32>(dot(q, along), dot(q, side));
        var j = 0u;
        if abs(local.x) / half.x > abs(local.y) / half.y {
            j = select(3u, 2u, local.x > 0.0);
        } else if local.y < 0.0 {
            j = 1u;
        }
        let w = wallOf(j, along, half);
        let mid = dot(q, w.run);
        let cut = open && shows(w, a.xy, a.z, a.w);
        var top = a.z + c.w;
        if cut {
            top = min(top, low);
        }
        let k = face - doorFaces;
        if k < 2u {
            var x = mid - c.z;
            var into = 1.0;
            if k == 1u {
                x = mid + c.z;
                into = -1.0;
            }
            p = vec3<f32>(a.xy + w.run * x + w.out * mix(w.deep - thick, w.deep, u0),
                mix(a.z, top, v0));
            n = vec3<f32>(w.run * into, 0.0);
        } else if k == 2u {
            p = vec3<f32>(a.xy + w.run * (mid + u * c.z) +
                w.out * mix(w.deep - thick, w.deep, v0), a.z + c.w);
            n = vec3<f32>(0.0, 0.0, -1.0);
            stands = !cut;
        } else if k == 3u {
            p = vec3<f32>(a.xy + w.run * (mid + u * c.z) +
                w.out * mix(w.deep - thick, w.deep, v0), a.z);
        } else {
            p = vec3<f32>(a.xy + w.run * mix(mid + c.z, w.wide, u0) +
                w.out * mix(w.deep - thick, w.deep, v0), low);
            stands = cut;
        }
        colour = vec4<f32>(inner, 2.0);
        if k == 3u {
            colour = vec4<f32>(e.rgb, 0.0);
        } else if k == 4u {
            colour = vec4<f32>(f.rgb, 3.0);
        }
        stands = stands && c.z > 0.0;
    } else {
        // the reveals of a window of the room's storey: either side of it, over it, under it
        let i = face - paneFaces;
        let bays = max(u32(U.SolidBays), 1u);
        let k = (i / 4u) % bays;
        let w = wallOf(min(i / (4u * bays), 3u), along, half);
        let count = max(floor(2.0 * w.wide / max(d.y, 1e-3)), 1.0);
        let step = 2.0 * w.wide / count;
        let mid = -w.wide + (f32(k) + 0.5) * step;
        let wide = U.SolidPane.x * step / 2.0;
        let sill = a.z + U.SolidPane.y * d.x;
        let head = a.z + U.SolidPane.z * d.x;
        let quad = i % 4u;
        if quad < 2u {
            var x = mid - wide;
            var into = 1.0;
            if quad == 1u {
                x = mid + wide;
                into = -1.0;
            }
            p = vec3<f32>(a.xy + w.run * x + w.out * mix(w.deep - thick, w.deep, u0),
                mix(sill, head, v0));
            n = vec3<f32>(w.run * into, 0.0);
        } else {
            var z = head;
            var nz = -1.0;
            if quad == 3u {
                z = sill;
                nz = 1.0;
            }
            p = vec3<f32>(a.xy + w.run * (mid + u * wide) +
                w.out * mix(w.deep - thick, w.deep, v0), z);
            n = vec3<f32>(0.0, 0.0, nz);
        }
        colour = vec4<f32>(inner, 2.0);
        // a window opens where the ground outside lies under its sill, its storey under the eaves,
        // clear of the door
        let outside = a.xy + w.run * mid + w.out * (w.deep + 0.5);
        let door = doorIn(c, a.xy + w.out * w.deep, 0.0, w.out, w.run, 0.0);
        stands = d.y > 0.0 && f32(k) < count && d.x <= d.z + 1e-3 &&
            groundHeightAt(outside) < sill && !(open && shows(w, a.xy, a.z, a.w)) &&
            !byDoor(mid, wide, step, door);
    }
    var s: Inner;
    s.clip = clipOf(p);
    if !stands {
        s.clip = vec4<f32>(2.0, 2.0, 2.0, 1.0); // off the picture
    }
    s.world = p;
    s.normal = n;
    s.at = at;
    s.a = a;
    s.b = b;
    s.c = c;
    s.d = d;
    s.colour = colour;
    s.thick = thick;
    return s;
}

// opening reports whether the way toward the sun from the point p of a room leaves the box of its
// walls half wide, deep in from their outer faces, in its door or in a pane of its storey's windows
// with the ground outside under its sill, under its ceiling
fn opening(p: vec3<f32>, way: vec3<f32>, a: vec4<f32>, b: vec4<f32>, c: vec4<f32>,
    d: vec4<f32>, half: vec2<f32>, deep: f32) -> bool {
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let q = p.xy - a.xy;
    let local = vec2<f32>(dot(q, along), dot(q, side));
    let dir = vec2<f32>(dot(way.xy, along), dot(way.xy, side));
    var tx = 1e9;
    var ty = 1e9;
    if abs(dir.x) > 1e-5 {
        tx = (sign(dir.x) * half.x - local.x) / dir.x;
    }
    if abs(dir.y) > 1e-5 {
        ty = (sign(dir.y) * half.y - local.y) / dir.y;
    }
    let t = max(min(tx, ty), 0.0);
    let out = p + way * t;
    let over = out.z - a.z;
    if over >= a.w - a.z {
        return false;
    }
    var wall = vec2<f32>(0.0);
    var run = vec2<f32>(0.0);
    var on = 0.0;
    var wide = 0.0;
    if tx < ty {
        wall = along * sign(dir.x);
        run = side;
        on = local.y + dir.y * t;
        wide = b.y;
    } else {
        wall = side * sign(dir.y);
        run = along;
        on = local.x + dir.x * t;
        wide = b.x;
    }
    if doorOf(out, wall, c, a.z, deep) {
        return true;
    }
    let pane = paneOf(on, over, wide, d.y, d.x, d.z, doorIn(c, out.xy, on, wall, run, deep));
    return pane.z > 0.5 && pane.y < 0.5 &&
        groundHeightAt(out.xy + wall * (deep + 0.5)) < a.z + U.SolidPane.y * d.x;
}

// sunIn is how much of the sun's light the point p of a room meets along the way toward the sun:
// all where that leaves its walls in an opening of their inner faces and of their outer, none
// where a wall, a reveal or the ceiling stops it
fn sunIn(p: vec3<f32>, way: vec3<f32>, a: vec4<f32>, b: vec4<f32>, c: vec4<f32>,
    d: vec4<f32>, thick: f32) -> f32 {
    if opening(p, way, a, b, c, d, b.xy - vec2<f32>(thick), thick) &&
        opening(p, way, a, b, c, d, b.xy, 0.0) {
        return 1.0;
    }
    return 0.0;
}

@fragment
fn fs_main(s: Inner) -> @location(0) vec4<f32> {
    let storey = max(s.d.x, 1e-3);
    // how many storeys a pixel spans, taken before anything is left out
    let span = length(fwidth(s.world)) / storey;
    if s.colour.w > 0.5 && s.colour.w < 1.5 {
        // the openings of a wall's inner face: the door, the panes of the room's storey, laid out
        // on its outer face
        let way = -s.normal.xy;
        let door = doorIn(s.c, s.world.xy, s.at.x, way, vec2<f32>(way.y, -way.x), s.thick);
        let pane = paneOf(s.at.x, s.at.y, s.at.z, s.d.y, s.d.x, s.d.z, door);
        if doorOf(s.world, way, s.c, s.a.z, s.thick) || (pane.z > 0.5 && pane.y < 0.5 &&
            groundHeightAt(s.world.xy + way * (s.thick + 0.5)) <
            s.a.z + U.SolidPane.y * s.d.x) {
            discard;
        }
    }
    let sun = lightWay();
    // its own lamp burns as the sky's light goes, as the windows glow
    let dusk = 1.0 - smoothstep(0.08, 0.2, dot(U.Ambience, vec3<f32>(0.3, 0.59, 0.11)));
    let lamps = U.SolidLamp * s.d.w * dusk + lampLightAt(s.world);
    var rgb = s.colour.rgb;
    var light = lamps;
    if s.colour.w > 2.5 {
        // a wall's section where it is cut: the plaster of its outside in the outside's light
        let off = s.world.xy - s.a.xy;
        let q = vec2<f32>(dot(off, s.b.zw), dot(off, vec2<f32>(-s.b.w, s.b.z))) / storey;
        rgb = plaster(rgb, q + vec2<f32>(0.0, 9.0), span);
        light += U.Ambience + U.LightColor * U.LightStrength * max(dot(s.normal, sun), 0.0);
    } else {
        light += U.Ambience * U.SolidIndoor.x;
        if sun.z > 0.0 {
            // what falls in thrown round the room, and the sun itself where it comes in
            light += U.LightColor * U.LightStrength * (U.SolidIndoor.y * sun.z +
                max(dot(s.normal, sun), 0.0) * sunIn(s.world, sun, s.a, s.b, s.c, s.d, s.thick));
        }
        if s.colour.w < 0.5 {
            // the floor's boards across the room
            rgb *= 0.92 + 0.08 * step(0.5, fract(dot(s.world.xy - s.a.xy, s.b.zw) / 6.0));
        } else if s.colour.w < 1.5 {
            rgb = plaster(rgb, s.at.xy / storey, span);
        }
    }
    rgb *= light;
    if U.SolidHaze.x > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(s.world) / U.SolidHaze.x));
    }
    if U.SolidSight.w > 0.0 {
        let far = distance(s.world.xy, U.SolidSight.xy);
        rgb = mix(rgb, U.Fog, smoothstep(U.SolidSight.z, U.SolidSight.w, far));
    }
    return vec4<f32>(rgb, 1.0);
}
