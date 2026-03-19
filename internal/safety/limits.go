package safety

import (
	"fmt"
	"math"

	"github.com/ecoscale/ecoscale/internal/config"
	"github.com/ecoscale/ecoscale/internal/kubernetes"
	"github.com/ecoscale/ecoscale/internal/optimizer"
)

// ApplySafetyLimits filters and caps recommendations based on config.
// - When DryRun or !EnableExecution: returns recs unchanged (no execution will occur)
// - When executing: caps evictions to EvictionCapPct of evictable pods
func ApplySafetyLimits(cfg config.Config, pods []kubernetes.PodInfo, recs []optimizer.Recommendation) []optimizer.Recommendation {
	if cfg.DryRun || !cfg.EnableExecution {
		return recs // No execution; return all for display
	}

	// Count evictable pods (flexible, not protected, running)
	evictableCount := 0
	for _, p := range pods {
		if p.Phase == "Running" && !p.Critical && !p.Protected {
			evictableCount++
		}
	}
	if evictableCount == 0 {
		return recs
	}

	capCount := int(math.Ceil(float64(evictableCount) * cfg.EvictionCapPct / 100))
	if capCount < 1 {
		capCount = 1
	}

	// Filter scale_down and node_drain to cap evictions
	filtered := make([]optimizer.Recommendation, 0, len(recs))
	evictionCount := 0
	for _, r := range recs {
		if r.Type == optimizer.ActionScaleDown || r.Type == optimizer.ActionNodeDrain {
			if evictionCount >= capCount {
				continue
			}
			evictionCount++
		}
		filtered = append(filtered, r)
	}

	return filtered
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
