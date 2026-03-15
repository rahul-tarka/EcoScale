package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ecoscale/ecoscale/internal/carbon"
	"github.com/ecoscale/ecoscale/internal/metrics"
	"github.com/ecoscale/ecoscale/internal/kubernetes"
	"github.com/ecoscale/ecoscale/internal/optimizer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	defaultAddr         = ":8080"
	defaultInterval     = 5 * time.Minute
	defaultCarbonThreshold = 350
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	addr := getEnv("ECOSCALE_ADDR", defaultAddr)
	interval := getDurationEnv("ECOSCALE_INTERVAL", defaultInterval)
	threshold := getFloatEnv("ECOSCALE_CARBON_THRESHOLD", defaultCarbonThreshold)
	inCluster := getEnv("ECOSCALE_IN_CLUSTER", "true") == "true"

	// Carbon client (mock for now)
	carbonClient := carbon.NewMockClient(true)

	// Kubernetes client
	var analyzer *kubernetes.Analyzer
	if inCluster {
		config, err := rest.InClusterConfig()
		if err != nil {
			slog.Warn("in-cluster config failed, running without Kubernetes", "error", err)
		} else {
			analyzer, err = kubernetes.NewAnalyzer(config)
			if err != nil {
				slog.Warn("kubernetes analyzer init failed", "error", err)
			}
		}
	}
	if analyzer == nil {
		// Try kubeconfig for local dev
		config, err := clientcmd.BuildConfigFromFlags("", getEnv("KUBECONFIG", ""))
		if err == nil {
			analyzer, _ = kubernetes.NewAnalyzer(config)
		}
		if analyzer == nil {
			slog.Info("running in standalone mode (no Kubernetes)")
		}
	}

	engine := optimizer.NewEngine(carbonClient, analyzer, optimizer.Config{
		CarbonThreshold:      threshold,
		CompareRegions:       []string{"us-east-1", "us-west-2"},
		DefaultCurrentRegion: "us-east-1",
	})

	// HTTP server
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/recommendations", recommendationsHandler(engine))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":    "EcoScale",
			"version": "0.1.0",
			"tagline": "World's First Carbon-Aware Kubernetes Scheduler",
		})
	})

	srv := &http.Server{Addr: addr, Handler: mux}

	// Reconciliation loop
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runReconciliationLoop(ctx, engine, interval)

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		slog.Info("shutting down")
		cancel()
		srv.Shutdown(context.Background())
	}()

	slog.Info("EcoScale started", "addr", addr, "interval", interval)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

func runReconciliationLoop(ctx context.Context, engine *optimizer.Engine, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		runOnce(ctx, engine)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func runOnce(ctx context.Context, engine *optimizer.Engine) {
	metrics.ReconciliationRuns.Inc()
	result, err := engine.Run(ctx)
	if err != nil {
		metrics.ReconciliationErrors.Inc()
		slog.Error("reconciliation failed", "error", err)
		return
	}

	metrics.CarbonIntensityGauge.Set(result.CurrentIntensity)

	var totalCO2Saved float64
	for _, r := range result.Recommendations {
		metrics.RecommendationsTotal.WithLabelValues(string(r.Type)).Inc()
		totalCO2Saved += r.CO2Savings
	}
	if totalCO2Saved > 0 {
		metrics.CO2SavedTotal.Add(totalCO2Saved)
	}

	slog.Info("reconciliation complete",
		"region", result.CurrentRegion,
		"intensity", result.CurrentIntensity,
		"recommendations", len(result.Recommendations),
	)
}

func recommendationsHandler(engine *optimizer.Engine) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := engine.Run(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
	}
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
