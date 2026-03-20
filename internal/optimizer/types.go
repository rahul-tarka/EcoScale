package optimizer

import "time"

// ActionType represents the type of optimization action.
type ActionType string

const (
	ActionNodeDrain   ActionType = "node_drain"
	ActionScaleDown   ActionType = "scale_down"
	ActionRegionShift ActionType = "region_shift"
)

// Recommendation represents a single optimization recommendation.
type Recommendation struct {
	Type        ActionType `json:"type"`
	Reason      string     `json:"reason"`
	Target      string     `json:"target"`
	Details     string     `json:"details"`
	Priority    int        `json:"priority"`
	Timestamp   time.Time  `json:"timestamp"`
	CO2Savings  float64    `json:"co2_savings"` // Estimated gCO2 saved
}

// RegionShiftRecommendation represents a Sun-Chaser recommendation.
type RegionShiftRecommendation struct {
	FromRegion               string    `json:"from_region"`
	ToRegion                 string    `json:"to_region"`
	IntensityFrom            float64   `json:"intensity_from"`
	IntensityTo              float64   `json:"intensity_to"`
	SavingsPercent           float64   `json:"savings_percent"`
	KarpenterConfig          string    `json:"karpenter_config"`
	ClusterAutoscalerConfig  string    `json:"cluster_autoscaler_config"`
	Timestamp                time.Time `json:"timestamp"`
}
