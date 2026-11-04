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
- Grunt w rozdzielczości kamery, z wariantem shadera po poziomie sektora z widoku kamery
  (`lod`, jedna drabina: L0 pełnia, L1 bez brzegu i wody biegnącej, L2 siatka zgrubna bez fal,
  L3 płachta albedo malarza, L4 nic; żadnych pierścieni staranności); figury ostre nad nim, w
  każdym pikselu, każdy rodzaj wersją swojego poziomu (L2 plama koloru, L3 kolor zsumowany w
  grunt).
- Rozdzielczość i poziom obrazu idą za budżetem klatki (adapter kamer: dźwignie, przesunięcie
  poziomu o 1–2 dla każdej kamery, najwyżej do L3, potem rozdzielczość): pomiar ustawia
  rozdzielczość (`cameras.Picture{Resolution}`) i uspokaja przesunięcie
  (`cameras.Detail{Follows: false}`) albo wyłącza warstwy, inaczej mierzy adapter.
- `Frame_Topography/…,level=k` rysuje każdy widziany sektor na poziomie k: pole wariantów gruntu.
- Figura na L2 to punkt bez `Look` (~43 ns na encję w `World_Draw/…,level=2` wobec ~85–99 przez
  `Look`; reszta to odczyt poziomu i rekordów: rekord zwarty dla L2 przy publikacji to następny
  krok, jeśli trzeba); rzedzenie `Keep` przy publikacji po stałym haszu id, zero = wszystko
  (ścieżka `sees` bajt w bajt jak bez rzedzenia; hasz ~7 ns na każdą encję w kadrze: `keep=0.25`
  publikuje 65 → 99 µs przy 5000, rysowanie oszczędza ~150 µs — opłaca się, gdy rysowanie
  przeważa; tańszy hasz do spróbowania).
- Figura na L3 to kolor jej plamy zsumowany przy publikacji w obraz agregatu zamiast rekordu
  (rysowanie ~0): ~94 ns na encję w `World_Publish/…,level=3` przy pudle 20 na tekselu 16
  (~5 tekseli; sam dodatek ~25 ns na teksel, Go rozlewa rejestry; stały punkt i szybszy floor
  nie pomogły). Świat, którego gruntu nikt nie rysuje (ekspansja; `Aggregates` niewołane), idzie
  starą ścieżką: rung `level=3, unread` kosztuje tyle, co L2. Do spróbowania: jeden teksel pod
  środkiem, rekordy zwarte.
- Płaska plansza: CPU pisze dane komórek, shadery liczą kształty; wypiek paintera idzie na GPU.
- Plansza, relief, pogoda, światło i znaki minimapy publikowane raz na klatkę; renderery czytają
  kopię, nigdy żywy stan.

## Odrzucone

- Płaska plansza przez topografię z zerowym reliefem: droższa niż własny renderer.
- Lustro chunków na GPU: `gram-perf-tick`.
