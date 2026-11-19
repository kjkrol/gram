// The world's bodies whole in relief (topography's sprites, world.BodyLook): an instance an
// entity — where its foot stands and how tall its body is; how wide, its kind's row in the
// bodies' table (image0), how it is turned about its foot, radians, and how many parts it has;
// the light it is drawn in and how hazed it is; how much of the light is the sun's, the ground
// under its foot, the shade it takes (render.Taking) and how much it sways in the wind. Each
// part is a card met in the pixel (world's body.wgsl), leant with the wind's gusts as high as
// it stands, lying as deep as where it is met and never nearer than its card; one giving way
// (givenWay) is left out where it is met nearer than something uncovered.

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) foot: vec4<f32>,
    @location(1) kind: vec4<f32>,
    @location(2) light: vec4<f32>,
    @location(3) ground: vec4<f32>,
) -> ShapeCard {
    var c: ShapeCard;
    c.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0);
    let slot = f32(vid / 6u);
    if slot >= kind.w {
        return c;
    }
    let col = shapeRowTexels + shapePartTexels * slot;
    let t0 = shapeTexel(col, kind.y);
    let t1 = shapeTexel(col + 1.0, kind.y);
    let t2 = shapeTexel(col + 2.0, kind.y);
    if round(t2.x * 255.0) < 0.5 {
        return c;
    }
    let turn = vec2<f32>(cos(kind.z), sin(kind.z));
    let back = vec2<f32>(turn.x, -turn.y);
    // in the body's frame: its top leans with the wind as far as it stands high
    let lean = shapeTurned(vec3<f32>(windLean(ground.w, foot.xy), 0.0), back);
    var a = shapePoint(t0, kind.x, foot.w);
    var b = shapePoint(t1, kind.x, foot.w);
    a += lean * a.z;
    b += lean * b.z;
    let ortho = U.Perspective < 0.5;
    var ahead = normalize(U.Look);
    if !ortho {
        ahead = normalize(foot.xyz + shapeTurned(0.5 * (a + b), turn) - U.Eye);
    }
    let p = shapeCardOf(&c, vid, a, b, t0.w * kind.x, t1.w * kind.x, t2, kind.y, kind.x,
        shapeTurned(ahead, back), shapeTurned(U.Eye - foot.xyz, back), ortho, slot * 7.31);
    c.more.y = turn.x;
    c.more.z = turn.y;
    c.clip = project(foot.xyz + shapeTurned(p, turn));
    c.deep = c.clip.zw;
    // the clip's z and w go along the line of sight as they go along its way, or from the eye's
    // to the card's
    var deep = (U.ViewProj * vec4<f32>(ahead, 0.0)).zw;
    if !ortho {
        deep = (U.ViewProj * vec4<f32>(U.Eye, 1.0)).zw;
    }
    c.lit.w = deep.x;
    c.dots.w = deep.y;
    // the shade it stands in, between its foot's and its top's as a billboard blends them: a
    // thing stands in its own block's shadow, lit the more the higher
    let up = clamp(0.5 * (a.z + b.z) / max(foot.w, 1e-3), 0.0, 1.0);
    let shade = mix(shaded(foot, ground.y, 0.1, ground.x, kind.x, ground.z),
        shaded(foot, ground.y, 1.0, ground.x, kind.x, ground.z), up);
    // how hazed it is, two more where it gives way: the card has no varying to spare
    var hazed = light.w;
    if takes(ground.z, givesWay) {
        hazed += 2.0;
    }
    c.light = vec4<f32>(light.rgb * shade, hazed);
    return c;
}

struct Met {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

// The part lies no nearer than its card, which stands on its front, and says so: a pixel of
// the card behind what is drawn is then never worked out.
@fragment
@early_depth_test(less_equal)
fn fs_main(c: ShapeCard) -> Met {
    let ortho = U.Perspective < 0.5;
    let m = shapeMet(c, ortho);
    if m.far < 0.0 {
        discard;
    }
    let sun = shapeTurned(lightWay(), vec2<f32>(c.more.y, -c.more.z));
    let rgb = shapeColour(c, m) * shapeTone(shapeFacing(m, sun)) * c.light.rgb;
    var deep = c.deep;
    var along = vec2<f32>(c.lit.w, c.dots.w);
    if !ortho {
        deep = along;
        along = c.deep - deep;
    }
    let lies = (deep.x + m.far * along.x) / max(deep.y + m.far * along.y, 1e-6);
    let depth = min(clamp(lies, 0.0, 1.0), c.clip.z);
    var hazed = c.light.a;
    if hazed > 1.5 {
        hazed -= 2.0;
        if givenWay(f32(givesWay), c.clip.xy, depth) {
            discard;
        }
    }
    return Met(vec4<f32>(mix(rgb, U.Fog, hazed), 1.0), depth);
}
