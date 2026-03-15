package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// CO2SavedTotal is the cumulative CO2 saved (gCO2) from optimization recommendations.
	CO2SavedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ecoscale_co2_saved_total",
		Help: "Total estimated CO2 saved (grams) from carbon-aware optimization recommendations",
	})

	// CarbonIntensityGauge is the current carbon intensity in gCO2/kWh.
	CarbonIntensityGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "ecoscale_carbon_intensity_gco2_per_kwh",
		Help: "Current carbon intensity of the cluster region in gCO2/kWh",
	})

	// RecommendationsTotal counts optimization recommendations produced.
	RecommendationsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ecoscale_recommendations_total",
		Help: "Total number of optimization recommendations by type",
	}, []string{"type"})

	// ReconciliationRuns counts optimizer reconciliation cycles.
	ReconciliationRuns = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ecoscale_reconciliation_runs_total",
		Help: "Total number of optimizer reconciliation runs",
	})

	// ReconciliationErrors counts failed reconciliation cycles.
	ReconciliationErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "ecoscale_reconciliation_errors_total",
		Help: "Total number of reconciliation errors",
	})
)
