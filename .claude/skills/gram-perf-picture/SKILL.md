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
  Po `f/photos` (2026-11-07): krata kładzie ćwiartki po najwyżej 8 tekseli boku (`photoQuad`;
  Madera 8 na L4, 4 na L5), a szczyt narożnika liczy osobny przebieg (`tops.wgsl`, teksel na
  narożnik) tylko gdy zmieni się wersja fotografii sektora (`Read.Painted`), nigdy na wierzchołek
  co klatkę; wcześniej 32×32 ćwiartki na każdy sfotografowany sektor czytały do 75 tekseli na
  wierzchołek i widok startowy Madery szedł 38 FPS wobec 110 na `main`, po poprawce 123–135;
  `iso,level=4,sheet` +0,34 ms (samotne latarnie benchmarku podnoszą margines ćwiartki wokół
  siebie), gęste lasy zyskują. Pułapka pomiaru: klatkarz etapu mierzyć w widoku startowym dema
  i na każdym poziomie, który treść wypełnia, nie tylko w reżimie, nad którym etap pracował.
  Pasy cienia idą w tempie słońca (`f/shine`, 2026-11-08): pas, gdy słońce obróciło się o 1/16°
  od ostatniego, najwyżej jeden na klatkę (dzień 16-minutowy: ~6 pasów/s zamiast pasa co klatkę
  nad 31 tys. bloków; iGPU i CPU dzielą budżet mocy, więc oszczędność GPU wraca w ticku: +35 %
  FPS przy rozwiniętej wyspie); skok godziny piecze pasy od góry po jednym na klatkę zamiast
  całości w jednej (2,8 FPS). Stałe klatki (`sky.Distant`, `Air`, cień promienia na powierzchnię
  w górę) czytane raz na kompozycję, nie na sprite: ~6 ms/klatkę przy 31 tys. drzew, obraz co do
  bitu ten sam. Zoom startowy kamery rośnie do progu, przy którym świat wypełnia okno (0,18 /
  0,28 / 0,38 przy 1024 / 1920 / 2560 px), więc progi drabiny dema ocenia się na pełnym ekranie.
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
  dalej: odrzucić trójkąty z zatopionym narożnikiem, blend kopiujący w `DrawMeshOptions` (3 draw
  mniej na rundę), albedo i relief w jednym obrazie (zmienia `Read.Sheet`). Głębsze topienie
  narożnika bez szczytu (pod najniższy grunt) sprawdzone 2026-11-09 i cofnięte: w FPS bez
  różnicy (skos i tak chował się za gruntem), a na grzbiecie, za którym grunt opada, skos wisiał
  jak kurtyna od korony w dół na tle nieba. Ścianki płacht (drugi draw instancjonowany na
  krawędziach kratki, 2026-11-09) sprawdzone i porzucone: w oknie 1024×768 kosztowały 0,7 ms
  klatki jako prostokąty i ~2,2 ms cięte drzewo po drzewie (8 tekseli szukania na fragment), w
  pochylonej orto widoku startowego 1,8 ms; i tak wyglądały źle. Kamera z okiem nie rysuje
  płacht wcale: stojące rzeczy sektorów L4/L5 idą dla niej jako punkty (`renderer.standIn`, ta
  sama ścieżka `render.Sprites.Point` co luźne jednostki z daleka); koszt w perspektywie nad
  lasem Madery: patrz BENCHMARKS.md „The forests in view"
  (tam klatkę ogranicza tick). Przebieg `Cutout` (render, 2026-11-09: figury całe z zapisem głębi
  i odcięciem alfy 0,5, bez sortowania) w tym samym reżimie nic nie zmienia (billboardy już pisały
  głębię i odcinały alfę): zysk jest w poprawności, nie w klatce; większa dźwignia to głębia drzew
  przed przebiegiem koloru gruntu (roadmap).
