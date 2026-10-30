---
name: gram-perf-picture
description: Wydajność klatki i obrazu grama — pierścienie jako warstwy kamery, grunt w rozdzielczości kamery z figurami ostrymi, jedna kompozycja na warstwę, rozdzielczość za budżetem klatki, płaska plansza na GPU, publikacja raz na klatkę; niezmienniki i odrzucone pomysły; benchmarki World_Draw, World_Frame, Frame_*, Text_Panel, Painter_Bake. Czytaj przy zmianie render, Composer, Direct, warstw, painterów, shaderów, gdy klatka zwolniła, i przed propozycją zmiany ścieżki rysowania.
---

# gram-perf-picture

Protokół i pułapki pomiaru: `gram-perf`; jak się rysuje: `gram-render`. Liczby: `BENCHMARKS.md`
(„The island's frame", „The flat board's frame", „The demos", „Where the time goes"); etapy:
`doc/plans-perf.md` (R, R2, T, U, „Płaska plansza na GPU").

## Pomiar

- `Frame_Topography` (wyspa w 1440p) to pole każdej zmiany `render`.
- `Painter_Bake` (`plugins/topography/internal/painter`) i `Composer_Render`
  (`render/internal/compose`) leżą poza `bench/`: `measure.sh` ich nie widzi.
- Klatki dem i ich GC mierzy `gram-probe`; klatka headless niesie readback (`CLAUDE.md`).

## Niezmienniki

- Direct rysuje raz na warstwę i trzyma to, co zebrał Compose; sprite'y, trasy i widoki złamały
  to po razie (`gram-render`).
- Jedna kompozycja na warstwę kamery, z dziurą pod warstwami wewnętrznymi.
- Grunt w rozdzielczości kamery, z rozdzielczością obrazu i starannością po pierścieniach; figury
  ostre nad nim, w każdym pikselu.
- Rozdzielczość idzie za budżetem klatki (adapter kamer): pomiar ją ustawia
  (`cameras.Picture{Resolution}`) albo wyłącza warstwy, inaczej mierzy adapter.
- Płaska plansza: CPU pisze dane komórek, shadery liczą kształty; wypiek paintera idzie na GPU.
- Plansza, relief, pogoda, światło i znaki minimapy publikowane raz na klatkę; renderery czytają
  kopię, nigdy żywy stan.

## Odrzucone

- Płaska plansza przez topografię z zerowym reliefem: droższa niż własny renderer.
- Lustro chunków na GPU: `gram-perf-tick`.
