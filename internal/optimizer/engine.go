package optimizer

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/ecoscale/ecoscale/internal/carbon"
	"github.com/ecoscale/ecoscale/internal/kubernetes"
)

// Config holds the optimizer engine configuration.
type Config struct {
	CarbonThreshold     float64  // gCO2/kWh - above this, suggest scale-down/drain
	CompareRegions      []string // Regions to compare for Sun-Chaser (e.g., [us-east-1, us-west-2])
	DefaultCurrentRegion string  // Fallback if cluster region cannot be detected
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		CarbonThreshold:      350,
		CompareRegions:       []string{"us-east-1", "us-west-2"},
		DefaultCurrentRegion: "us-east-1",
	}
}

// Engine is the brain that produces carbon-aware optimization recommendations.
type Engine struct {
	carbonClient carbon.Client
	analyzer     *kubernetes.Analyzer
	config       Config
}

// NewEngine creates an optimizer engine.
func NewEngine(carbonClient carbon.Client, analyzer *kubernetes.Analyzer, config Config) *Engine {
	return &Engine{
		carbonClient: carbonClient,
		analyzer:     analyzer,
		config:       config,
	}
}

// Run produces a set of recommendations based on current carbon intensity and cluster state.
func (e *Engine) Run(ctx context.Context) (*Result, error) {
	return e.RunWithThreshold(ctx, e.config.CarbonThreshold)
}

// RunWithThreshold runs with an optional threshold override (0 = use config default).
func (e *Engine) RunWithThreshold(ctx context.Context, threshold float64) (*Result, error) {
	thresh := threshold
	if thresh <= 0 {
		thresh = e.config.CarbonThreshold
	}
	result := &Result{Timestamp: time.Now().UTC()}

	// 1. Get current region
	currentRegion := e.config.DefaultCurrentRegion
	if e.analyzer != nil {
		if r, err := e.analyzer.GetCurrentRegion(ctx); err == nil && r != "" {
			currentRegion = r
		}
	}
	result.CurrentRegion = currentRegion

	// 2. Get carbon intensity for current region
	intensity, err := e.carbonClient.GetIntensity(ctx, currentRegion)
	if err != nil {
		return nil, fmt.Errorf("get carbon intensity: %w", err)
	}
	result.CurrentIntensity = intensity.Value
	result.IntensitySource = "carbon"

	// 3. High intensity? Suggest node-drain / scale-down for non-critical, non-protected pods
	if intensity.Value > thresh {
		pods, _ := e.listFlexiblePods(ctx)
		for _, p := range pods {
			if !p.Critical && !p.Protected && p.Phase == "Running" {
				result.Recommendations = append(result.Recommendations, Recommendation{
					Type:      ActionScaleDown,
					Reason:    fmt.Sprintf("Carbon intensity %.0f gCO2/kWh exceeds threshold %.0f", intensity.Value, thresh),
					Target:    fmt.Sprintf("%s/%s", p.Namespace, p.Name),
					Details:   "Consider scaling down or deferring non-critical workload",
					Priority:  1,
					Timestamp: time.Now().UTC(),
					CO2Savings: estimateCO2Savings(intensity.Value, 1),
				})
			}
		}
		// Add node-drain suggestion if we have nodes with flexible pods
		if len(pods) > 0 {
			result.Recommendations = append(result.Recommendations, Recommendation{
				Type:      ActionNodeDrain,
				Reason:    fmt.Sprintf("Carbon intensity %.0f gCO2/kWh exceeds threshold %.0f", intensity.Value, thresh),
				Target:    "nodes with ecoscale/flexible pods",
				Details:   "Consider draining nodes during high-carbon periods to reduce consumption",
				Priority:  2,
				Timestamp: time.Now().UTC(),
				CO2Savings: estimateCO2Savings(intensity.Value, len(pods)),
			})
		}
	}

	// 4. Sun-Chaser: Compare regions and suggest region shift
	if len(e.config.CompareRegions) >= 2 {
		comp, err := e.carbonClient.CompareRegions(ctx, e.config.CompareRegions[0], e.config.CompareRegions[1])
		if err == nil && comp.GreenerRegion != currentRegion {
			regionRec := RegionShiftRecommendation{
				FromRegion:     currentRegion,
				ToRegion:       comp.GreenerRegion,
				SavingsPercent: comp.SavingsPercent,
				Timestamp:      time.Now().UTC(),
			}
			if comp.GreenerRegion == comp.RegionA {
				regionRec.IntensityFrom = comp.IntensityB
				regionRec.IntensityTo = comp.IntensityA
			} else {
				regionRec.IntensityFrom = comp.IntensityA
				regionRec.IntensityTo = comp.IntensityB
			}
			regionRec.KarpenterConfig = e.generateKarpenterConfig(comp)
			regionRec.ClusterAutoscalerConfig = e.generateClusterAutoscalerConfig(comp)
			result.RegionShift = &regionRec
			result.Recommendations = append(result.Recommendations, Recommendation{
				Type:       ActionRegionShift,
				Reason:     fmt.Sprintf("Region %s is %.1f%% greener than %s", comp.GreenerRegion, comp.SavingsPercent, currentRegion),
				Target:     comp.GreenerRegion,
				Details:    regionRec.KarpenterConfig,
				Priority:   0,
				Timestamp:  time.Now().UTC(),
				CO2Savings: math.Abs(regionRec.IntensityFrom-regionRec.IntensityTo) * 10, // gCO2/kWh diff × 10 (rough estimate)
			})
		}
	}

	// Sort by priority (lower = more important)
	sort.Slice(result.Recommendations, func(i, j int) bool {
		return result.Recommendations[i].Priority < result.Recommendations[j].Priority
	})

	return result, nil
}

func (e *Engine) listFlexiblePods(ctx context.Context) ([]kubernetes.PodInfo, error) {
	if e.analyzer == nil {
		return nil, nil
	}
	return e.analyzer.ListFlexiblePods(ctx)
}

func estimateCO2Savings(intensityGPerKWh float64, podCount int) float64 {
	// Assume ~50W average per pod, 1 hour = 0.05 kWh per pod
	kWhPerPodHour := 0.05
	return intensityGPerKWh * kWhPerPodHour * float64(podCount)
}

func (e *Engine) generateKarpenterConfig(comp *carbon.RegionComparison) string {
	greener := comp.GreenerRegion
	return fmt.Sprintf(`# Karpenter: Shift capacity to greener region

apiVersion: karpenter.sh/v1beta1
kind: NodePool
metadata:
  name: ecoscale-green
spec:
  template:
    spec:
      requirements:
        - key: topology.kubernetes.io/region
          operator: In
          values: ["%s"]
  # Use this NodePool during high-carbon periods in your primary region
  # Consider using Karpenter's scheduling with taints/tolerations for gradual migration
`, greener)
}

func (e *Engine) generateClusterAutoscalerConfig(comp *carbon.RegionComparison) string {
	greener := comp.GreenerRegion
	return fmt.Sprintf(`# Cluster Autoscaler: Prefer greener region

# Set node group in %s with higher priority during high-carbon periods.
# Example: scale down node groups in high-carbon regions first.

# Option 1: Use separate ASG/node groups per region and scale based on EcoScale recommendations
# Option 2: Configure --scale-down-delay-after-add for nodes in high-carbon regions
`, greener)
}