- Kafle płacht (`sheet_files.go`, `d/files`, 2026-11-06): kafel 512 tekseli, PNG przez
  `image.NRGBA` nad bajtami (pułapka: `image.RGBA` jest premultiplied i koder dzieli kolor
  każdego półprzykrytego teksela przez alfę); brudny kafel pisany 2 s po ostatniej fotografii w
  nim, najwyżej jeden na sekundę: `CopyFrom` i `ReadPixels` w klatce, `png.BestSpeed` w
  goroutine, `.tmp` i rename. `World_SheetFiles` (pakiet `plugins/world`, `measure.sh` go nie
  widzi): `load,tile` i `save,tile`; liczby w BENCHMARKS. Drugi bieg Madery ma pełną płachtę od
  pierwszej narysowanej klatki (klatki 5 i 10 różnią się od 120 o 40 px, bez kafli o ~30 tys.);
  klatka 1 niczego nie dowodzi, świat dopiero się budzi. Dźwignia, gdyby zapis przeszkadzał:
  kompresja (BestSpeed → brak) albo zapis rzadziej.
- Cień gruntu w kafelkach (`compose/bake.go`, `d/shade`, 2026-11-07): światło publikuje bloki
  w zestawach po sektorze z wersją (`Lights.Casts`; suma hashy bloków, więc kolejność zapytania
  nie gra roli; 0 alokacji; `Light_Publish` ~0,6 ms na 30 000 bloków: ~0,25 przebieg, ~0,18
  `Grid.Of` (dwa dzielenia na blok), ~0,15 hash). Baker trzyma własną kopię zestawów, zestaw o
  nowej wersji brudzi kafelki 256 px w zasięgu cienia starego i nowego pudła, 8 kafelków na
  klatkę, bez przejścia; obrót słońca dalej pasami z `Mix`. Pokład mostu to jeden blok
  jego uchwytu (`world.Whole` z `Lie`; `t/whole`), baker nic nie skleja (`joined` odeszło; przed
  uchwytem sklejanie całego świata stawiało Maderę na ~26 s). Pułapki: przełącznik
  `Coarse` z dźwigni budżetu klatki wciąż piecze cały cień (~42 000 draw po 16 pasach), bo obraz
  trzyma wysokości względem rozpiętości; zmiana rozpiętości (najwyższy szczyt) też. Czworokąt
  bloku w `bake_blocks.wgsl` kończy się tam, gdzie linia cienia spotyka grunt (marsz po ¾
  komórki jak w cieniu reliefu), nie dalej niż `render.ShadowReach` (6) wysokości bloku — to
  reguła domu dla każdego cienia (CLAUDE.md, 2026-11-10), ta sama w `reach`/`shading` po stronie
  CPU; przedtem sięgał do najniższego gruntu mapy (Madera o 18:30: 28 km, każdy kafelek widział
  każde drzewo, klatka stawała), a ograniczenie żyło w trzech miejscach pod trzema nazwami.
- Płaska plansza: CPU pisze dane komórek, shadery liczą kształty; wypiek paintera idzie na GPU.
- Plansza, relief, pogoda, światło i znaki minimapy publikowane raz na klatkę; renderery czytają
  kopię, nigdy żywy stan.
- Pętla `renderer.Compose` nie sięga do map na encję (2026-11-08): `shaded`, `lying`, `standing`,
  `edges` są tablicami po sprite'ach (`flags`, `shadedOf`, `edgeOf`, `table()` przy `shade`/`lie`);
  cztery `mapaccess` na warstwę na encję były piątą częścią Compose nad 26 tys. drzew na L3 (0,7 ms
  na klatkę). `View.Fog` liczy pierwiastek tylko w pasie mgły (porównanie kwadratów przed nim): 0,45
  ms na klatkę przy 26 tys. encji. Zostaje: `billboards.sprites.add` czyta relief (`GroundAt`) na
  billboard na klatkę (0,7 ms przy 26 tys.; rzecz `Fixed` stoi na gruncie, więc jej `Z.Altitude` to
  ta wysokość: flaga w rekordzie figury), `light.standing` buduje bloki `Baked` od nowa co klatkę
  (1,2 ms przy 26 tys. drzew; warstwa nieruchoma nie zmienia się, ale `Z` ustawiany po spawnie i bez
  wersji nie daje bramki).

## Odrzucone

- Płaska plansza przez topografię z zerowym reliefem: droższa niż własny renderer.
- Lustro chunków na GPU: `gram-perf-tick`.
