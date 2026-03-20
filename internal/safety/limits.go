package safety

import (
	"fmt"
	"math"

	"github.com/ecoscale/ecoscale/internal/config"
	"github.com/ecoscale/ecoscale/internal/kubernetes"
	"github.com/ecoscale/ecoscale/internal/optimizer"
)

// ApplySafetyLimits returns recommendations unchanged. Eviction volume is enforced
// per reconciliation by MaxPodEvictions inside the executor.
func ApplySafetyLimits(_ config.Config, _ []kubernetes.PodInfo, recs []optimizer.Recommendation) []optimizer.Recommendation {
	return recs
}

// MaxPodEvictions returns the maximum number of flexible pods to evict in one cycle.
func MaxPodEvictions(cfg config.Config, pods []kubernetes.PodInfo) int {
	evictableCount := 0
	for _, p := range pods {
		if p.Phase == "Running" && !p.Critical && !p.Protected {
			evictableCount++
		}
	}
	if evictableCount == 0 {
		return 0
	}
	capCount := int(math.Ceil(float64(evictableCount) * cfg.EvictionCapPct / 100))
	if capCount < 1 {
		capCount = 1
	}
	return capCount
}

// ShouldExecute returns true if the system should execute (not just recommend).
func ShouldExecute(cfg config.Config) bool {
	return !cfg.DryRun && cfg.EnableExecution
}

// ValidateConfig returns an error if config is invalid.
func ValidateConfig(cfg config.Config) error {
	if cfg.EvictionCapPct < 0 || cfg.EvictionCapPct > 100 {
		return fmt.Errorf("EvictionCapPct must be 0-100, got %.1f", cfg.EvictionCapPct)
	}
	return nil
}
