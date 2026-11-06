// The sheets of what stands drawn over the ground (terrain's photo.go) at LEVEL, L4 or L5: an
// instance a sector whose sheet holds a photograph of something, its box (x0, y0, x1, y1), a
// lattice of U.SheetQuads quads a side over it, a vertex a corner, row by row. A corner stands
// at the highest top the relief (the third image, U.SheetPx texels a world unit, a float16 in red
// and green) holds within U.SheetReach texels of it, a cell under the ground (the lattice, the
// first image) where nothing stands so near, sought at L4 first in the albedo at L5 (the fourth
// image); drawn through U.ViewProj sunk U.Bend·d² under U.Eye's level. A point shows the
// albedo (the second image), premultiplied, nothing where nothing stands, lit as the relief faces
// there ((n+1)/2 in blue and alpha) as the ground of its level is lit, shaded by the clouds at
// L4, hazed and fogged as the ground is.

struct Photo {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) cloud: f32, // how much of the sun the clouds take
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32, @location(0) box: vec4<f32>) -> Photo {
    let n = u32(U.SheetQuads);
    let c = vec2<f32>(f32(vid % (n + 1u)), f32(vid / (n + 1u))) / f32(n);
    let xy = mix(box.xy, box.zw, c);
    let p = vec3<f32>(xy, topNear(xy));
    let off = p.xy - U.Eye.xy;
    var out = Photo(U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0), p, 0.0);
    if LEVEL <= 4u && U.Cover > 0.0 && U.CloudsShadeGround > 0.5 {
        out.cloud = cloudsShadeAt(p, U.Cell);
    }
    return out;
}

// topNear is the highest top the relief holds within U.SheetReach texels of p, a cell under the
// ground there where nothing stands so near: a quad over a thing narrower than it stands at its
// top at every corner.
fn topNear(p: vec2<f32>) -> f32 {
    let at = floor(p * U.SheetPx);
    let reach = i32(U.SheetReach);
    if LEVEL == 4u && !coarselyNear(at, reach) {
        return height(p) - U.Cell;
    }
    var top = -3.0e38;
    for (var dy = -reach; dy <= reach; dy++) {
        for (var dx = -reach; dx <= reach; dx++) {
            let t = at + vec2<f32>(f32(dx), f32(dy)) + 0.5;
            if imageSrc1At(imageSrc1Origin() + t).a > 0.0 {
                let r = imageSrc2At(imageSrc2Origin() + t);
                let bits = u32(r.r * 255.0 + 0.5) | (u32(r.g * 255.0 + 0.5) << 8u);
                top = max(top, unpack2x16float(bits).x);
            }
        }
    }
    if top < -1.0e38 {
        return height(p) - U.Cell;
    }
    return top;
}

// coarselyNear reports whether something stands within reach texels at L4 of the texel at, as
// the albedo at L5, half as fine, has it: most corners have nothing near, sought so at a quarter
// of the reads.
fn coarselyNear(at: vec2<f32>, reach: i32) -> bool {
    let c = floor(at * 0.5);
    let r = reach / 2 + 1;
    for (var dy = -r; dy <= r; dy++) {
        for (var dx = -r; dx <= r; dx++) {
            let t = c + vec2<f32>(f32(dx), f32(dy)) + 0.5;
            if imageSrc3At(imageSrc3Origin() + t).a > 0.0 {
                return true;
            }
        }
    }
    return false;
}

@fragment
fn fs_main(g: Photo) -> @location(0) vec4<f32> {
    let t = g.world.xy * U.SheetPx;
    let albedo = imageSrc1Linear(imageSrc1Origin() + t);
    if albedo.a < 0.02 {
        discard;
    }
    let xy = imageSrc2Linear(imageSrc2Origin() + t).ba * 2.0 - 1.0;
    let n = vec3<f32>(xy, sqrt(max(1.0 - dot(xy, xy), 0.0)));
    let sun = lightWay();
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
    }
    var diffuse = max(dot(n, sun), 0.0);
    if U.Bands > 0.5 {
        diffuse = banded(diffuse, U.Bands);
    }
    var sunColor = U.LightColor;
    if U.Warmth > 0.0 {
        sunColor = mix(U.LightColor, U.LightColor * vec3<f32>(1.08, 0.92, 0.7), U.Warmth);
    }
    let shade = 1.0 - diffuse * lit;
    var light = U.Ambience * mix(vec3<f32>(1.0), U.ShadowTint, shade);
    light += sunColor * diffuse * lit * U.LightStrength;
    var rgb = albedo.rgb * light;
    if g.cloud > 0.0 {
        rgb *= 1.0 - cloudShade(g.cloud);
    }
    // the fog over a colour premultiplied
    let fog = U.Fog * albedo.a;
    if U.Visibility > 0.0 {
        rgb = mix(rgb, fog, 1.0 - exp(-hazeDepth(g.world) / U.Visibility));
    }
    if U.Sight.w > 0.0 {
        rgb = mix(rgb, fog, smoothstep(U.Sight.z, U.Sight.w, distance(g.world.xy, U.Sight.xy)));
    }
    return vec4<f32>(rgb, albedo.a);
}

// banded is the light d quantised into bands steps, each edge softened a little, as the ground's.
fn banded(d: f32, bands: f32) -> f32 {
    let b = d * bands;
    let k = floor(b);
    return (k + smoothstep(0.42, 0.58, b - k)) / bands;
}

// hazeDepth is how much air lies between the eye and p, as the ground's.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}
