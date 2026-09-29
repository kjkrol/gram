// The world's entities in relief, drawn on the GPU (topography's sprites): an instance an entity —
// where its feet stand and how tall it is; how wide, how far its top leans with the wind and how
// hazed it is; its sprite's corners on the atlas, the first image; the light it is drawn in — a
// billboard upright on the screen, U.Right and U.Up the screen's ways in the world, at the depths
// of a thing standing upright at its feet, nudged towards the eye: the ground before it hides it,
// the ground it stands on and the slope behind it do not.

struct Board {
    @builtin(position) clip: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) feet: vec4<f32>, @location(1) shape: vec4<f32>, @location(2) uv: vec4<f32>, @location(3) light: vec4<f32>) -> Board {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let a = across[vid];
    let up = 1.0 - down[vid];
    let base = feet.xyz;
    let p = base + U.Right * ((a - 0.5) * shape.x) + U.Up * (up * feet.w) + vec3<f32>(shape.yz * up, 0.0);
    let q = upright(p, base);
    let at = project(q + nudge(q));
    var b: Board;
    b.clip = project(p);
    b.clip.z = at.z / max(at.w, 1e-6) * b.clip.w;
    if project(base).w <= 0.0 {
        b.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0); // its feet behind the eye: none of it
    }
    b.uv = mix(uv.xy, uv.zw, vec2<f32>(a, down[vid]));
    b.light = vec4<f32>(light.rgb, shape.w);
    return b;
}

// project is where the world point p goes in clip space, sunk under the eye's level as far off as
// it lies, as the ground is.
fn project(p: vec3<f32>) -> vec4<f32> {
    let off = p.xy - U.Eye.xy;
    return U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
}

// upright is where the line of sight through the billboard's point p meets the upright plane
// through its feet base, facing the eye: where a thing standing there upright is seen at p. Looking
// straight down there is no such plane: the feet.
fn upright(p: vec3<f32>, base: vec3<f32>) -> vec3<f32> {
    if U.Perspective > 0.5 {
        let across = vec3<f32>((base - U.Eye).xy, 0.0);
        let v = p - U.Eye;
        if length(across) < 1e-3 || dot(v, across) <= 0.0 {
            return base;
        }
        return U.Eye + v * (dot(base - U.Eye, across) / dot(v, across));
    }
    let d = normalize(U.Look);
    let across = vec3<f32>(d.xy, 0.0);
    if length(across) < 1e-3 {
        return base;
    }
    return p - d * (dot(p - base, across) / dot(d, across));
}

// nudge moves the world point p towards the eye, so the ground p stands on passes behind it.
fn nudge(p: vec3<f32>) -> vec3<f32> {
    var toward = -U.Look;
    if U.Perspective > 0.5 {
        toward = U.Eye - p;
    }
    return normalize(toward) * spriteLift;
}

// fs_main is the sprite's texel in its light, turned to the Fog as hazed as it is; what is less
// than half there is left out, the depth kept for what shows.
@fragment
fn fs_main(b: Board) -> @location(0) vec4<f32> {
    let c = imageSrc0At(b.uv);
    if c.a < 0.5 {
        discard;
    }
    return vec4<f32>(mix(c.rgb * b.light.rgb, U.Fog * c.a, b.light.a), c.a);
}

// spriteLift is how far towards the eye a billboard's depth is taken, world units.
const spriteLift: f32 = 2.0;
