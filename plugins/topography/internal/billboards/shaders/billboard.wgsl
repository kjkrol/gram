// The world's entities in relief, drawn on the GPU (topography's sprites): an instance an entity —
// where its feet stand and how tall it is; how wide, how much it sways in the wind, how it lies
// and how hazed it is; its sprite's corners on the atlas, the first image; the light it is drawn
// in and, a body's card, how high over its feet it leans and lies about (world's Bodied.TopAt);
// the ground under its feet, the shade it takes (render.Taking) and, one past each, where on the
// atlas the ways out of a body's card lie, 0 for a plain sprite — a billboard standing upright in
// the world across U.Right, the screen's way across, as tall on the screen as what stands there,
// at the depths of a thing standing upright at its feet, nudged towards the eye: the ground
// before it hides it, the ground it stands on and the slope behind it do not. A body's card
// reaches a margin under its feet and over its top (shapeMargin) and, seen from steeper than it
// was photographed, leans back about that height with its rows spread (shapeRow), still as deep
// as a thing upright at its feet: what it holds there as tall on the screen as the body's, what
// stands under it going under the ground as the body hides it. Its card from above lies
// instead: held over its feet, turned about them by how it is said to lie, a whole turn on (0
// what stands), and tipped up to face the eye, at its own depths. U.SunShare of the light is
// the sun's.

struct Board {
    @builtin(position) clip: vec4<f32>,
    @location(0) uv: vec2<f32>,
    @location(1) @interpolate(flat) light: vec4<f32>,
    @location(2) shade: f32,
    // a body's card: how far off its sprite the ways out of it lie on the atlas, and 1
    @location(3) @interpolate(flat) ways: vec3<f32>,
    // and the way toward the light as its ways are told (cardSun)
    @location(4) @interpolate(flat) sun: vec3<f32>,
    // the shade it takes, with whether it gives way (givenWay) and the shelter it is under
    @location(5) @interpolate(flat) taking: f32,
    // where in the world the pixel lies, for the light that gets under a shelter
    @location(6) world: vec3<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) feet: vec4<f32>,
    @location(1) shape: vec4<f32>,
    @location(2) uv: vec4<f32>,
    @location(3) light: vec4<f32>,
    @location(4) ground: vec4<f32>,
) -> Board {
    var across = array<f32, 6>(0.0, 1.0, 0.0, 1.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let a = across[vid];
    let up = 1.0 - down[vid];
    // its top leans with the wind as far as it stands high (the air's gust.wgsl)
    let lean = windLean(shape.y, feet.xy);
    // a body's card is seen along d at the height it leans about, light.w over its feet
    let mid = feet.xyz + vec3<f32>(lean * light.w, light.w);
    var d = normalize(U.Look);
    var margin = 0.0;
    var row = vec2<f32>(0.0, 1.0);
    if ground.z > 0.5 {
        if U.Perspective > 0.5 {
            d = normalize(mid - U.Eye);
        }
        margin = shapeMargin(shape.x);
        row = shapeRow(d, shape.x, feet.w - light.w);
    }
    // up its picture from that height, and back off the eye as far as the card leans
    let pictured = feet.w + 2.0 * margin;
    let over = up * pictured - light.w - margin;
    let back = vec2<f32>(U.Right.y, -U.Right.x) * (over * row.x);
    var p = feet.xyz + U.Right * ((a - 0.5) * shape.x)
        + vec3<f32>(back + lean * pictured * up, light.w + over * row.y);
    var q = upright(p, feet.xyz);
    // the shade it stands in, from its feet's to its top's
    var shade = shaded(feet, ground.x, mix(0.1, 1.0, up), U.SunShare, shape.x, ground.y);
    if shape.z > 0.5 {
        var ahead = vec2<f32>(U.Right.y, -U.Right.x);
        if dot(d.xy, d.xy) > 1e-6 {
            ahead = normalize(d.xy);
        }
        let right = vec2<f32>(-ahead.y, ahead.x);
        // the picture's way across is the body's x and its way up against the body's y
        let east = vec2<f32>(cos(shape.z), sin(shape.z));
        let on = (east * (a - 0.5) + vec2<f32>(east.y, -east.x) * (up - 0.5)) * shape.x;
        // level, what of it runs on from the eye tipped up across the line of sight
        p = mid + vec3<f32>(right * dot(on, right), 0.0)
            + vec3<f32>(ahead * -d.z, length(d.xy)) * dot(on, ahead);
        q = p;
        // as a part of the body as high: a thing stands in its own shadow, lit the more the
        // higher
        shade = mix(shaded(feet, ground.x, 0.1, U.SunShare, shape.x, ground.y),
            shaded(feet, ground.x, 1.0, U.SunShare, shape.x, ground.y),
            clamp(light.w / max(feet.w, 1e-3), 0.0, 1.0));
    }
    let at = project(q + nudge(q));
    var b: Board;
    b.clip = project(p);
    b.clip.z = at.z / max(at.w, 1e-6) * b.clip.w;
    if project(feet.xyz).w <= 0.0 {
        b.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0); // its feet behind the eye: none of it
    }
    b.uv = mix(uv.xy, uv.zw, vec2<f32>(a, down[vid]));
    b.light = vec4<f32>(light.rgb, shape.w);
    b.shade = shade;
    b.world = p;
    b.ways = vec3<f32>(0.0);
    b.sun = vec3<f32>(0.0);
    b.taking = ground.y;
    if ground.z > 0.5 {
        b.ways = vec3<f32>(ground.zw - 1.0 - uv.xy, 1.0);
        b.sun = cardSun(shape.z);
    }
    return b;
}

