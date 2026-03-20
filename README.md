# EcoScale — World's First Carbon-Aware Kubernetes Controller

> **GreenOps for Kubernetes.** EcoScale uses carbon intensity (CO₂ per kWh) by region to recommend scale-down, node-drain, and region-shift — so workloads can chase greener grids.

[![Go 1.22+](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-Apache%202.0-green.svg)](LICENSE)

> **Important — what you see on the dashboard**  
> By default EcoScale uses **demo (mock) data** so you can run it with **zero signup**. The UI shows a **Demo vs Live** banner at the top. For **real grid data**, follow [**Live carbon in 4 steps or less**](#live-carbon-in-4-steps-or-less) below.

---

## Why EcoScale?

**Traditional Kubernetes schedulers only care about CPU and RAM.** They have no concept of:

- **Carbon intensity** — CO₂ emitted per kWh in the region powering your cluster  
- **Time-of-day** — Grid mix changes by hour  
- **Region comparison** — e.g. us-west-2 can be much greener than us-east-1  

EcoScale:

1. Fetches carbon intensity (mock, **CarbonIntensity.org.uk**, or **ElectricityMaps**)  
2. Labels flexible workloads with `ecoscale/flexible=true`  
3. Recommends scale-down, node-drain, or region-shift when intensity is high  
4. **Sun-Chaser** — compares regions and suggests Karpenter / Cluster Autoscaler style config  

---

## Quick Start (Docker — 2 commands, demo data)

```bash
git clone https://github.com/rahul-tarka/eco-scale-optimizer.git && cd eco-scale-optimizer
docker compose up
```

Open **http://localhost:8080/ui** — dashboard with **demo** carbon numbers (banner explains this).

- **http://localhost:8080/** — JSON API info (`requested_carbon_api`, `effective_carbon_api`, `is_live_data`)  
- **http://localhost:8080/api/status** — same details for tools/automation  

---

## Live carbon in 4 steps or less

### Option A — UK grid, **free**, **no API key** (3 steps)

Uses [CarbonIntensity.org.uk](https://carbonintensity.org.uk/) (UK regional data). Cloud regions are mapped to UK zones where possible — good for a **real** signal without signup.

```bash
git clone https://github.com/rahul-tarka/eco-scale-optimizer.git && cd eco-scale-optimizer
printf 'ECOSCALE_CARBON_API=carbonintensity\n' > .env
docker compose up
```

Open **http://localhost:8080/ui** — banner should say **Live grid data**.

### Option B — Global zones, **ElectricityMaps** (4 steps)

1. Create a free API token at [ElectricityMaps](https://www.electricitymaps.com/) (developer / app dashboard).  
2. `git clone https://github.com/rahul-tarka/eco-scale-optimizer.git && cd eco-scale-optimizer`  
3. `cp .env.example .env` — set `ECOSCALE_CARBON_API=electricitymaps` and `ECOSCALE_CARBON_API_KEY=<your token>`.  
4. `docker compose up` → **http://localhost:8080/ui**  

### Run locally without Docker (live)

```bash
make build
ECOSCALE_IN_CLUSTER=false ECOSCALE_CARBON_API=carbonintensity ./bin/ecoscale
# or
ECOSCALE_IN_CLUSTER=false ECOSCALE_CARBON_API=electricitymaps ECOSCALE_CARBON_API_KEY=your_key ./bin/ecoscale
```

---

## Configuration

| Env Var | Default | Description |
|---------|---------|-------------|
| `ECOSCALE_ADDR` | `:8080` | HTTP listen address |
| `ECOSCALE_INTERVAL` | `5m` | Reconciliation interval |
| `ECOSCALE_CARBON_THRESHOLD` | `350` | gCO₂/kWh — above this, suggest scale-down / drain |
| `ECOSCALE_IN_CLUSTER` | `true` | In-cluster Kubernetes config |
| `ECOSCALE_CARBON_API` | `mock` | `mock` \| `carbonintensity` \| `electricitymaps` |
| `ECOSCALE_CARBON_API_KEY` | — | Required when `ECOSCALE_CARBON_API=electricitymaps` |

---

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /` | API info + carbon backend summary |
| `GET /api/status` | Demo vs live, messages for the dashboard banner |
| `GET /ui` | Carbon-Aware Dashboard |
| `GET /health` | Health check |
| `GET /metrics` | Prometheus metrics |
| `GET /recommendations` | Recommendations JSON (`?threshold=350`) |
| `GET /api/regions?regions=us-east-1,us-west-2` | Multi-region intensities |

---

## Build & Helm

```bash
make build
ECOSCALE_IN_CLUSTER=false ./bin/ecoscale
```

```bash
docker build -t ecoscale:0.3.0 .
docker run -p 8080:8080 -e ECOSCALE_IN_CLUSTER=false ecoscale:0.3.0
```

```bash
helm install ecoscale ./helm/ecoscale -n kube-system
```

---

## Website (GitHub Pages)

Product site: `web/index.html` and `docs/index.html`.  
Settings → Pages → branch `main`, folder **`/docs`**.

---

## Mark Workloads as Carbon-Flexible

```yaml
metadata:
  labels:
    ecoscale/flexible: "true"
```

---

## Project Structure

```
ecoscale/
├── cmd/ecoscale/main.go
├── cmd/ecoscale/web/dashboard.html   # embedded dashboard
├── internal/carbon/                  # mock, carbonintensity, electricitymaps
├── internal/kubernetes/
├── internal/optimizer/
├── docker-compose.yml
├── .env.example
└── helm/ecoscale/
```

---

## License

Apache 2.0

**EcoScale — Building the future of Green Computing.**
