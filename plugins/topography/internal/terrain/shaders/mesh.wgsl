// The ground's mesh: every lattice corner a vertex, two triangles a cell, and round the world a
// skirt of U.Skirt.y rings reaching U.Skirt.x times as far from its middle as its edge, the edge's
// ground running on out to the horizon (terrain.Renderer builds them); drawn through U.ViewProj
// (camera.Transform) with the ground d off U.Eye sunk by U.Bend·d², so the skirt sinks under the
// horizon as the world's curve would have it.

struct Ground {
    @builtin(position) @invariant clip: vec4<f32>,
    @location(0) world: vec3<f32>,
    @location(1) cloud: f32, // how much of the sun the clouds take, blended between the corners
    @location(2) sway: vec2<f32>, // how much what stands here sways, how hard the gust blows
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
    if LEVEL <= 4u && U.Cover > 0.0 && U.CloudsShadeGround > 0.5 && vid < lattice {
        g.cloud = cloudsShadeAt(p, U.Cell);
    }
    // the gust over what stands and over the water alike
    if LEVEL <= 4u && (U.DetailPx > 0.0 || U.WaterPx > 0.0) && vid < lattice {
        g.sway = vec2<f32>(swayAmountAt(i), gustAt(p.xy, U.Cell));
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
fn fs_prime(g: Ground) -> @location(0) vec4<f32> {
    if U.Mouths > 0.5 && mouthed(g.world) {
        discard;
    }
    return vec4<f32>(0.0);
}

// how far past its cell's middle a mouth's cut runs and how far beside the floor's width, in
// cells, and how many times the floor's room it reaches up
const mouthPast: f32 = 0.1;
const mouthBeside: f32 = 0.02;
const mouthOver: f32 = 1.1;

// how far under a room's floor the ground in it is cut too, in cells: a byte's step of where the
// floor lies and the corners' 16 bits round it
const roomUnder: f32 = 0.01;

// mouthed says whether the ground at p is cut away: it lies over the mouth of a floor sunk under
// its cell (terrain.Mouth, the marks at the cell's corner), on the half of the cell the floor
// comes out through, within its width and lower than its roof; or in a room standing on its
// cell, over its floor and under its roof.
fn mouthed(p: vec3<f32>) -> bool {
    let i = clamp(floor(p.xy / U.Cell), vec2<f32>(0.0), U.Corners - 2.0);
    let mark = imageSrc0At(imageSrc0Origin() + i + 0.5);
    let code = u32(mark.b * 255.0 + 0.5);
    if code == 0u {
        return false;
    }
    if code == 5u {
        let floor = corner(i) + (mark.a - 0.5) * 2.0 * U.Cell;
        let room = imageSrc0At(imageSrc0Origin() + vec2<f32>(U.Corners.x, 0.0) + i + 0.5).a;
        return p.z > floor - roomUnder * U.Cell && p.z < floor + room * U.Cell * mouthOver;
    }
    let way = vec2<f32>(f32((code - 1u) % 3u) - 1.0, f32((code - 1u) / 3u) - 1.0);
    let mid = (i + 0.5) * U.Cell;
    let along = normalize(way);
    let q = p.xy - mid;
    // a little past the middle, wider and higher than the floor's room: no sliver of ground is
    // left in it
    if dot(q, along) < -mouthPast * U.Cell ||
        abs(dot(q, vec2<f32>(-along.y, along.x))) > (mark.a + mouthBeside) * U.Cell {
        return false;
    }
    let room = imageSrc0At(imageSrc0Origin() + vec2<f32>(U.Corners.x, 0.0) + i + 0.5).a;
    return p.z < height(mid + way * U.Cell * 0.5) + room * U.Cell * mouthOver;
}

// fs_main is the ground's colour at its point in the sun's light, turned to the Fog as far off
// as it lies and before the sight's end at every level, drawn at the level LEVEL (lod.Level),
// each dropping more, the close levels drawn as Standard:
//   2  everything
//   3  the grid
//   4  the baked shade and the lamps, every cell taking every shade; the albedo at half its size
//   5  the clouds' shadow and the wind's sway (vs_main), the details' grain and the water, its
//      waves, its surf and its running: a sheet of the albedo's flat colour, a quarter its size
// The water is alive at every level short of 5, what of it is finer than a pixel or two left
// out (water's sea and stream).
@fragment
fn fs_main(g: Ground) -> @location(0) vec4<f32> {
    let p = g.world;
    if U.Mouths > 0.5 && mouthed(p) {
        discard;
    }
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
    if LEVEL <= 4u && inside {
        flags = flagsAt(p.xy);
    }
    var taken = f32(takesRelief | takesBlocks | takesClouds | takesLamps);
    if LEVEL <= 3u && inside {
        taken = takesOf(flags);
    }
    var lit = 0.0;
    if sun.z > 0.0 {
        lit = 1.0;
        if LEVEL <= 3u && inside && (takes(taken, takesRelief) || takes(taken, takesBlocks)) {
            lit = groundShadeOf(p.xy).r;
        }
    }
    let light = lightOn(n, sun, p, lit, taken);
    var ground = levelAlbedo(p.xy);
    if LEVEL <= 4u && U.DetailPx > 0.0 && inside {
        let show = detailShow(span);
        if show > 0.0 {
            ground *= mix(vec3<f32>(1.0), grain(p.xy, g.sway, cellDetailAt(flags), span), show);
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
    if LEVEL <= 4u && U.WaterPx > 0.0 && wetAt(p.xy) {
        still = waterAt(p.xy, vec2<f32>(1.0, 0.0));
        // far off the flow is read where the water that shines runs, and no mouth: read for
        // every pixel of the sea they cost the strategic level a twelfth of its frame
        if LEVEL <= 2u || still.r > 0.004 {
            run = waterAt(p.xy, vec2<f32>(0.0, 0.0));
        }
        if LEVEL <= 3u {
            mouth = waterAt(p.xy, vec2<f32>(0.0, 1.0));
        }
    }
    // the sea lies at its level over ground lower than it, its surface its own (sea.wgsl): the
    // ground here is its bed, darker under the water, glinting nowhere
    let sunk = still.b > 0.004 && p.z < -sunkBy * U.Span;
    if sunk {
        rgb *= mix(1.0, bedWet, smoothstep(0.0, -bedWetting * U.Cell, p.z));
    }
    // the sea's glint, the waves turning to the shore and breaking on it. The shore is read
    // under a condition of its own: read for every pixel of water, it cost a fifth of the frame
    // on every pixel of the ground (an Intel GPU, Benchmark_Frame_Topography)
    if still.b > 0.004 && !sunk {
        var shore = vec4<f32>(0.0);
        if span >= shoreFrom {
            shore = shoreAt(p.xy);
        }
        let sea = SeaGlintAt(p.xy, still.g / still.b, wet, shore, pixel, toward, g.sway.y);
        rgb = over(sea * still.b, rgb);
    }
    if cover > 0.0 {
        rgb *= 1.0 - cloudShade(cover);
    }
    if LEVEL <= 2u && U.GridFrom > 0.0 && span >= U.GridFrom && inside {
        let e = edge(p.xy, moveX, moveY);
        rgb = rgb * (1.0 - U.GridLine.a * e) + U.GridLine.rgb * e;
    }
    // the water running down the ways and the cells it runs over, and where a way turns into water
    if run.b > 0.004 {
        let v = vec4<f32>((run.rg / run.b * 2.0 - 1.0) * U.FlowSpan, 0.0, 0.0);
        rgb = over(RunningWaterAt(p.xy, still.r / run.b, wet, v, pixel, toward) * run.b, rgb);
    }
    if LEVEL <= 3u && mouth.g > 0.004 && detail > 0.0 && !sunk {
        rgb = over(SeaGlintAt(p.xy, mouth.r / mouth.g * detail, wet, vec4<f32>(0.0), pixel, toward,
            g.sway.y) * mouth.g, rgb);
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

// grain is what the details make of the ground's colour at p: their tone doubled, so a tile of
// mean tone leaves the colour as it is; where what stands sways (sway: how much, how hard the
// gust blows over it), the grain leans with the wind (swayShift of a cell at a full lean) and
// lightens under the gust, in bands as the light is.
fn grain(p: vec2<f32>, sway: vec2<f32>, own: f32, span: f32) -> vec3<f32> {
    var q = p;
    var sheen = 0.0;
    if sway.x > 0.0 {
        q += windLeanUnder(sway.x, sway.y) * U.Cell * swayShift;
        sheen = sway.y * windStrength() * sway.x * swaySheen;
        if U.Bands > 0.5 {
            sheen = step(swayBand, sway.y * windStrength()) * sway.x * swaySheen;
        }
    }
    return detailAt(q, own, span) * 2.0 * (1.0 + sheen);
}

// how far a cell's grain shifts at a full lean, of the cell; how much lighter what sways wholly
// is under a gust's middle in a full wind; the gust a band of the light begins at
const swayShift = 0.3;
const swaySheen = 0.3;
const swayBand = 0.15;

// how dark the sea's bed is under the water, and over how much of a cell's depth it darkens
const bedWet = 0.45;
const bedWetting = 0.02;