// cardSun is the way toward the light as a body's card holds the ways out of it: across its
// picture, up it and toward the picture's eye. That eye looked down the card's own way ahead
// from the cards' elevation, or straight down on the body turned by lies.
fn cardSun(lies: f32) -> vec3<f32> {
    let sun = lightWay();
    if lies > 0.5 {
        let east = vec2<f32>(cos(lies), sin(lies));
        return vec3<f32>(dot(sun.xy, east), dot(sun.xy, vec2<f32>(east.y, -east.x)), sun.z);
    }
    let ahead = vec2<f32>(U.Right.y, -U.Right.x);
    let e = vec2<f32>(cos(shapeElevation), sin(shapeElevation));
    return vec3<f32>(dot(sun, U.Right), dot(sun, vec3<f32>(ahead * e.y, e.x)),
        dot(sun, vec3<f32>(-ahead * e.x, e.y)));
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

// fs_main is the sprite's texel in its light, turned to the Fog as hazed as it is; what is less
// than half there is left out, the depth kept for what shows, and so is what gives way over
// something uncovered. A body's card is its paint in the
// tone of the way out of the body there (world's body.wgsl), as the body whole is lit, and no
// darker than its paint where that shines.
@fragment
fn fs_main(b: Board) -> @location(0) vec4<f32> {
    let c = imageSrc0At(b.uv);
    if c.a < 0.5 || givenWay(b.taking, b.clip.xy, b.clip.z) {
        discard;
    }
    var rgb = c.rgb;
    var glow = vec3<f32>(0.0);
    if b.ways.z > 0.5 {
        let ways = imageSrc0At(b.uv + b.ways.xy);
        rgb *= shapeTone(cardFacing(ways, b.sun));
        if shapeWaysShine(ways.b) {
            glow = c.rgb;
        }
    }
    var light = b.light.rgb * b.shade;
    let shelter = shelterOf(b.taking);
    if shelter > 0u {
        // under a roof, the light that gets in at the pixel and none of the open air's shade
        light = shelteredLight(b.world, shelter - 1u, b.light.rgb);
    }
    return vec4<f32>(mix(max(rgb * light, glow), U.Fog * c.a, b.light.a), c.a);
}

// cardFacing is how squarely a body faces the light where its card's texel of ways says: the
// way out of it across and up the picture it was photographed in, the rest toward that
// picture's eye, against the way toward the light told so (cardSun); a cone the less the
// further up.
fn cardFacing(ways: vec4<f32>, sun: vec3<f32>) -> f32 {
    let across = ways.rg * 2.0 - 1.0;
    let toward = sqrt(max(1.0 - dot(across, across), 0.0));
    return dot(vec3<f32>(across, toward), sun) - shapeUnder * shapeWaysUp(ways.b);
}
