// A building's room an instance, seen from inside: its floor and the inner faces of the two walls
// across it from the eye, their openings left out, lit by the sun where it comes in by a window or
// the door (sunIn), by as much of the sky as gets in, by the room's own lamp after dusk and by the
// frame's lamps.

struct Inner {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) @interpolate(flat) normal: vec3<f32>,
    // along the wall, world units from its middle, and over the floor; a wall's half width
    @location(2) at: vec3<f32>,
    @location(3) @interpolate(flat) a: vec4<f32>,
    @location(4) @interpolate(flat) b: vec4<f32>,
    @location(5) @interpolate(flat) c: vec4<f32>,
    @location(6) @interpolate(flat) d: vec4<f32>,
    @location(7) @interpolate(flat) colour: vec4<f32>,
}

const floorVertices = 6u;

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) a: vec4<f32>, // its middle, its floor and its ceiling
    @location(1) b: vec4<f32>, // half its length and its width, the way its length runs
    @location(2) c: vec4<f32>, // its door: its middle, half its width and its height
    @location(3) d: vec4<f32>, // a storey, a bay, the eaves over the floor, its lamp's strength
    @location(4) e: vec4<f32>, // the floor's colour
    @location(5) f: vec4<f32>, // its walls' colour
) -> Inner {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var up = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let half = b.xy;
    let u = across[vid % 6u] * 2.0 - 1.0;
    let v = up[vid % 6u];
    var p = vec3<f32>(0.0);
    var n = vec3<f32>(0.0, 0.0, 1.0);
    var at = vec3<f32>(0.0);
    var colour = vec4<f32>(e.rgb, 0.0);
    if vid < floorVertices {
        p = vec3<f32>(a.xy + along * u * half.x + side * (v * 2.0 - 1.0) * half.y, a.z);
    } else if vid < floorVertices + 6u {
        // the long wall across from the eye, its inner face toward it
        let sign = -facing(a.xy, side * half.y, along * half.x, a.z, a.w);
        p = vec3<f32>(a.xy + along * u * half.x + side * half.y * sign, mix(a.z, a.w, v));
        n = vec3<f32>(-side * sign, 0.0);
        at = vec3<f32>(u * half.x, p.z - a.z, half.x);
        colour = vec4<f32>(f.rgb, 1.0);
    } else {
        let sign = -facing(a.xy, along * half.x, -side * half.y, a.z, a.w);
        p = vec3<f32>(a.xy + along * half.x * sign - side * u * half.y, mix(a.z, a.w, v));
        n = vec3<f32>(-along * sign, 0.0);
        at = vec3<f32>(u * half.y, p.z - a.z, half.y);
        colour = vec4<f32>(f.rgb, 1.0);
    }
    var s: Inner;
    s.clip = clipOf(p);
    s.world = p;
    s.normal = n;
    s.at = at;
    s.a = a;
    s.b = b;
    s.c = c;
    s.d = d;
    s.colour = colour;
    return s;
}

// sunIn is how much of the sun's light the point p of a room meets along the way toward the sun:
// all where that leaves the room's walls under its ceiling in its door or in a pane of its
// storey's windows with the ground outside under its sill, none where a wall or the ceiling
// stops it
fn sunIn(p: vec3<f32>, way: vec3<f32>, a: vec4<f32>, b: vec4<f32>, c: vec4<f32>,
    d: vec4<f32>) -> f32 {
    let along = b.zw;
    let side = vec2<f32>(-along.y, along.x);
    let q = p.xy - a.xy;
    let local = vec2<f32>(dot(q, along), dot(q, side));
    let dir = vec2<f32>(dot(way.xy, along), dot(way.xy, side));
    var tx = 1e9;
    var ty = 1e9;
    if abs(dir.x) > 1e-5 {
        tx = (sign(dir.x) * b.x - local.x) / dir.x;
    }
    if abs(dir.y) > 1e-5 {
        ty = (sign(dir.y) * b.y - local.y) / dir.y;
    }
    let t = max(min(tx, ty), 0.0);
    let out = p + way * t;
    let over = out.z - a.z;
    if over >= a.w - a.z {
        return 0.0;
    }
    var wall = vec2<f32>(0.0);
    var on = 0.0;
    var wide = 0.0;
    if tx < ty {
        wall = along * sign(dir.x);
        on = local.y + dir.y * t;
        wide = b.y;
    } else {
        wall = side * sign(dir.y);
        on = local.x + dir.x * t;
        wide = b.x;
    }
    if doorOf(out, wall, c, a.z) {
        return 1.0;
    }
    let pane = paneOf(on, over, wide, d.y, d.x, d.z);
    if pane.z > 0.5 && pane.y < 0.5 &&
        groundHeightAt(out.xy + wall * 0.5) < a.z + U.SolidPane.y * d.x {
        return 1.0;
    }
    return 0.0;
}

@fragment
fn fs_main(s: Inner) -> @location(0) vec4<f32> {
    let wall = s.colour.w > 0.5;
    if wall {
        // the openings of the wall: the door, the panes of the room's storey
        let way = -s.normal.xy;
        let pane = paneOf(s.at.x, s.at.y, s.at.z, s.d.y, s.d.x, s.d.z);
        if doorOf(s.world, way, s.c, s.a.z) || pane.z > 0.5 && pane.y < 0.5 &&
            groundHeightAt(s.world.xy + way * 0.5) < s.a.z + U.SolidPane.y * s.d.x {
            discard;
        }
    }
    let sun = lightWay();
    var light = U.Ambience * U.SolidIndoor.x;
    if sun.z > 0.0 {
        // what falls in thrown round the room, and the sun itself where it comes in
        light += U.LightColor * U.LightStrength * (U.SolidIndoor.y * sun.z +
            max(dot(s.normal, sun), 0.0) * sunIn(s.world, sun, s.a, s.b, s.c, s.d));
    }
    // its own lamp burns as the sky's light goes, as the windows glow
    let dusk = 1.0 - smoothstep(0.08, 0.2, dot(U.Ambience, vec3<f32>(0.3, 0.59, 0.11)));
    light += U.SolidLamp * s.d.w * dusk + lampLightAt(s.world);
    var rgb = s.colour.rgb;
    if !wall {
        // the floor's boards across the room
        rgb *= 0.92 + 0.08 * step(0.5, fract(dot(s.world.xy - s.a.xy, s.b.zw) / 6.0));
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
