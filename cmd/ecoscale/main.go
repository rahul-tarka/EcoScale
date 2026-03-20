package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ecoscale/ecoscale/internal/carbon"
	"github.com/ecoscale/ecoscale/internal/config"
	"github.com/ecoscale/ecoscale/internal/executor"
	"github.com/ecoscale/ecoscale/internal/metrics"
	"github.com/ecoscale/ecoscale/internal/kubernetes"
	"github.com/ecoscale/ecoscale/internal/optimizer"
	"github.com/ecoscale/ecoscale/internal/safety"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

//go:embed web
var webFS embed.FS

const (
	defaultAddr            = ":8080"
	defaultInterval        = 5 * time.Minute
	defaultCarbonThreshold = 350
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	if err := safety.ValidateConfig(cfg); err != nil {
		slog.Error("invalid config", "error", err)
		os.Exit(1)
	}

	addr := getEnv("ECOSCALE_ADDR", defaultAddr)
	interval := getDurationEnv("ECOSCALE_INTERVAL", defaultInterval)
	inCluster := getEnv("ECOSCALE_IN_CLUSTER", "true") == "true"

	// Carbon client: mock | carbonintensity | electricitymaps
	carbonClient := newCarbonClient(cfg)
	slog.Info("carbon client", "api", cfg.CarbonAPI)

	// Kubernetes client
	var k8sConfig *rest.Config
	var analyzer *kubernetes.Analyzer
	if inCluster {
		var err error
		k8sConfig, err = rest.InClusterConfig()
		if err != nil {
			slog.Warn("in-cluster config failed, running without Kubernetes", "error", err)
		} else {
			analyzer, err = kubernetes.NewAnalyzer(k8sConfig)
			if err != nil {
				slog.Warn("kubernetes analyzer init failed", "error", err)
			}
		}
	}
	if analyzer == nil {
		k8sConfig, _ = clientcmd.BuildConfigFromFlags("", getEnv("KUBECONFIG", ""))
		if k8sConfig != nil {
			analyzer, _ = kubernetes.NewAnalyzer(k8sConfig)
		}
		if analyzer == nil {
			slog.Info("running in standalone mode (no Kubernetes)")
		}
	}

	// Executor (only when execution enabled)
	var exec *executor.Executor
	if safety.ShouldExecute(cfg) && k8sConfig != nil {
		var err error
		exec, err = executor.NewExecutor(k8sConfig)
		if err != nil {
			slog.Error("executor init failed", "error", err)
			os.Exit(1)
		}
		slog.Info("execution enabled", "dry_run", cfg.DryRun, "eviction_cap_pct", cfg.EvictionCapPct)
	} else {
		slog.Info("execution disabled (dry-run or recommendations-only mode)")
	}

	engine := optimizer.NewEngine(carbonClient, analyzer, optimizer.Config{
		CarbonThreshold:       cfg.CarbonThreshold,
		CompareRegions:        []string{"us-east-1", "us-west-2"},
		DefaultCurrentRegion:  "us-east-1",
	})

	// HTTP server
	mux := http.NewServeMux()
	mux.Handle("/ui/", http.StripPrefix("/ui/", http.FileServer(http.Dir("ui"))))
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/ui", func(w http.ResponseWriter, _ *http.Request) {
		data, err := webFS.ReadFile("web/dashboard.html")
		if err != nil {
			http.Error(w, "dashboard not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})
	mux.HandleFunc("/recommendations", recommendationsHandler(engine))
	mux.HandleFunc("/api/regions", regionsHandler(carbonClient))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"name":       "EcoScale",
			"version":    "0.3.0",
			"tagline":    "World's First Carbon-Aware Kubernetes Scheduler",
			"dashboard":  "/ui",
			"dashboard_ui": "/ui/",
			"carbon_api": cfg.CarbonAPI,
			"dry_run":    cfg.DryRun,
		})
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runReconciliationLoop(ctx, engine, analyzer, exec, cfg, interval)

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		slog.Info("shutting down")
		cancel()
		srv.Shutdown(context.Background())
	}()

	slog.Info("EcoScale started", "addr", addr, "interval", interval, "carbon_api", cfg.CarbonAPI, "dry_run", cfg.DryRun)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func loadConfig() config.Config {
	cfg := config.DefaultConfig()
	cfg.CarbonAPI = strings.ToLower(getEnv("ECOSCALE_CARBON_API", "mock"))
	cfg.CarbonAPIKey = getEnv("ECOSCALE_CARBON_API_KEY", "")
	cfg.CarbonThreshold = getFloatEnv("ECOSCALE_CARBON_THRESHOLD", defaultCarbonThreshold)
	cfg.DryRun = getEnv("ECOSCALE_DRY_RUN", "true") == "true"
	cfg.EvictionCapPct = getFloatEnv("ECOSCALE_EVICTION_CAP_PCT", 10)
	cfg.EnableExecution = getEnv("ECOSCALE_ENABLE_EXECUTION", "false") == "true"
	return cfg
}

