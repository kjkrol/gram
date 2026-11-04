// A sector of a hex board seen as a sheet (topography's hexes at lod.L3): an instance a sector,
// its box (x0, y0, x1, y1) and its height (the mean of its cells' corners), a quad over the box
// at that height, drawn through U.ViewProj (camera.Transform) sunk U.Bend·d² under U.Eye's level;
// coloured from the board from above, the first image, U.TopPx pixels a world unit, the figures'
// colour summed over it (world.AggregateImage, U.AggregatePx texels a world unit), in the ambient
// light and the sun's on level ground, nothing off the board; hazed and fogged as the cells are.

struct Sheet {
    @builtin(position) clip: vec4<f32>,
    @location(0) world: vec3<f32>,
}

@vertex
fn vs_main(
    @builtin(vertex_index) vid: u32,
    @location(0) box: vec4<f32>,
    @location(1) at: vec4<f32>,
) -> Sheet {
    // two triangles: top-left, top-right, bottom-right, then top-left, bottom-right, bottom-left
    var across = array<f32, 6>(0.0, 1.0, 1.0, 0.0, 1.0, 0.0);
    var down = array<f32, 6>(0.0, 0.0, 1.0, 0.0, 1.0, 1.0);
    let k = vid % 6u;
    let p = vec3<f32>(mix(box.x, box.z, across[k]), mix(box.y, box.w, down[k]), at.x);
    let off = p.xy - U.Eye.xy;
    return Sheet(U.ViewProj * vec4<f32>(p.xy, p.z - U.Bend * dot(off, off), 1.0), p);
}

@fragment
fn fs_main(s: Sheet) -> @location(0) vec4<f32> {
    let albedo = imageSrc0Linear(imageSrc0Origin() + s.world.xy * U.TopPx);
    if albedo.a < 0.5 {
        discard;
    }
    let summed = aggregateAt(s.world.xy);
    let light = U.Ambience + U.LightColor * max(lightWay().z, 0.0) * U.LightStrength;
    var rgb = (summed.rgb + albedo.rgb * (1.0 - summed.a)) * light;
    if U.Visibility > 0.0 {
        rgb = mix(rgb, U.Fog, 1.0 - exp(-hazeDepth(s.world) / U.Visibility));
    }
    if U.Sight.w > 0.0 {
        rgb = mix(rgb, U.Fog, smoothstep(U.Sight.z, U.Sight.w, distance(s.world.xy, U.Sight.xy)));
    }
    return vec4<f32>(rgb, 1.0);
}

// hazeDepth is how much air lies between the eye and the world point p, world units, as the
// cells take it (hexes.wgsl).
fn hazeDepth(p: vec3<f32>) -> f32 {
    if U.HazeUp <= 0.0 {
        return distance(U.Eye, p);
    }
    return max(U.HazeUp - p.z, 0.0) / max(-U.HazeWay.z, 0.05);
}

// aggregateAt is the figures' colour summed into the ground at p, premultiplied, blended between
// the four texels round p; clear where U.AggregatePx is 0.
fn aggregateAt(p: vec2<f32>) -> vec4<f32> {
    if U.AggregatePx <= 0.0 {
        return vec4<f32>(0.0);
    }
    let size = vec2<f32>(textureDimensions(Aggregate));
    let t = clamp(p * U.AggregatePx, vec2<f32>(0.5), size - 0.5);
    return textureSampleLevel(Aggregate, linear, t / size, 0.0);
}
