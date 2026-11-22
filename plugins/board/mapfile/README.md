# A level in files

A level is a folder: a manifest, the ground's heights as an image and what lies on the ground
as GeoJSON. It is the map a person or a tool writes; what a game derives from it (a ground's
points, a board's cells) is worked out as it loads and never stored in it.

```
level/
  level.yaml          the manifest
  heights.png         the ground's heights
  features.geojson    what lies on the ground
```

`mapfile.Load(fsys, "level/level.yaml")` reads one; the files it names lie beside the manifest.
The package lays nothing: a board takes its geography from the `File`, a square or hex one by
its level's `geography:` (`board.Lay`, the board's `doc.go`), the mosaic by `mosaic.Load`. A level that works is `testdata/level` here and `examples/look/ground/level`.

## Units and axes

A metre is a unit, everywhere. A place in the features is metres **east and north of the
level's south-west corner**, as a map is read. The world's y runs down, so a reader turns a
feature's `y` into `size.y - y`. The image's first row is the level's north edge: it needs no
turning.

## The manifest

```yaml
size: [1536, 1536]      # metres east to west and north to south
seed: 7                 # what the ground's points are scattered from
heights:
  file: heights.png
  pixel: 2              # metres between two pixels
  lowest: -60           # the height of the value 0
  step: 0.002           # metres a step of the value
features: features.geojson
corner: 8               # the radius the kerbs round a junction's corners at; 8 unset
roads:                  # how a road of each highway lies across
  residential:
    lanes: 2
    lane: 3.5           # a lane's width
    walk: 2.5           # a pavement's, either side
    kerb: 0.15          # how high the pavement stands
    verge: 1.5          # the verge's past it
```

`size` is required. `heights` and `features` are each optional: a level without the first is
flat at zero, without the second bare.

Any other key at the top is a **part** of a reader's own, read by that reader alone and
skipped by the rest. The mosaic's:

```yaml
mosaic:
  apart: 2              # how far apart the ground's points lie at their finest; 2 unset
  rungs: 6              # over how many rungs they thin out where the heights let them; 6 unset
```

## Heights

A PNG of 16-bit grey. A pixel is a **corner**, not a square: the pixel `(i, j)` is the height
at `i × pixel` metres east and `j × pixel` metres south of the north-west corner, so an image
covering `size` has `size / pixel + 1` pixels a side. Its height is `lowest + value × step`,
exactly that on a pixel; between the pixels the ground is even (bilinear).

Refused: an image of another depth or colour, `pixel` or `step` unset, an image not reaching
`size`.

## Features

A GeoJSON `FeatureCollection`. A feature's properties are named as OpenStreetMap's tags are,
so a map drawn there, in QGIS or in JOSM is a level's features with its places turned to
metres. Every feature is read as it is, a `Point`, a `LineString` or a `Polygon`
(its first ring the outline, the rest holes) with its `id` and its properties as tags; another
geometry is refused. What a feature becomes is its reader's:

| Feature | Geometry | Reader | What it becomes |
|---|---|---|---|
| `highway=<kind>` | `LineString` | the mosaic | a road's middle line, laid across as `roads.<kind>` says |
| `highway=<kind>` | `LineString` | a square or hex board | a road of the kind the level's table names, over the cells the line's points lie in |
| `waterway=<kind>` | `LineString` | a square or hex board | a river, its last point where the water goes; `width`, metres, is how wide it runs: one number for the line, or one a point between semicolons (`"4;4.5;6"`), even from point to point; without it as wide as the level's table says |
| `highway` with `bridge`, `tunnel` or `ford` | `LineString` | a square or hex board | a bridge, a tunnel or fords of the kind the level's table names, from the cell before what it crosses to the cell after; the roads lead onto it |
| `ford=yes` | `Point` | a square or hex board | a ford of the river it lies on |
| any tag the level's table names | `Polygon` | a square or hex board | the kind of the cells whose middles it holds |

The mosaic refuses a `highway` the manifest names no road for, never skips it: a road missing
from the picture is harder to find than a message. Every road it lays is of one section yet.

### Where roads meet

Junctions are never written. Lines that share a point (to a centimetre) meet there:

- a line passing through a point another line has is **cut** there;
- where exactly two ends of one highway meet, they are **joined** into one road, so a road may
  be drawn in as many lines as is handy;
- where three or more meet, there is a **meeting**: a junction, its corners rounded at
  `corner`.

A point twice in a row is one point. The mosaic lays a junction only where its roads leave no
more than a straight angle open between two neighbours: a kerb cannot be led round a corner
wider than that yet, and the level is refused with the junction's place.

## A level in tiles

A level of an island's size is cut into tiles by `mapfile.Cut(fsys, "level/level.yaml", out,
mapfile.Cutting{...})` (`mosaic.Cut` with the mosaic's defaults). The folder above stays what a
person edits; the tiles are cut anew from it and never edited by hand. `Load` reads either.

```
out/
  level.yaml                      the manifest, the files it named taken out, its tiles said
  0/<column>_<row>/heights.png    the level as it is
  0/<column>_<row>/features.geojson
  3/<column>_<row>/...            the overview
```

```yaml
tiles: {side: 1024, margin: 32, tiers: [0, 3]}
```

- A tile's side is a power of two metres and a whole number of the heights' pixels; a game
  picks a multiple of its world's sector side. Tiles count from the north-west corner, as an
  image's rows do; one at the level's edge is as small as the level leaves it.
- Two tiers. `0` is the level as it is. `3`, the overview, has tiles eight times the side,
  heights every eighth pixel (taken, not averaged) and only what the cutter is told is drawn
  from far off (`Cutting.Overview`; the mosaic's, unset: the roads and whatever is 64 m or
  more across). A
  line is never simplified, in either.
- A tile with no height over zero and nothing on it is not written, and a tile absent is the
  sea: level zero, nothing on it.
- A tile's heights reach one pixel into the next tile, the edge both share, and every tile is
  of the manifest's one `lowest` and `step`: no seam shows.
- A point, a line and an area no wider than a tile lie whole in every tile whose box grown by
  the margin they touch, and are read once, by their `id`. Give the features ids: one without
  is named by its place in the file, which the next cut may change.
- An area wider than a tile is cut to each tile and its margin, the piece tagged
  `clipped=true` and keeping the area's id.
- Places stay the level's, metres east and north of its south-west corner, in every tile.

Not there yet (`doc/roadmap.md`): the mosaic builds its whole ground as the game starts, so a
level in tiles is read whole; the ground built tile by tile as the camera moves, and what is
derived from a tile kept beside it, come after.
