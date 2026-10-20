# Briefy sesji

Jeden plik to jedna sesja Claude Code, którą użytkownik otwiera w repozytorium i na gałęzi,
jakie brief nazywa. Kolejność, zależności i stan: `doc/plans-perf.md`, sekcja „Wykonanie w
sesjach”. Zasady wspólne każdej sesji:

- Przeczytać `CLAUDE.md` repozytorium i skille, które brief wymienia; w gram pracować w
  worktree poza `/home/karol/CODE` (`git worktree add /home/karol/worktrees/gram/<część>
  -b <gałąź> main`), bo `go.work` w `/home/karol/CODE` nie buduje zagnieżdżonych worktree.
- Dotykać tylko plików, które brief wymienia; szwów nie ruszać. Dokumenty, skille,
  `BENCHMARKS.md`, `CHANGELOG.md`, `go.mod` i `go.work` są integratora: sesja proponuje ich
  treść w raporcie.
- Przed każdym commitem: `go build ./... && go vet ./... && gofmt -l . && go test ./...`
  (testy GPU w gram: `GRAM_GPU=software` headless; test staranności wody może pod nim nie
  przejść, to znane). Dema tylko w kopii (`gram-probe` § Checking a change), nigdy w
  `examples/`.
- Zmiana wydajnościowa jest mierzona przed i po, naprzemiennie, na kopii drzewa sprzed
  zmiany i na drzewie roboczym, na bezczynnej maszynie (sprawdzić obciążenie przed pomiarem),
  z profilem CPU przy zmianie w gorącej pętli (`-cpuprofile`, `go tool pprof -top`).
- Commity na własnej gałęzi: prefiksy konwencjonalne, bez `Co-Authored-By`, trailer
  `Claude-Session: <adres sesji>`; linie do 100 kolumn; komentarze tylko kontraktu.
- Raport na końcu (w ostatniej wiadomości sesji): commity; co zmierzono przed i po, w jakim
  reżimie; co odbiega od briefu i dlaczego; czego nie zrobiono; proponowane wpisy do
  `CHANGELOG.md` i skilli.
