// The sheets of what stands drawn over the ground (terrain's photo.go) at LEVEL, L4 or L5: an
// instance a sector whose sheet holds a photograph of something, its box (x0, y0, x1, y1) and
// where its corners lie among the tops (x, y), a lattice of U.SheetQuads quads a side over it, a
// vertex a corner, row by row. A corner stands at the top tops.wgsl worked out for it (the fourth
// image), a cell under the ground (the lattice, the first image) where nothing stands near it;
// drawn through U.ViewProj sunk U.Bend·d² under U.Eye's level. A point shows the albedo (the
// second image, U.SheetPx texels a world unit), premultiplied, nothing where nothing stands, lit
// as the relief (the third image) faces there ((n+1)/2 in blue and alpha) as the ground of its
// level is lit, shaded by the clouds at L4, hazed and fogged as the ground is.

struct Photo {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) cloud: f32, // how much of the sun the clouds take
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) box: vec4<f32>,
    @location(1) corners: vec4<f32>,
) -> Photo {
    let n = u32(U.SheetQuads);
    let at = vec2<f32>(f32(vid % (n + 1u)), f32(vid / (n + 1u)));
    let xy = mix(box.xy, box.zw, at / f32(n));
    let p = vec3<f32>(xy, topAt(corners.xy + at, xy));
    let off = p.xy - U.Eye.xy;
    var out = Photo(U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0), p, 0.0);
    if LEVEL <= 4u && U.Cover > 0.0 && U.CloudsShadeGround > 0.5 {
        out.cloud = cloudsShadeAt(p, U.Cell);
    }
    return out;
}

// topAt is the top the corner at among the tops stands at, xy in the world, a cell under the
// ground there where nothing stands near it.
fn topAt(at: vec2<f32>, xy: vec2<f32>) -> f32 {
    let t = imageSrc3At(imageSrc3Origin() + at + 0.5);
    if t.b < 0.5 {
        return height(xy) - U.Cell;
    }
    let bits = u32(t.r * 255.0 + 0.5) | (u32(t.g * 255.0 + 0.5) << 8u);
    return unpack2x16float(bits).x;
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