func newCarbonClient(cfg config.Config) carbon.Client {
	switch cfg.CarbonAPI {
	case "carbonintensity":
		return carbon.NewCarbonIntensityClient()
	case "electricitymaps":
		if cfg.CarbonAPIKey == "" {
			slog.Warn("ECOSCALE_CARBON_API_KEY not set; falling back to mock")
			return carbon.NewMockClient(true)
		}
		return carbon.NewElectricityMapsClient(cfg.CarbonAPIKey)
	default:
		return carbon.NewMockClient(true)
	}
}

func runReconciliationLoop(ctx context.Context, engine *optimizer.Engine, analyzer *kubernetes.Analyzer, exec *executor.Executor, cfg config.Config, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		runOnce(ctx, engine, analyzer, exec, cfg)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runOnce(ctx context.Context, engine *optimizer.Engine, analyzer *kubernetes.Analyzer, exec *executor.Executor, cfg config.Config) {
	metrics.ReconciliationRuns.Inc()
	result, err := engine.Run(ctx)
	if err != nil {
		metrics.ReconciliationErrors.Inc()
		slog.Error("reconciliation failed", "error", err)
		return
	}

	metrics.CarbonIntensityGauge.Set(result.CurrentIntensity)

	// Get pods for safety limits
	var pods []kubernetes.PodInfo
	if analyzer != nil {
		pods, _ = analyzer.ListFlexiblePods(ctx)
	}

	// Apply safety limits (caps evictions when executing)
	filtered := safety.ApplySafetyLimits(cfg, pods, result.Recommendations)

	var totalCO2Saved float64
	for _, r := range filtered {
		metrics.RecommendationsTotal.WithLabelValues(string(r.Type)).Inc()
		totalCO2Saved += r.CO2Savings
	}
	if totalCO2Saved > 0 {
		metrics.CO2SavedTotal.Add(totalCO2Saved)
	}

	// Execute if enabled
	if exec != nil && safety.ShouldExecute(cfg) && len(filtered) > 0 {
		n, err := exec.Execute(ctx, filtered)
		if err != nil {
			slog.Error("execution failed", "error", err)
		} else if n > 0 {
			slog.Info("executed actions", "count", n)
		}
	}

	slog.Info("reconciliation complete",
		"region", result.CurrentRegion,
		"intensity", result.CurrentIntensity,
		"recommendations", len(result.Recommendations),
		"filtered", len(filtered),
	)
}

func recommendationsHandler(engine *optimizer.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		threshold := getFloatEnv("ECOSCALE_CARBON_THRESHOLD", defaultCarbonThreshold)
		if t := r.URL.Query().Get("threshold"); t != "" {
			if f, err := parseFloat(t); err == nil && f > 0 {
				threshold = f
			}
		}
		result, err := engine.RunWithThreshold(r.Context(), threshold)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
}

func regionsHandler(carbonClient carbon.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		regionsParam := r.URL.Query().Get("regions")
		if regionsParam == "" {
			regionsParam = "us-east-1,us-west-2,eu-north-1,ap-southeast-1"
		}
		regions := strings.Split(regionsParam, ",")
		ctx := r.Context()
		var out []struct {
			Region    string  `json:"region"`
			Intensity float64 `json:"intensity"`
		}
		for _, region := range regions {
			region = strings.TrimSpace(region)
			if region == "" {
				continue
			}
			intensity, err := carbonClient.GetIntensity(ctx, region)
			if err != nil {
				slog.Warn("failed to get intensity for region", "region", region, "error", err)
				continue
			}
			out = append(out, struct {
				Region    string  `json:"region"`
				Intensity float64 `json:"intensity"`
			}{Region: region, Intensity: intensity.Value})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(out)
	}
}

func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getFloatEnv(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		var f float64
		if n, err := fmt.Sscanf(v, "%f", &f); n >= 1 && err == nil {
			return f
		}
	}
	return def
}
