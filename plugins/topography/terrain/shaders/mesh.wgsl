// The ground's mesh: every lattice corner a vertex, two triangles a cell (terrain.Renderer builds
// them), drawn through U.ViewProj (camera.Transform) with the ground d off U.Eye sunk by U.Bend·d².

struct Ground {
    @builtin(position) @invariant clip: vec4<f32>,
    @location(0) world: vec3<f32>,
}

// vs_main stands the lattice corner vid, counted row by row, at its height.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Ground {
    let cols = u32(U.Corners.x);
    let i = vec2<f32>(f32(vid % cols), f32(vid / cols));
    let p = vec3<f32>(i * U.Cell, corner(i));
    let off = p.xy - U.Eye.xy;
    var g: Ground;
    g.clip = U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
    g.world = p;
    return g;
}

// fs_prime lays the ground's depth alone, before fs_main shades only what is seen of it.
@fragment
fn fs_prime() -> @location(0) vec4<f32> {
    return vec4<f32>(0.0);
}

// fs_main is the ground's colour at its point in the sun's light, turned to the Fog as far off
// as it lies.
@fragment
fn fs_main(g: Ground) -> @location(0) vec4<f32> {
    let p = g.world;
    // how far p moves over the ground for a screen pixel, and how many a cell spans there
    let moveX = dpdx(p);
    let moveY = dpdy(p);
    let span = U.Cell / max(min(length(moveX.xy), length(moveY.xy)), 1e-6);
    let n = normal(p.xy);
    let sun = sunWay();
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
        if U.Shadows > 0.5 {
            lit = baked(p.xy, 0.0, U.ShadePx).r;
        }
    }
    var rgb = albedo(p.xy) * (U.Ambience + U.SunColor * max(dot(n, sun), 0.0) * lit * U.SunStrength);
    var cover = 0.0;
    if U.Cover > 0.0 {
        cover = baked(p.xy, U.CloudFrom, U.CloudPx).r;
    }
    let wet = lit * (1.0 - cover); // the sun on the water, the clouds' shadow taken off
    let detail = clamp((span - 6.0) / 6.0, 0.0, 1.0);
    var run = vec3<f32>(0.0);
    var still = vec3<f32>(0.0);
    var mouth = vec3<f32>(0.0);
    if U.WaterPx > 0.0 {
        run = waterAt(p.xy, vec2<f32>(0.0, 0.0));
        still = waterAt(p.xy, vec2<f32>(1.0, 0.0));
        mouth = waterAt(p.xy, vec2<f32>(0.0, 1.0));
    }
    // the sea's glint, the waves turning to the shore and breaking on it near enough
    if still.b > 0.004 && detail > 0.0 {
        var shore = vec4<f32>(0.0);
        if span >= 16.0 {
            shore = shoreAt(p.xy);
        }
        rgb = over(SeaGlint(p.xy, still.g / still.b * detail, wet, shore) * still.b, rgb);
    }
    if cover > 0.0 {
        rgb *= 1.0 - cloudShade(cover);
    }
    if U.GridFrom > 0.0 && span >= U.GridFrom {
        rgb *= 1.0 - outlineDark * edge(p.xy, moveX, moveY);
    }
    // the water running down the ways and the cells it runs over, and where a way turns into water
    if run.b > 0.004 && span > 16.0 {
        let v = (run.rg / run.b * 2.0 - 1.0) * U.FlowSpan;
        rgb = over(RunningWater(p.xy, still.r / run.b * clamp((span - 16.0) / 8.0, 0.0, 1.0), wet, vec4<f32>(v, 0.0, 0.0)) * run.b, rgb);
    }
    if mouth.g > 0.004 && detail > 0.0 {
        rgb = over(SeaGlint(p.xy, mouth.r / mouth.g * detail, wet, vec4<f32>(0.0)) * mouth.g, rgb);
    }
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-distance(U.Eye, p) / U.Visibility));
    }
    return vec4<f32>(rgb, 1.0);
}

// baked is the fourth image's part from x across, px pixels a cell over the lattice's cells, at
// p: blended between its pixels, held within the part — the shade (shade.wgsl) and the clouds
// (cover.wgsl) the Renderer bakes.
fn baked(p: vec2<f32>, x: f32, px: f32) -> vec4<f32> {
    let size = (U.Corners - 1.0) * px;
    let t = clamp(p / U.Cell * px, vec2<f32>(0.5), size - 0.5);
    return imageSrc3Linear(imageSrc3Origin() + vec2<f32>(x, 0.0) + t);
}
