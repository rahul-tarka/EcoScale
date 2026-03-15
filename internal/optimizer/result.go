package optimizer

import "time"

// Result holds the output of an optimizer run.
type Result struct {
	Timestamp        time.Time
	CurrentRegion     string
	CurrentIntensity  float64
	IntensitySource   string
	Recommendations  []Recommendation
	RegionShift      *RegionShiftRecommendation
}
