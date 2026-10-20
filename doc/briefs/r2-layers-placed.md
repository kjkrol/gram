# R2. Dziura pod warstwami wewnętrznymi; Directy rysowane raz i umieszczane

**Repo, gałąź:** gram, `r/placed` od `main` (worktree `/home/karol/worktrees/gram/placed`).
**Etap:** R, dźwignie po etapie U (`doc/plans-perf.md` „Etap U: wynik”, „Uwagi”; B6 z „Etap R2:
wynik”).
**Skille:** `gram-render` (Laid, Place, PlaceDepth, feed), `gram-probe`.

## Cel

1. **Dziura (B6).** W warstwach zewnętrzna warstwa rysuje także to, co zasłoni warstwa
   wewnętrzna (preset = 0,71 piksela na piksel ekranu). Po etapie U składanie jest jedno, a
   głębie per warstwa, więc warstwa zewnętrzna może pominąć okno wewnętrznej (poza piórem):
   nożyce/scissor na `DrawLayer` albo test w shaderze. Zysk: mniej pikseli gruntu w warstwach.
2. **Directy umieszczane.** Dziś Directy (terrain, aloft, hexes…) rysują per warstwa przez
   przyciętą kamerę; mają rysować raz w pikselach kadru i być umieszczane jak kawałki, co wymaga
   `Place` w `DrawMesh` (`render.DrawMeshOptions`, `gpu`): zysk z „drawing” 1,7 ms w demie
   warstw (Directy nadal per warstwa po U).

## Pliki

Wolno: `render/composer.go` (`DrawLayer`, `laid`, nie `renderOver` poza tym, co umieszczanie
wymusza), `render/feed.go` (`drawLayers`, `fadeOf`), `render/mesh.go`, `render/gpu/mesh.go`,
`render/gpu/shaders/*.wgsl`, `render/gpu/program.go` (`Place`), testy render i gpu,
`bench/frame_test.go`. Szew: `renderOver` i aloft są R1. Sprawdzić przed startem, czy każdy
Direct rysuje przez `camera.SceneTransform` z `RayField` (wtedy umieszczanie jest czyste) i
nazwać w raporcie te, które nie.

## Kroki

1. Pomiar przed: `Benchmark_Frame_Topography` `iso,layered`, `persp,footed`, `…,sharp`;
   demo 2560×1440 bez vsync w kopii z warstwami presetu (`drawing`, FPS).
2. Dziura: warstwa i > 0 wycina okno warstwy i+1 pomniejszone o pióro (`LayerFeather`) w
   `DrawLayer` (nożyce w `gpu.Draw` albo `Scissor` w opcjach); test w `composer_internal_test`
   albo `feed_test`, że piksele pod oknem wewnętrznym nie są rysowane; zrzuty bez szwów.
3. Directy umieszczane: `Place` w `DrawMeshOptions` (jak `dw.Place` przy kawałkach), Direct
   rysowany raz w pikselach kadru do obrazu kadru i umieszczany per warstwa; głębie spójne z
   `PlaceDepth`. Jeśli któryś Direct musi rysować per warstwa (np. czyta rozmiar celu), zostaje
   per warstwa i jest nazwany.
4. Zrzuty `GRAM_SHOT` klatka 600 z `PROBE_SEED=7`: izo w warstwach, Tab (footed), V (jazda),
   ostre i miękkie figury, przed i po; różnica pikseli; obrys warstw „Show the layers” bez
   przesunięć.
5. Pomiar po, naprzemiennie; `Benchmark_Frame_Topography` nowych liczb do raportu.

## Poza zakresem

Aloft w przebiegu górnym (R1). Profile i układy warstw (etap U).

## Raport

Liczby przed/po, lista Directów umieszczanych i nieumieszczanych, różnice zrzutów, propozycje
wpisów do `CHANGELOG.md` i skilla `gram-render`.
