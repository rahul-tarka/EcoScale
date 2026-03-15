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
	Type        ActionType
	Reason      string
	Target      string
	Details     string
	Priority    int
	Timestamp   time.Time
	CO2Savings  float64 // Estimated gCO2 saved
}

// RegionShiftRecommendation represents a Sun-Chaser recommendation.
type RegionShiftRecommendation struct {
	FromRegion      string
	ToRegion        string
	IntensityFrom   float64
	IntensityTo     float64
	SavingsPercent  float64
	KarpenterConfig string
	ClusterAutoscalerConfig string
	Timestamp       time.Time
}
