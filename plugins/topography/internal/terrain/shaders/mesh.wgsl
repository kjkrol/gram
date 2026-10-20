// The ground's mesh: every lattice corner a vertex, two triangles a cell, and round the world a
// skirt of U.Skirt.y rings reaching U.Skirt.x times as far from its middle as its edge, the edge's
// ground running on out to the horizon (terrain.Renderer builds them); drawn through U.ViewProj
// (camera.Transform) with the ground d off U.Eye sunk by U.Bend·d², so the skirt sinks under the
// horizon as the world's curve would have it.

struct Ground {
    @builtin(position) @invariant clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) cloud: f32, // how much of the sun the clouds take, blended between the corners
    @location(2) sway: vec2<f32>, // how much what stands here sways, the wind's wave over it
}

// vs_main stands the lattice corner vid, counted row by row, at its height, under as much of the
// clouds' shadow as falls on it; past the lattice's corners, the skirt's, unshaded: ring after
// ring, each round the edge clockwise from the top-left, pushed out from the middle, at the height
// of the edge's corner it runs out from.
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
    var g = Ground(U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0), p, 0.0,
        vec2<f32>(0.0));
    // at the corners alone: a cloud spans many cells, and every pixel cost a frame dearly; none on
    // the skirt, whose cells grow so big out to the horizon that the shadows showed as their
    // triangles
    if CARE >= 1u && U.Cover > 0.0 && U.CloudsShadeGround > 0.5 && vid < lattice {
        g.cloud = cloudsShadeAt(p, U.Cell);
    }
    if CARE >= 1u && U.DetailPx > 0.0 && vid < lattice {
        g.sway = vec2<f32>(swayAmountAt(i), swayWave(p.xy));
    }
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
// as it lies and before the sight's end whatever the care, shaded with the care CARE
// (terrain.Renderer.SetCare), each dropping one more thing:
//   4  everything
//   3  the shore: the waves come in with no swell, surf or foam, no water runs down the ways
//      and no mouth glints
//   2  the waves and their glint: the still water a level mirror of the sky
//   1  the baked shade and the lamps: every cell takes every shade
//   0  the grid, the clouds' shadow and the wind's sway (vs_main), the details' grain and the
//      water's picture: the albedo's flat colour
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
    let sun = lightWay();
    var flags = vec4<f32>(1.0, 1.0, 0.0, 1.0);
    if CARE >= 1u && inside {
        flags = flagsAt(p.xy);
    }
    var taken = f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    if CARE >= 2u && inside {
        taken = takesOf(flags);
    }
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
        if CARE >= 2u && inside && (takes(taken, takesRelief) || takes(taken, takesBlocks)) {
            lit = groundShadeOf(p.xy).r;
        }
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
    if CARE >= 2u && takes(taken, takesLamps) {
        light += lampLightAt(p);
    }
    var ground = albedo(p.xy);
    if CARE >= 1u && U.DetailPx > 0.0 && inside {
        let show = detailShow(span);
        if show > 0.0 {
            ground *= mix(vec3<f32>(1.0), grain(p.xy, g.sway, cellDetailAt(flags)), show);
        }
    }
    var rgb = ground * light;
    var cover = g.cloud;
    if !takes(taken, takesClouds) {
        cover = 0.0;
    }
    let wet = lit * (1.0 - cover); // the sun on the water, the clouds' shadow taken off
    let detail = clamp((span - 6.0) / 6.0, 0.0, 1.0);
    var run = vec3<f32>(0.0);
    var still = vec3<f32>(0.0);
    var mouth = vec3<f32>(0.0);
    if CARE >= 1u && U.WaterPx > 0.0 && wetAt(p.xy) {
        still = waterAt(p.xy, vec2<f32>(1.0, 0.0));
        if CARE >= 4u {
            run = waterAt(p.xy, vec2<f32>(0.0, 0.0));
            mouth = waterAt(p.xy, vec2<f32>(0.0, 1.0));
        }
    }
    // the sea's glint, the waves turning to the shore and breaking on it near enough; with less
    // care the open sea everywhere, with less still a level mirror
    if still.b > 0.004 {
        if CARE >= 3u {
            var shore = vec4<f32>(0.0);
            if CARE >= 4u && span >= 16.0 {
                shore = shoreAt(p.xy);
            }
            let sea = SeaGlintAt(p.xy, still.g / still.b, wet, shore, pixel, toward);
            rgb = over(sea * still.b, rgb);
        } else {
            rgb = over(LevelWater(still.g / still.b, toward) * still.b, rgb);
        }
    }
    if cover > 0.0 {
        rgb *= 1.0 - cloudShade(cover);
    }
    if CARE >= 1u && U.GridFrom > 0.0 && span >= U.GridFrom && inside {
        let e = edge(p.xy, moveX, moveY);
        rgb = rgb * (1.0 - U.GridLine.a * e) + U.GridLine.rgb * e;
    }
    // the water running down the ways and the cells it runs over, and where a way turns into water
    if CARE >= 4u && run.b > 0.004 && span > 16.0 {
        let v = (run.rg / run.b * 2.0 - 1.0) * U.FlowSpan;
        rgb = over(RunningWater(p.xy, still.r / run.b * clamp((span - 16.0) / 8.0, 0.0, 1.0), wet, vec4<f32>(v, 0.0, 0.0)) * run.b, rgb);
    }
    if CARE >= 4u && mouth.g > 0.004 && detail > 0.0 {
        rgb = over(SeaGlintAt(p.xy, mouth.r / mouth.g * detail, wet, vec4<f32>(0.0), pixel, toward) * mouth.g, rgb);
    }
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(p) / U.Visibility));
    }
    // the fog before the camera's sight ends (U.Sight: from where, where it begins and ends)
    if U.Sight.w > 0.0 {
        rgb = mix(rgb, U.Fog, smoothstep(U.Sight.z, U.Sight.w, distance(p.xy, U.Sight.xy)));
    }
    return vec4<f32>(rgb, 1.0);
}

// banded is the light d quantised into bands steps, each edge softened a little.
fn banded(d: f32, bands: f32) -> f32 {
    let b = d * bands;
    let k = floor(b);
    return (k + smoothstep(0.42, 0.58, b - k)) / bands;
}

// grain is what the details make of the ground's colour at p: their tone doubled, so a tile of
// mean tone leaves the colour as it is; where what stands sways (sway: how much, the wave over
// it), the grain leans with the wind (swayShift of a cell at a full lean) and the wave lightens
// its crests and darkens its troughs, in bands as the light is.
fn grain(p: vec2<f32>, sway: vec2<f32>, own: f32) -> vec3<f32> {
    var q = p;
    var sheen = 0.0;
    if sway.x > 0.0 {
        q += swayLean(sway.x, sway.y) * U.Cell * swayShift;
        sheen = sway.y * swayStrength() * sway.x * swaySheen;
        if U.Bands > 0.5 {
            sheen = (step(0.3, sway.y) - step(0.3, -sway.y)) * sway.x * swaySheen;
        }
    }
    return detailAt(q, own) * 2.0 * (1.0 + sheen);
}

const swayShift = 0.3;
const swaySheen = 0.12;

// hazeDepth is how much air lies between the eye and the ground point p, world units: its
// distance from U.Eye, or seen from above (U.HazeUp) the air over p up to that height, along the
// lines of sight U.HazeWay.
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}
