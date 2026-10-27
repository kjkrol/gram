---
name: gram-pr
description: How a PR of gram is readied for review — the squash commit message (what the PR brings, grouped Added, Fixed, Changed, Removed, Improved; short; no test plan; no word of the tool that wrote it), the PR's title and body made from it, the branch squashed to one commit and force-pushed, CI left to the user. Use when the user asks to squash, to write a squash or PR message, or to report a PR ready for review.
---

# gram-pr

The user reviews and merges every PR. A PR goes to review as one commit whose message is the
PR's title and body: the repository's squash merge takes a single commit's title and message
(`COMMIT_OR_PR_TITLE`, `COMMIT_MESSAGES`), so that message is what lands on `main`.

## The message

- The title: a conventional prefix (`feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `chore`),
  `!` where the API breaks, then what the PR brings in a few words; no `(#N)`, GitHub adds it.
- The body says what the PR brings and nothing else: the groups in bold, **Added**, **Fixed**,
  **Changed**, **Removed**, **Improved**, those with content, in that order; a bullet a thing,
  one to three lines, a name in backticks where a reader must find it; "Breaking:" opens a
  bullet that breaks the API.
- Short enough to be read, not an epic: no history (what was tried, which commit did what), no
  how, no test plan, no checklist. The by-eye checks and the open questions go to the user in
  the chat, not into the PR.
- No word of the tool that wrote it: no "Generated with", no `Co-Authored-By`, no session
  trailer. The working commits on the branch carry the session trailer; the squash commit and
  the PR's body carry none.
- Wrapped at 100 columns in the commit; the PR's body is the same text with each bullet's lines
  joined, since GitHub breaks a line where the text does.

## The squash

The verification sequence has run green and the demos the change touches have run before. Then,
with the message in `squash.md` (the title, a blank line, the body):

```bash
git fetch -q origin main
git reset --soft "$(git merge-base origin/main HEAD)"    # the tree stays as it is
git commit -F squash.md
git push --force-with-lease origin <branch>
awk 'NR<3{next} /^  /{sub(/^  +/," "); b=b $0; next} {if(n++)print b; b=$0} END{print b}' \
    squash.md > pr-body.md
gh pr edit <N> --title "$(head -1 squash.md)" --body-file pr-body.md
```

Report the PR ready for review and stop there: the user waits for CI and reviews it themselves.
