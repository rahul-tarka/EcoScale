package optimizer

import "time"

// Result holds the output of an optimizer run.
type Result struct {
	Timestamp        time.Time                  `json:"timestamp"`
	CurrentRegion    string                     `json:"current_region"`
	CurrentIntensity float64                    `json:"current_intensity"`
	IntensitySource  string                     `json:"intensity_source"`
	Recommendations  []Recommendation            `json:"recommendations"`
	RegionShift      *RegionShiftRecommendation `json:"region_shift,omitempty"`
}
