---
name: gram-perf-picture
description: Wydajność klatki i obrazu grama — rozdzielczość obrazu i staranność gruntu po pierścieniach, pierścienie jako warstwy kamery, grunt w rozdzielczości kamery z figurami ostrymi, jedna kompozycja na warstwę, dźwignie za budżetem klatki, płaska plansza na GPU; niezmienniki i liczby; benchmarki World_Draw, World_Frame, Frame_Topography, Frame_Flat, Frame_UI, Text_Panel, Painter_Bake; readback klatki headless. Czytaj przy zmianie render, Composer, Direct, warstw, painterów, shaderów, gdy klatka zwolniła, i przed propozycją zmiany ścieżki rysowania.
---

# gram-perf-picture

Protokół pomiaru i pułapki są w `gram-perf`; tu mapa tego, co w klatce zrobiono. Liczby:
`BENCHMARKS.md` („The island's frame", „The demos"), historie: `doc/plans-perf.md` (etapy R, R2,
T, U, „Płaska plansza na GPU"); jak się rysuje: `gram-render`.

## Gdzie idzie czas

Klatka (GPU headless): `World_Draw`, `World_Frame`, `Frame_Topography` (wyspa w 1440p: pole
każdej zmiany `render`), `Frame_Flat`, `Frame_UI`, `Text_Panel`; cały wypiek paintera
`Painter_Bake` w jego pakiecie. Klatka headless niesie ~10 ms readbacku w 1440p: klatki
porównuje się z klatkami, nigdy z liczbą klatek na sekundę. Klatki dem i ich garbage collector:
`gram-probe`, „Measuring a demo's frames". Profile CPU głównych benchmarków: `BENCHMARKS.md`,
„Where the time goes".

## Co zrobiono i na czym stoi

- **R, R2, T, U** (2026-10-19/20): rozdzielczość obrazu i staranność gruntu po pierścieniach,
  potem pierścienie jako warstwy (`camera.Layer`, `Inset`/`Footed`), grunt w rozdzielczości
  kamery z figurami ostrymi nad nim, jedna kompozycja na każdą warstwę (`Lay`/`DrawLayer`/
  `DrawOver`) i dziura pod wewnętrznymi warstwami (preset rysuje 0,52 pikseli klatki). Wyspa w
  1440p, izo: 7,3 → 5,9 ms w pierścieniach, 8,0 → 7,0 ms z jedną kompozycją; demo w 2560×1440:
  45 FPS przy każdym pikselu, 70–106 przy 0,72/0,52 z nich. Niezmiennik: Direct rysuje raz na
  warstwę i trzyma to, co zebrał Compose (`gram-render`; sprite'y, trasy i widoki złamały go po
  razie). Rozdzielczość idzie za budżetem klatki (adapter kamer): pomiar ją ustawia
  (`cameras.Picture{Resolution}`) albo wyłącza warstwy.
- **Płaska plansza na GPU** (2026-10-23): mapa kafli w shaderze renderera planszy (blok 2×2
  piksele na komórkę), wypiek paintera na GPU (warstwa 19 → 4 ms), martwa ścieżka równoległa
  CPU usunięta. Niezmiennik: CPU pisze dane komórek, shadery liczą kształty.
- **Publikacja i rysowanie z opublikowanego** (2026-10-24): plansza, relief, pogoda, światło i
  znaki minimapy publikowane raz na klatkę, renderery czytają kopię, nigdy żywy stan
  (`gram-plugins`, wpisy `board`, `topography`, `atmosphere`, `light`, `minimap`).

## Próbowano i porzucono

Płaska plansza przez topografię z zerowym reliefem (droższa niż własny renderer), lustro chunków
na GPU (patrz `gram-perf-tick`). Nie proponować bez nowego powodu.
