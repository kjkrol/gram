// The ground's mesh: every lattice corner a vertex, two triangles a cell, and round the world a
// skirt of U.Skirt.y rings reaching U.Skirt.x times as far from its middle as its edge, the edge's
// ground running on out to the horizon (terrain.Renderer builds them); drawn through U.ViewProj
// (camera.Transform) with the ground d off U.Eye sunk by U.Bend·d², so the skirt sinks under the
// horizon as the world's curve would have it.

struct Ground {
    @builtin(position) @invariant clip: vec4<f32>,
    @location(0) world: vec3<f32>,
}

// vs_main stands the lattice corner vid, counted row by row, at its height; past the lattice's
// corners, the skirt's: ring after ring, each round the edge clockwise from the top-left, pushed
// out from the middle, at the height of the edge's corner it runs out from.
@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Ground {
    let cols = u32(U.Corners.x);
    let lattice = cols * u32(U.Corners.y);
    var i = vec2<f32>(f32(vid % cols), f32(vid / cols));
    var xy = i * U.Cell;
    if vid >= lattice {
        let k = vid - lattice;
        let rim = 2u * (cols - 1u) + 2u * (u32(U.Corners.y) - 1u);
        i = rimCorner(k % rim);
        let mid = (U.Corners - 1.0) * U.Cell * 0.5;
        xy = mid + (i * U.Cell - mid) * pow(U.Skirt.x, f32(k / rim + 1u) / U.Skirt.y);
    }
    let p = vec3<f32>(xy, corner(i));
    let off = p.xy - U.Eye.xy;
    var g: Ground;
    g.clip = U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0);
    g.world = p;
    return g;
}

// rimCorner is the j-th lattice corner round the world's edge, clockwise from the top-left
// (terrain.rim, the same).
fn rimCorner(j: u32) -> vec2<f32> {
    let a = u32(U.Corners.x) - 1u;
    let b = u32(U.Corners.y) - 1u;
    if j < a {
        return vec2<f32>(f32(j), 0.0);
    }
    if j < a + b {
        return vec2<f32>(f32(a), f32(j - a));
    }
    if j < 2u * a + b {
        return vec2<f32>(f32(a - (j - a - b)), f32(b));
    }
    return vec2<f32>(0.0, f32(b - (j - 2u * a - b)));
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
    let pixel = max(length(moveX.xy), length(moveY.xy)); // world units a pixel spans here, at most
    var toward = U.Toward; // the way to the eye: from p in a perspective
    if U.Perspective > 0.5 {
        toward = normalize(U.Eye - p);
    }
    // the world, or the skirt round it: level, in full sun, gridless
    let size = (U.Corners - 1.0) * U.Cell;
    let inside = p.x >= 0.0 && p.y >= 0.0 && p.x <= size.x && p.y <= size.y;
    var n = vec3<f32>(0.0, 0.0, 1.0);
    if inside {
        n = normal(p.xy);
    }
    let sun = sunWay();
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
        if U.Shadows > 0.5 && inside {
            lit = baked(p.xy, 0.0, U.ShadePx).r;
        }
    }
    var rgb = albedo(p.xy) * (U.Ambience + U.SunColor * max(dot(n, sun), 0.0) * lit * U.SunStrength);
    var cover = 0.0;
    if U.Cover > 0.0 {
        cover = clouds(p.xy, pixel);
    }
    let wet = lit * (1.0 - cover); // the sun on the water, the clouds' shadow taken off
    let detail = clamp((span - 6.0) / 6.0, 0.0, 1.0);
    var run = vec3<f32>(0.0);
    var still = vec3<f32>(0.0);
    var mouth = vec3<f32>(0.0);
    if U.WaterPx > 0.0 && wetAt(p.xy) {
        run = waterAt(p.xy, vec2<f32>(0.0, 0.0));
        still = waterAt(p.xy, vec2<f32>(1.0, 0.0));
        mouth = waterAt(p.xy, vec2<f32>(0.0, 1.0));
    }
    // the sea's glint, the waves turning to the shore and breaking on it near enough
    if still.b > 0.004 {
        var shore = vec4<f32>(0.0);
        if span >= 16.0 {
            shore = shoreAt(p.xy);
        }
        rgb = over(SeaGlintAt(p.xy, still.g / still.b, wet, shore, pixel, toward) * still.b, rgb);
    }
    if cover > 0.0 {
        rgb *= 1.0 - cloudShade(cover);
    }
    if U.GridFrom > 0.0 && span >= U.GridFrom && inside {
        rgb *= 1.0 - outlineDark * edge(p.xy, moveX, moveY);
    }
    // the water running down the ways and the cells it runs over, and where a way turns into water
    if run.b > 0.004 && span > 16.0 {
        let v = (run.rg / run.b * 2.0 - 1.0) * U.FlowSpan;
        rgb = over(RunningWater(p.xy, still.r / run.b * clamp((span - 16.0) / 8.0, 0.0, 1.0), wet, vec4<f32>(v, 0.0, 0.0)) * run.b, rgb);
    }
    if mouth.g > 0.004 && detail > 0.0 {
        rgb = over(SeaGlintAt(p.xy, mouth.r / mouth.g * detail, wet, vec4<f32>(0.0), pixel, toward) * mouth.g, rgb);
    }
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-distance(U.Eye, p) / U.Visibility));
    }
    return vec4<f32>(rgb, 1.0);
}

// baked is the fourth image's part from x across, px pixels a cell over the lattice's cells, at
// p: blended between its pixels, held within the part — the shade (shade.wgsl) the Renderer bakes.
fn baked(p: vec2<f32>, x: f32, px: f32) -> vec4<f32> {
    let size = (U.Corners - 1.0) * px;
    let t = clamp(p / U.Cell * px, vec2<f32>(0.5), size - 0.5);
    return imageSrc3Linear(imageSrc3Origin() + vec2<f32>(x, 0.0) + t);
}

// clouds is how thick the clouds stand over p, a pixel pixel world units wide there: their noise
// looked up in the tile of it baked into the fourth image from U.CloudFrom across (air.BakeTile),
// evened out as far as the pixel sees it.
fn clouds(p: vec2<f32>, pixel: f32) -> f32 {
    let torn = cloudsTile(p, cloudTileLevel(pixel, cloudTile * cloudSize), cloudTile * cloudSize).r;
    var core = 0.0;
    if U.Billow > 0.0 {
        core = cloudsTile(p, cloudTileLevel(pixel, heapTile * heapSize), heapTile * heapSize).g;
    }
    return cloudCoverSoft(cloudFromTile(torn, core), cloudTileSoft * cloudTileLevel(pixel, cloudTile * cloudSize));
}

// cloudsTile is the clouds' tile at p on level lod, its period period: the two levels round it
// blended.
fn cloudsTile(p: vec2<f32>, lod: f32, period: f32) -> vec2<f32> {
    let low = floor(lod);
    let dims = vec2<f32>(textureDimensions(image3));
    let from = imageSrc3Origin() + vec2<f32>(U.CloudFrom, 0.0);
    var n = textureSampleLevel(image3, linear, (from + cloudTileSpot(p, low, period)) / dims, 0.0).rg;
    if lod > low {
        n = mix(n, textureSampleLevel(image3, linear, (from + cloudTileSpot(p, low + 1.0, period)) / dims, 0.0).rg, lod - low);
    }
    return n;
}
