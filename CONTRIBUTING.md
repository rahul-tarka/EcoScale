# Contributing to EcoScale

## One repo, one working tree (recommended)

To avoid merge conflicts and duplicate “xgw / bra / …” worktrees:

1. **Use a single clone** on your machine (e.g. `~/code/eco-scale-optimizer`).
2. **Work on `main`** (or a short-lived branch from `main`), pull before you start, push when done.
3. **Avoid Git worktrees** unless you know you need two checkouts at once (e.g. hotfix + feature). Extra worktrees often diverge and cause painful merges.

### Cursor

- Prefer **opening the same folder** every time instead of creating new worktrees from Cursor.
- If you use **Setup Worktree** scripts, keep them minimal (this repo is Go-only; no `npm install` at root).

### If you already have extra worktrees

```bash
git worktree list
git worktree remove <path>   # after merging or stashing changes
```

---

## Build & test

```bash
go build ./cmd/ecoscale
go test ./...
```

---

## Pull requests

- Keep changes focused; rebase or merge from `main` before opening a PR.
- Update `README.md` if you change user-facing behavior (APIs, env vars, Docker Compose).
