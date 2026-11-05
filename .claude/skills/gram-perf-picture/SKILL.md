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
- Drabina `lod` ma trzy skale po dwa szczeble (dokładny i zgrubny) i pominięcie: L0–L1 bliska
  (`lod.Close`; gra wchodzi w nią, ustawiając dwa pierwsze progi, zero tam znaczy, że skali nie
  używa), L2–L3 standardowa (`lod.Standard`), L4–L5 strategiczna (`lod.Strategic`), L6
  pominięta (`lod.Omitted`). Siedem szczebli `Paces` liczy się od najdrobniejszego poziomu
  faktycznie widzianego, a `world.Plugin.Within(first)` zawęża system do encji jednej skali
  (wyklucza znaczniki pozostałych poziomów, bo goke nie ma OR), więc system bliskiej skali tyka
  tylko tam, gdzie ktoś patrzy z bliska.
- Grunt w rozdzielczości kamery, z wariantem shadera po poziomie sektora z widoku kamery
  (`lod`, jedna drabina: L2 pełnia, L3 bez brzegu i wody biegnącej, L4 siatka co drugi narożnik
  bez fal z albedo w pół, L5 siatka co czwarty narożnik z albedo w ćwierć, L6 nic; żadnych
  pierścieni staranności); figury ostre nad nim, w każdym pikselu, każdy rodzaj wersją swojego
  poziomu (L4 i L5 plama koloru, na L5 rzedzona `Keep`); to, co stoi (`world.Fixed`), od L4 w
  płachcie sektora: sfotografowane na GPU z wyglądu L3 (albedo i relief: szczyt f16 i normalna),
  rysowane przez grunt jako relief w świetle klatki, na L5 w pół (2026-11-05).
- Rozdzielczość i poziom obrazu idą za budżetem klatki (adapter kamer: dźwignie, przesunięcie
  poziomu o 1–2 dla każdej kamery, najwyżej do L5 i nigdy sektora widzianego na L2 lub bliżej, potem
  rozdzielczość): pomiar ustawia
  rozdzielczość (`cameras.Picture{Resolution}`) i uspokaja przesunięcie
  (`cameras.Detail{Follows: false}`) albo wyłącza warstwy, inaczej mierzy adapter.
- `Frame_Topography/…,level=k` rysuje każdy widziany sektor na poziomie k: pole wariantów gruntu.
- Figura na L4 to punkt bez `Look` (~43 ns na encję w `World_Draw/…,level=4` wobec ~85–99 przez
  `Look`; reszta to odczyt poziomu i rekordów: rekord zwarty dla L4 przy publikacji to następny
  krok, jeśli trzeba); rzedzenie `Keep` przy publikacji po stałym haszu id, zero = wszystko
  (ścieżka `sees` bajt w bajt jak bez rzedzenia; hasz ~7 ns na każdą encję w kadrze: `keep=0.25`
  publikuje 65 → 99 µs przy 5000, rysowanie oszczędza ~150 µs — opłaca się, gdy rysowanie
  przeważa; tańszy hasz do spróbowania).
- Suma koloru na L5 (etap `g`) odeszła w etapie `p` (2026-11-05): figura na L5 to plama jak na
  L4, a to, co stoi, siedzi w płachcie i nie ma rekordu w sektorze namalowanym. Fotografia
  sektora to dwa przebiegi `Primed` do buforów sektorowych z własną głębią, `CopyFrom` do płachty
  i przebieg zmniejszający do L5 (~10 przebiegów na sektor; najwyżej 8 sektorów na publikację;
  `World_Sheet/sector,64` i `frame,8`); następna dźwignia, gdyby trzeba: wiele sektorów w jednym
  buforze. Sygnatura treści liczy się przy publikacji dla każdego sektora rysowanego na L4+, nie
  tylko widzianego, inaczej sektory brzegowe fotografowałyby się przy każdym przesunięciu
  kamery. Krata płachty rysuje się tylko dla sektorów, które coś trzymają (`Read.Painted`):
  pusty sektor nic nie kosztuje; `Frame_Topography/iso,level=4,sheet` to grunt L4 z płachtą.
  Po `p/photo` (2026-11-06): runda do `perFrame` sektorów fotografuje się jednym rysowaniem
  instancjonowanym do jednej ciemni (albedo po lewej, relief po prawej, każda rzecz trzymana w
  swoim slocie przez shader fragmentów; czyszczenie rysowaniem zera w tym samym przebiegu, nie
  `Clear`, bo to osobny przebieg), kopie do płacht i połówki L5 jednym rysowaniem na obraz
  (prostokąt najpierw wypełniony jedynkami, wartości kładzione `Darkest`, żeby teksel L5
  wspólny dwóm sektorom rundy wyszedł jak przy sektorach po jednym): 5 przebiegów i ~10 draw
  na rundę zamiast 10 przebiegów i ~12 draw na sektor (`frame,8` 6,2 → 2,0 ms). Prawie każda
  alokacja to wgpu: ~45 na przebieg i ~25 na draw (goffi); sam bench `World_Sheet` ma podłogę
  ~0,45 ms i 436 alokacji (publikacja, kompozycja, kopia 1×1 i odczyt na czekanie na GPU), więc
  `sector,64` ≈ 0,8 ms nad nią. Krata płachty rysowana przez bufor indeksów nad wspólnymi
  narożnikami (6 wierzchołków na kwadrat bez dzielenia kosztowało 0,8 ms); reszta kosztu
  (`level=4,sheet` ≈ +1,4 ms nad `level=4`) to raster podniesionej geometrii: marginesy wokół
  szczytów i ściany od szczytu do komórki pod gruntem (912 tys. pikseli na 1440p przy kilku
  tysiącach pikseli latarni); shader fragmentów tylko z `discard` kosztuje tyle samo. Dźwignie
  dalej: odrzucić trójkąty z zatopionym narożnikiem albo topić dalsze narożniki głębiej (zmienia
  obraz: półprzezroczysty obrys pod szczytem to frędzel albedo na ścianach), blend kopiujący w
  `DrawMeshOptions` (3 draw mniej na rundę), albedo i relief w jednym obrazie (zmienia
  `Read.Sheet`).
- Płaska plansza: CPU pisze dane komórek, shadery liczą kształty; wypiek paintera idzie na GPU.
- Plansza, relief, pogoda, światło i znaki minimapy publikowane raz na klatkę; renderery czytają
  kopię, nigdy żywy stan.

## Odrzucone

- Płaska plansza przez topografię z zerowym reliefem: droższa niż własny renderer.
- Lustro chunków na GPU: `gram-perf-tick`.
