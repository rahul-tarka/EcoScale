# Contributing

- Work on a **single clone** of `main` when possible (avoids multi-worktree merge friction).
- **Build:** `make build` or `go build -o bin/ecoscale ./cmd/ecoscale`
- **Run locally (mock carbon, no cluster):** `ECOSCALE_IN_CLUSTER=false ./bin/ecoscale` → open [http://localhost:8080/ui](http://localhost:8080/ui)
- **PRs:** keep changes focused; match existing style and defaults (dry-run safe).

Apache-2.0 — see repository license.
