# s5. aabbworld: promienie w lockstep po polu wysokości

**Repo, gałąź:** aabbworld, `s/lockstep` od `perf/sight-simd` po scaleniu części A, B, C
(worktree `/home/karol/worktrees/aabbworld/lockstep`). Moduł buduje się sam, bez `go.work`.
**Etap:** `doc/plans-perf.md` gram „SIMD w widzeniu", część E (dopisana 2026-10-26 po
profilu).
**Lektura:** `internal/raycast/heightfield.go` (walk po siatce: `lattice.walk`, `folded`,
`crossedX/Y`, `point`, `take`), `elevation.go` (`castElevated`, `walkProfile`, `ray.ground`),
`sweep.go` (`walk`, bloki kątów, `caster.refine`), `slab.go` i `slab_simd.go` (jak część A
kładzie slaby czterema promieniami naraz we float64, bramka `wide`, `ClearAVXUpperBits`),
`heightfield_test.go` (oracle `fieldOracle`: punkty profilu bit w bit), `doc.go` (Numbers,
SIMD).

## Cel

Po części C profil gruntu z `Cone.Heights` liczy się w aabbworld bez callbacków, ale promień po
promieniu: `lattice.walk` to 52% ticku `Vision_Relief/profile` w gram i 68% szczebla
`View_Elevated/heights=field`; w nim arytmetyka (`folded`, `point`, `clamp01`) ok. połowy, loady
narożników i appendy (`crossedX/Y`, `take`) ok. jednej czwartej. Promienie jednego skanu wychodzą z
jednego punktu pod sąsiednimi kątami, więc idą po tych samych komórkach w podobnym rytmie: walk
może prowadzić cztery promienie naraz we float64 (`Float64x4`, dwa rejestry na wielkość tam, gdzie
trzeba osiem), każdy pas z własnym stanem DDA, maskowany tam, gdzie pasy się rozchodzą (inna
komórka, fałda poza komórką, uskok na krawędzi, koniec). `archsimd` nie ma instrukcji gather:
narożniki ładuje się per pas skalarnie, appendy też są per pas. Punkty każdego promienia mają być
**bit w bit** te same co z walku skalarnego (ta sama kolejność operacji IEEE na pasie).

## Pliki

Wolno: `internal/raycast/heightfield.go`, nowe `heightfield_simd.go`
(`//go:build goexperiment.simd && amd64`) i `heightfield_scalar.go`, `elevation.go` (tylko
`castElevated`/`walkProfile`: profil czytany z bufora bloku), `sweep.go` (tylko `walk`: bloki
kątów dla `castElevated`, jak dla slabów; `refine` zostaje po jednym), `raycast.go` tylko pole
scratch, testy pakietu, `bench/view_test.go` (nowe szczeble tylko jeśli potrzebne). Nie wolno:
`slab*.go` (gotowe), `internal/spatial/*`, `view.go`, `space.go`, `doc.go`, `BENCHMARKS.md`,
`CHANGELOG.md`, `README.md`, `go.mod`.

## Kroki

1. Bufor profili bloku w `scratch`: dla bloku do czterech (albo ośmiu) kątów dzielących zbiór
   aktywnych kandydatów, profile `d`/`alt` per promień, z bufora czyta `castElevated`
   (`walkProfile` z bloku zamiast per promień). `walk` grupuje kąty jak dla slabów (ten sam
   podział na przebiegi; bloki krótsze niż `minWide` idą skalarnie).
2. Jądro lockstep w `heightfield_simd.go`: stan DDA per pas (`cx, cy, nextX, nextY, t`), krok
   komórki wspólny z maskami: pasy, które wychodzą z komórki po X, po Y albo kończą się, osobno;
   fałda liczona wektorowo (`folded`: wybór przekątnej, dzielenie, clamp), punkty appendowane
   per pas do jego profilu w tej samej kolejności co skalarnie. Narożniki per pas skalarnie
   (dwa nowe na komórkę, dwa z poprzedniej jak w `crossedX/Y`).
3. Bramka jak w części A: `wide` w pliku simd, `const wide = false` w skalarnym;
   `ClearAVXUpperBits` na wyjściu. W buildzie zwykłym profil liczy się jak dziś.
4. Testy: (a) dla losowych pól, początków i wachlarzy kątów profil każdego promienia z lockstepu
   równa się walkowi skalarnemu **dokładnie** (oracle `fieldOracle` już jest); (b) skan
   lockstep vs skalarny: `Entities`, `Depths`, `Shadows` bit w bit, w obu buildach; (c) bloki o
   1, 2, 3, 4 kątach, promienie rozchodzące się do różnych komórek, pas kończący się wcześniej
   (krawędź siatki), torus; (d) istniejące testy bez zmian oczekiwań.
5. Bez benchmarków po stronie agenta (nawet dymnych): decyzje z kodu i testów; polecenia do
   zmierzenia w raporcie.

## Poza zakresem

Mip wysokości i pomijanie gruntu. Osłona (`castCover`). Grunty bez siatki (`Ground`, `Profile`).
Slaby (A). Gram.

## Zasady

Sekwencja przed każdym commitem: `go build ./... && go vet ./... && gofmt -l . && go test ./...`
oraz `GOEXPERIMENT=simd go test ./...`. Commity konwencjonalne (`perf:`, `test:`), trailer
`Claude-Session: https://claude.ai/code/session_014GbD5V5FZgxcEYK1zPRzjF`, bez `Co-Authored-By`.
Bez pushu. Liczby mierzy integrator na bezczynnej maszynie. Komentarze tylko tam, gdzie kod nie
powie; linie ≤ 100 kolumn.

## Raport

Co idzie w lockstep, a co zostało per pas (loady, appendy) i dlaczego; udział pasów zajętych
(pomiar liczników na instrumentowanej kopii, jak w części A); lista plików; dokładne polecenia
benchmarków (`View_Elevated/.../heights=field`, `View_Elevated/.../heights=profile`, gram
`Vision_Relief/profile`) i oczekiwany kierunek; propozycja wpisu do CHANGELOG i zdania do
`doc.go`; co nie weszło. Jeśli lockstep nie da się zrobić bit w bit albo wychodzi dłuższy niż
skalar w kodzie bez wyraźnego zysku do pokazania, raport ma to powiedzieć zamiast zmuszać kod.
