---
name: goke
description: goke, the archetype ECS gram and ceremony run on (github.com/kjkrol/goke/v3, locally /home/karol/CODE/goke through go.work). Use when reading or writing systems, queries, command buffers or components in gram or ceremony, when a query reads a wrong entity, when touching goke itself, or when releasing it.
---

# goke

Local checkout `/home/karol/CODE/goke`, joined to gram and ceremony by `/home/karol/CODE/go.work`,
so gram builds against the working tree, not the version in `go.mod`. A fix in goke reaches a
published gram only through a tag and a `go.mod` bump. `scripts/api.sh [Type]` prints the API
of the checkout; read it instead of grepping.

## The model

- An entity is a `uid.UID64` in one archetype (one set of component types); its components lie
  in chunks, a column a type. Adding or removing a component **moves the entity** to another
  archetype (~200 ns); a value write is 1–2 ns. gram keeps states that flip often as tag bits
  for this reason.
- `ECS.RegSys(system)` registers a `System` (`Init(*SysInit)`, `Update(*CmdBuf, d)`; `SystemFn`
  for a literal) and hands back a `Runnable`; `ECS.Setup(systems...)` runs every `Init` once;
  `ECS.SetPlan(func(RunCtx, d))` says the order each tick: `ctx.Run(runnable, d)`, `ctx.Sync()`
  flushes the command buffers (adds, removes, migrations land here, never inside Update).
  `ECS.Tick(d)` runs the plan. A `Module` (`RegSystems`, `RunPlan`, `SetupSystems`) bundles
  systems; gram's plugins are modules.
- 512 component types at most; one `RegComp[T]` id a type per ECS.

## Queries

- Built in `Init`: `si.NewQueryBuilder(&comp, …).Optional(&opt, …).Include/Exclude(...).Build()`
  with `goke.Comp[T]` and `goke.OptComp[T]` columns the builder binds. **A type may be tracked
  once in one query**: two columns of one type need two queries.
- `q.All(); q.Next()` walks chunks: `comp.Slice(cur)` the column, `cur.IDs` the entities,
  `opt.Slice(cur)` nil where the chunk lacks it, `opt.Present(cur)`.
- `q.Seek(id)` positions `Cursor` on one entity, mask ignored (the caller knows it carries the
  tracked types); `comp.At(cur)`. `q.SeekH(id)` is the cheap one for a batch sharing the
  archetype of the last `Seek`; false when it differs. `q.Pick(ids); q.Next()` walks given
  entities, `comp.At(cur)`.
- **One query for `Seek` and `All` is fine since v3.4.1** — before, a `Seek` after an `All` or
  `Pick` kept the last chunk's offsets and `At` read another archetype's column (gram's
  `effect_commands` read a wrong `Label`, so a plate's `Trigger` matched its command by chance).
  Keep gram's pattern anyway: a `lookup` query for `Seek`, a `walk` query for `All`.
- A query sees components as they are; what a `CmdBuf` added lands at `Sync`.

## Command buffers and columns

- `cb.AddOne(id, compID, value)`, `cb.RemoveCompOne(id, compID)`, `cb.RemoveOne(id)`: staged,
  applied at `Sync`. `cb.Reserve(id, compID)` is the first half of `AddOne`: a pointer to the
  staged value, written in place (ceremony's `store.Facts.Stage`, no copy). `ECS.CmdBuf(runnable)`
  is a system's own buffer outside Update.
- `NewColumn(t)` / `NewOptColumn(t)` (`Base`, `At`, `Present`, `Type`, `Size`) and `RegCompOf(t)`,
  `LoadCompOf(t)` are the reflect-typed twins of `Comp[T]`, `RegComp[T]`, `LoadComp[T]`: the
  bridge for a library that learns its component types at run time (ceremony's `cgoke`).
- Persistence: `ECS.Save(path)`, `ECS.Load(path, comps...)` with the `CompToken`s of every
  component type to load (`LoadComp[T]`, `LoadComps`); a type's path is its name in a save, so
  moving a type between packages breaks old saves.

## Conventions of the repo

- Go 1.27: generic methods are allowed and used (`func (c *Columns) Own[T any](...)`).
- **100% statement coverage** in every package but `internal/chunk` (99.5% before any of my
  work) and the examples; CI runs `go test -race -coverprofile`. A change without a test does
  not go in. Check with `go test -cover ./...`.
- Internal tests may build `AddOne` with nil defs: `stage()` fills a def from `T`.
- Release: CHANGELOG entry **before** the tag (the tag then carries it); tags `vX.Y.Z`; release
  notes cover everything since the last published release, not since the last tag; the user
  tags and publishes, never Claude. After a tag, bump ceremony's and gram's `go.mod`.
- No `Co-Authored-By`; conventional prefixes; comments as gram's: what a thing is, in one or two
  sentences.

## Pitfalls met

- `go doc -short Type | grep -v '^type'` hides the methods: use `scripts/api.sh Type`.
- In zsh `echo ==` fails (`=cmd` expansion): write `echo '--'`.
- A chunk's `Cursor.Offsets` belong to that chunk; never keep a `Cursor` across `Next`.
