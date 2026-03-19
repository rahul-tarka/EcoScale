# EcoScale — World's First Carbon-Aware Kubernetes Controller

> **GreenOps for Kubernetes.** EcoScale intercepts scheduling decisions based on real-time carbon intensity (CO2 per kWh) of cloud regions, enabling workloads to chase the sun and reduce cloud carbon footprint.

[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)

---

## Why EcoScale?

**Traditional Kubernetes schedulers only care about CPU and RAM.** They have no concept of:

- **Carbon intensity** — How much CO2 is emitted per kWh in the region powering your cluster
- **Time-of-day** — Solar peaks, wind patterns, and grid mix vary by hour
- **Region comparison** — us-west-2 (hydro-heavy) can be 2–3× greener than us-east-1 (fossil-heavy)

EcoScale is the **first controller** that:

1. **Fetches real-time carbon intensity** from CarbonIntensity.org.uk / ElectricityMaps
2. **Labels flexible workloads** — Only workloads with `ecoscale/flexible=true` are considered
3. **Recommends actions** — Scale-down, node-drain, or region-shift when intensity exceeds threshold
4. **Sun-Chaser logic** — Compares regions (e.g., us-east-1 vs us-west-2) and suggests Karpenter/Cluster Autoscaler config to shift capacity to the greener region

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          EcoScale Controller                             │
├─────────────────────────────────────────────────────────────────────────┤
│  internal/carbon/client.go     → Carbon intensity API (mock / live)      │
│  internal/kubernetes/analyzer → List pods with ecoscale/flexible=true   │
│  internal/optimizer/engine.go → Brain: threshold + Sun-Chaser logic     │
└─────────────────────────────────────────────────────────────────────────┘
         │                    │                         │
         ▼                    ▼                         ▼
   CarbonIntensity.org   Kubernetes API           Prometheus /metrics
   ElectricityMaps      (pods, nodes)             CO2_Saved_Total
```

### Core Components

| Component | Purpose |
|-----------|---------|
| `internal/carbon/client.go` | Fetches carbon intensity by region. Mock client included; pluggable for CarbonIntensity.org.uk or ElectricityMaps. |
| `internal/kubernetes/analyzer.go` | Uses client-go to list pods with `ecoscale/flexible=true`. Detects cluster region from node labels. |
| `internal/optimizer/engine.go` | **The brain.** If intensity > threshold → suggest scale-down/node-drain. Sun-Chaser: compare regions, suggest Karpenter/Cluster Autoscaler config to shift to greener region. |

### Sun-Chaser (Unique Feature)

EcoScale compares carbon intensity between regions (e.g., **us-east-1** vs **us-west-2**):

- **us-west-2** (Oregon) — Hydro-dominated grid, ~180 gCO2/kWh
- **us-east-1** (Virginia) — Fossil-heavy, ~420 gCO2/kWh

When your cluster runs in a high-carbon region, EcoScale outputs:

- **Karpenter NodePool** YAML targeting the greener region
- **Cluster Autoscaler** guidance for multi-region scaling

---

## Website

A product website lives in `web/index.html` (and `docs/index.html` for GitHub Pages) with What, Why, How, For Whom, Features, and live GitHub stats (stars, forks, contributors). To host on GitHub Pages:

1. In repo **Settings → Pages**, set Source to **Deploy from a branch**
2. Branch: `main`, Folder: **`/docs`**
3. Site will be live at `https://rahul-tarka.github.io/eco-scale-optimizer/`

To preview locally: `cd web && python3 -m http.server 8000` then open http://localhost:8000

---

## Quick Start

### 1. Build & Run (Standalone)

```bash
# Build
make build

# Run (no Kubernetes required for mock mode)
ECOSCALE_IN_CLUSTER=false ./bin/ecoscale
```

### 2. Docker

```bash
docker build -t ecoscale:0.1.0 .
docker run -p 8080:8080 ecoscale:0.1.0
```

### 3. Helm (Deploy to kube-system)

```bash
helm install ecoscale ./helm/ecoscale -n kube-system
```

> **Note:** Build the Docker image first, or set `image.repository` and `image.tag` in values to your registry.

### 4. Production: Live Carbon Data

**Option A — CarbonIntensity.org.uk (free, UK zones):**
```bash
ECOSCALE_CARBON_API=carbonintensity ./bin/ecoscale
```

**Option B — ElectricityMaps (global, requires API key):**
```bash
ECOSCALE_CARBON_API=electricitymaps ECOSCALE_CARBON_API_KEY=your-key ./bin/ecoscale
```

### 5. Production: Enable Execution (optional)

