# R1. Chmury aloft w przebiegu górnym w rozmiarze gruntu

**Repo, gałąź:** gram, `r/aloft` od `main` (worktree `/home/karol/worktrees/gram/aloft`).
**Etap:** R, dźwignie po etapie U (`doc/plans-perf.md` „Etap U: wynik”, „Uwagi”; „Etap T:
wynik”, „Uwagi”).
**Skille:** `gram-render`, `gram-plugins` (atmosphere, `atmosphere/aloft`), `gram-probe`.

## Cel

Z ostrymi figurami przebieg górny (`render.Laid.DrawOver`) idzie w każdym pikselu ekranu, a
aloft rysuje w nim chmury: stały koszt ~1,5 ms na klatkę (etap T), 2,7–3,1 ms na wyspie w
warstwach (etap U). Chmury są miękkie (dziś i tak liczone na 1/4–1/8 pikseli i kładzione
powiększone), więc w przebiegu górnym mogą być liczone w rozmiarze gruntu (rozdzielczość
kamery, nie ekranu) i położone powiększone, jak kawałki. Oczekiwanie: koszt przebiegu górnego
spada o większość tego, co dodaje aloft; obraz bez szwów.

## Pliki

Wolno: `plugins/atmosphere/aloft/aloft.go` (+ shadery `shaders/*.wgsl`, testy), w `render`
tylko to, czego aloft potrzebuje, by znać rozmiar gruntu i rozdzielczość w przebiegu górnym
(`render/target.go` lub `Uniforms`: np. `Target.Coarse`/rozmiar gruntu; mały dodatek, nazwany
w raporcie), `bench/frame_test.go` (przypadki istniejące). Szew: `render/composer.go` poza
`renderOver` i `render/feed.go` są R2; nie ruszać `DrawLayer`, `Place`, `drawLayers`.

## Kroki

1. Pomiar przed: `Benchmark_Frame_Topography` przypadki `iso,0.7,sharp`, `iso,layered,sharp`,
   `persp,footed,sharp` wobec miękkich; demo board/topography 2560×1440 bez vsync w kopii
   z ostrymi figurami (`GRAM_FPS_LOG=1`), `drawing` z logu.
2. Aloft w `Draw` przebiegu górnego: rozmiar obrazu `coarse` od rozmiaru gruntu
   (viewport kamery w jej rozdzielczości), nie od `t.Screen`; `lay.wgsl` kładzie powiększone na
   ekran z głębią ekranu; sprawdzić, że `LayCoarse` i `sizeFor` (klucz: rozmiar + coarse) dają
   jeden obraz na klatkę, nie dwa.
3. Zrzuty `GRAM_SHOT` klatka 600 z `PROBE_SEED=7` i pochmurną pogodą (`PROBE_SED` na
   `Start: weather.Cloudy`), przed i po, w izometrii, perspektywie (Tab) i jeździe (V) przez
   sondę klawiszy; różnica pikseli policzona; bez szwów przy krawędziach warstw.
4. Pomiar po, naprzemiennie.

## Poza zakresem

Dziura pod warstwami wewnętrznymi i Directy umieszczane (R2). Wygląd chmur.

## Raport

Liczby przed/po (benchmark, `drawing`, FPS), różnice zrzutów, dodatek w `render` (sygnatura),
propozycje wpisów do `CHANGELOG.md`, skilli `gram-render` i `gram-plugins`.