By default, EcoScale runs in **dry-run** mode (recommendations only). To execute pod evictions:

```bash
ECOSCALE_DRY_RUN=false ECOSCALE_ENABLE_EXECUTION=true ./bin/ecoscale
```

Safety limits apply: max 10% of flexible pods evicted per cycle; protected workloads (`ecoscale/protected=true`) are never evicted.

---

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `ECOSCALE_ADDR` | `:8080` | HTTP listen address |
| `ECOSCALE_INTERVAL` | `5m` | Reconciliation interval |
| `ECOSCALE_CARBON_THRESHOLD` | `350` | gCO2/kWh — above this, suggest scale-down |
| `ECOSCALE_IN_CLUSTER` | `true` | Use in-cluster Kubernetes config |
| `ECOSCALE_CARBON_API` | `mock` | Carbon data source: `mock` \| `carbonintensity` \| `electricitymaps` |
| `ECOSCALE_CARBON_API_KEY` | — | ElectricityMaps API key (required when `ECOSCALE_CARBON_API=electricitymaps`) |
| `ECOSCALE_DRY_RUN` | `true` | If `true`, only recommend; never execute evictions |
| `ECOSCALE_ENABLE_EXECUTION` | `false` | If `true` and not dry-run, execute pod evictions |
| `ECOSCALE_EVICTION_CAP_PCT` | `10` | Max % of flexible pods to evict per cycle (0–100) |

---

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | API info |
| `GET /health` | Health check |
| `GET /metrics` | Prometheus metrics |
| `GET /recommendations` | Live optimization recommendations (JSON) |

### Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ecoscale_co2_saved_total` | Counter | Total estimated CO2 saved (grams) |
| `ecoscale_carbon_intensity_gco2_per_kwh` | Gauge | Current region carbon intensity |
| `ecoscale_recommendations_total` | Counter | Recommendations by type |
| `ecoscale_reconciliation_runs_total` | Counter | Reconciliation cycles |
| `ecoscale_reconciliation_errors_total` | Counter | Failed cycles |

---

## Mark Workloads as Carbon-Flexible

Add the label to pods that can be rescheduled or scaled based on carbon:

```yaml
metadata:
  labels:
    ecoscale/flexible: "true"
```

EcoScale will **only** consider these pods for scale-down or node-drain recommendations. System-critical pods (kube-system, DaemonSets) are never suggested for drain.

### Protect Workloads from Eviction

Add `ecoscale/protected: "true"` to workloads that must never be evicted:

```yaml
metadata:
  labels:
    ecoscale/flexible: "true"
    ecoscale/protected: "true"   # Never evict, even when carbon is high
```

---

## Project Structure

```
ecoscale/
├── cmd/ecoscale/main.go          # Entrypoint, HTTP server, reconciliation loop
├── internal/
│   ├── carbon/
│   │   ├── client.go             # Carbon intensity client (mock + interface)
│   │   ├── carbonintensity.go    # CarbonIntensity.org.uk (free, UK)
│   │   ├── electricitymaps.go    # ElectricityMaps (global, API key)
│   │   └── types.go              # Intensity, RegionMapping
│   ├── config/
│   │   └── config.go             # Runtime config (dry-run, eviction cap, etc.)
│   ├── executor/
│   │   └── executor.go           # Pod eviction execution
│   ├── kubernetes/
│   │   └── analyzer.go           # Pod/node discovery via client-go
│   ├── metrics/
│   │   └── metrics.go            # Prometheus metrics
│   ├── optimizer/
│   │   ├── engine.go             # Brain: threshold + Sun-Chaser
│   │   ├── types.go              # Recommendation, RegionShiftRecommendation
│   │   └── result.go             # Result struct
│   └── safety/
│       └── limits.go             # Dry-run, eviction cap, protected workloads
├── helm/ecoscale/                # Helm chart for kube-system
├── Dockerfile
├── Makefile
└── README.md
```

---

## Production Features

- [x] **Live Carbon API** — CarbonIntensity.org.uk (free, UK) and ElectricityMaps (global, API key)
- [x] **Safety Layer** — Dry-run mode, 10% eviction cap, `ecoscale/protected=true`
- [x] **Execution** — Pod eviction when `ECOSCALE_ENABLE_EXECUTION=true` and `ECOSCALE_DRY_RUN=false`

## Roadmap

- [ ] **Webhook Scheduler** — Intercept pod scheduling (not just recommendations)
- [ ] **Multi-region Karpenter** — Auto-apply NodePool changes
- [ ] **Carbon budget** — Enforce daily/weekly CO2 caps per namespace

---

## License

Apache 2.0

---

**EcoScale — Building the future of Green Computing.**
